package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// gitLabDraftTitle matches the title prefixes GitLab treats as draft markers.
// The update API has no draft attribute, so clearing draft means removing it.
var gitLabDraftTitle = regexp.MustCompile(`(?i)^\s*(\[draft\]|\(draft\)|draft:)\s*`)

// detailed_merge_status values that refuse a merge for a reason other than
// checks, conflicts, draft, or a pending mergeability computation.
var gitLabBlockingMergeStatus = map[string]bool{
	"approvals_syncing": true, "commits_status": true, "discussions_not_resolved": true,
	"jira_association_missing": true, "locked_lfs_files": true, "locked_paths": true,
	"merge_request_blocked": true, "merge_time": true, "need_rebase": true, "not_approved": true,
	"requested_changes": true, "security_policy_pipeline_check": true, "security_policy_violations": true,
	"status_checks_must_pass": true, "title_regex": true,
}

func (Provider) ReadPullRequestMergeState(ctx context.Context, repo, artifactURL string) (model.RemotePullRequestMergeState, error) {
	hostname, endpoint, err := gitLabMergeRequestEndpoint(artifactURL)
	if err != nil {
		return model.RemotePullRequestMergeState{}, err
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint)
	if err != nil {
		return model.RemotePullRequestMergeState{}, err
	}
	state, _, err := parseGlabMergeState(out)
	return state, err
}

// MergePullRequest never sets auto_merge or should_remove_source_branch: a
// scheduled merge leaves nothing to verify, and the remote branch belongs to
// `cleanup remote-branch`. GitLab fixes merge-commit vs fast-forward per
// project, so only squash and merge are selectable here.
func (p Provider) MergePullRequest(ctx context.Context, req port.IssueProviderPullRequestMergeRequest) (model.RemotePullRequestMergeState, error) {
	hostname, endpoint, err := gitLabMergeRequestEndpoint(req.URL)
	if err != nil {
		return model.RemotePullRequestMergeState{}, err
	}
	head := strings.TrimSpace(req.HeadOID)
	if head == "" {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("merge requires the expected head commit")
	}
	var squash string
	switch req.Method {
	case model.RemoteMergeMethodSquash:
		squash = "true"
	case model.RemoteMergeMethodMerge:
		squash = "false"
	case model.RemoteMergeMethodRebase:
		return model.RemotePullRequestMergeState{}, fmt.Errorf("GitLab sets fast-forward or merge commits per project; use --method squash or merge")
	default:
		return model.RemotePullRequestMergeState{}, fmt.Errorf("merge method must be squash, merge, or rebase")
	}
	if req.MarkReady {
		out, err := runGlabAPIContext(ctx, req.Repo, hostname, endpoint)
		if err != nil {
			return model.RemotePullRequestMergeState{}, err
		}
		_, title, err := parseGlabMergeState(out)
		if err != nil {
			return model.RemotePullRequestMergeState{}, err
		}
		ready := gitLabDraftTitle.ReplaceAllString(title, "")
		if ready == title {
			return model.RemotePullRequestMergeState{}, fmt.Errorf("merge request is a draft but its title %q has no draft prefix to remove", title)
		}
		if _, err := runGlabAPIContext(ctx, req.Repo, hostname, endpoint, "--method", "PUT", "-f", "title="+ready); err != nil {
			return model.RemotePullRequestMergeState{}, err
		}
	}
	if _, err := runGlabAPIContext(ctx, req.Repo, hostname, endpoint+"/merge", "--method", "PUT",
		"-f", "sha="+head, "-F", "squash="+squash, "-F", "should_remove_source_branch=false", "-F", "auto_merge=false"); err != nil {
		return model.RemotePullRequestMergeState{}, err
	}
	return p.ReadPullRequestMergeState(ctx, req.Repo, req.URL)
}

func gitLabMergeRequestEndpoint(artifactURL string) (string, string, error) {
	hostname, projectPath, iid, err := parseGitLabMergeRequestURL(artifactURL)
	if err != nil {
		return "", "", err
	}
	return hostname, "projects/" + url.PathEscape(projectPath) + "/merge_requests/" + iid, nil
}

func parseGlabMergeState(out []byte) (model.RemotePullRequestMergeState, string, error) {
	var payload struct {
		WebURL              string `json:"web_url"`
		Title               string `json:"title"`
		State               string `json:"state"`
		Draft               bool   `json:"draft"`
		SHA                 string `json:"sha"`
		TargetBranch        string `json:"target_branch"`
		HasConflicts        bool   `json:"has_conflicts"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
		MergeCommitSHA      string `json:"merge_commit_sha"`
		SquashCommitSHA     string `json:"squash_commit_sha"`
		HeadPipeline        *struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"head_pipeline"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return model.RemotePullRequestMergeState{}, "", fmt.Errorf("parse glab merge request merge state: %w", err)
	}
	state := model.RemotePullRequestMergeState{
		URL: payload.WebURL, Draft: payload.Draft, HeadOID: payload.SHA, BaseBranch: payload.TargetBranch,
		Conflict:       payload.HasConflicts || payload.DetailedMergeStatus == "conflict",
		MergeCommitOID: payload.SquashCommitSHA,
		Checks:         model.RemoteChecksNone,
	}
	if state.MergeCommitOID == "" {
		state.MergeCommitOID = payload.MergeCommitSHA
	}
	switch strings.ToLower(payload.State) {
	case "opened":
		state.State = "open"
	case "merged":
		state.State = "merged"
	default:
		state.State = "closed"
	}
	if gitLabBlockingMergeStatus[payload.DetailedMergeStatus] {
		state.Blocked = payload.DetailedMergeStatus
	}
	if pipeline := payload.HeadPipeline; pipeline != nil {
		name := "pipeline #" + strconv.FormatInt(pipeline.ID, 10)
		switch strings.ToLower(pipeline.Status) {
		case "success", "skipped":
			state.Checks = model.RemoteChecksPassing
		case "failed", "canceled", "canceling":
			state.Checks, state.Failing = model.RemoteChecksFailing, []string{name}
		default:
			state.Checks, state.Pending = model.RemoteChecksPending, []string{name}
		}
	}
	return state, payload.Title, nil
}
