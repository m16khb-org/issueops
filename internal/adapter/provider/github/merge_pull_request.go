package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

const ghMergeStateFields = "url,state,isDraft,headRefOid,baseRefName,mergeable,mergeStateStatus,statusCheckRollup,mergeCommit"

// ReadPullRequestMergeState reads draft, head, checks and mergeability in one
// `gh pr view` so the merge decision never mixes two observations.
func (Provider) ReadPullRequestMergeState(ctx context.Context, repo, artifactURL string) (model.RemotePullRequestMergeState, error) {
	artifactURL = strings.TrimSpace(artifactURL)
	if !validCanonicalGitHubPullRequestURL(artifactURL) {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("pull request url must be a canonical https://<host>/<owner>/<repo>/pull/<number> url")
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", "pr", "view", artifactURL, "--json", ghMergeStateFields)
	if err != nil {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("gh pr view failed: %w", err)
	}
	return parseGhMergeState(out)
}

// MergePullRequest never passes --delete-branch, --admin, or --auto: the
// remote branch belongs to `cleanup remote-branch`, protection is not bypassed,
// and a queued merge would leave nothing to verify.
func (p Provider) MergePullRequest(ctx context.Context, req port.IssueProviderPullRequestMergeRequest) (model.RemotePullRequestMergeState, error) {
	artifactURL := strings.TrimSpace(req.URL)
	if !validCanonicalGitHubPullRequestURL(artifactURL) {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("pull request url must be a canonical https://<host>/<owner>/<repo>/pull/<number> url")
	}
	head := strings.TrimSpace(req.HeadOID)
	if head == "" {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("merge requires the expected head commit")
	}
	var methodFlag string
	switch req.Method {
	case model.RemoteMergeMethodSquash, model.RemoteMergeMethodMerge, model.RemoteMergeMethodRebase:
		methodFlag = "--" + req.Method
	default:
		return model.RemotePullRequestMergeState{}, fmt.Errorf("merge method must be squash, merge, or rebase")
	}
	if req.MarkReady {
		if _, _, err := providerutil.RunBoundedMutationContext(ctx, req.Repo, "gh", "pr", "ready", artifactURL); err != nil {
			return model.RemotePullRequestMergeState{}, fmt.Errorf("gh pr ready failed: %w", err)
		}
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, req.Repo, "gh", "pr", "merge", artifactURL, methodFlag, "--match-head-commit", head); err != nil {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("gh pr merge failed: %w", err)
	}
	return p.ReadPullRequestMergeState(ctx, req.Repo, artifactURL)
}

type ghCheck struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

func parseGhMergeState(out []byte) (model.RemotePullRequestMergeState, error) {
	var payload struct {
		URL              string    `json:"url"`
		State            string    `json:"state"`
		IsDraft          bool      `json:"isDraft"`
		HeadRefOid       string    `json:"headRefOid"`
		BaseRefName      string    `json:"baseRefName"`
		Mergeable        string    `json:"mergeable"`
		MergeStateStatus string    `json:"mergeStateStatus"`
		Checks           []ghCheck `json:"statusCheckRollup"`
		MergeCommit      *struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return model.RemotePullRequestMergeState{}, fmt.Errorf("parse gh pull request merge state: %w", err)
	}
	state := model.RemotePullRequestMergeState{
		URL: payload.URL, State: strings.ToLower(payload.State), Draft: payload.IsDraft,
		HeadOID: payload.HeadRefOid, BaseBranch: payload.BaseRefName,
		Conflict: strings.EqualFold(payload.Mergeable, "CONFLICTING"),
	}
	if payload.MergeCommit != nil {
		state.MergeCommitOID = payload.MergeCommit.OID
	}
	for _, check := range payload.Checks {
		name := check.Name
		if name == "" {
			name = check.Context
		}
		switch ghCheckOutcome(check) {
		case model.RemoteChecksFailing:
			state.Failing = append(state.Failing, name)
		case model.RemoteChecksPending:
			state.Pending = append(state.Pending, name)
		}
	}
	switch {
	case len(state.Failing) > 0:
		state.Checks = model.RemoteChecksFailing
	case len(state.Pending) > 0:
		state.Checks = model.RemoteChecksPending
	case len(payload.Checks) > 0:
		state.Checks = model.RemoteChecksPassing
	default:
		state.Checks = model.RemoteChecksNone
	}
	// BLOCKED also covers failing required checks; report it only when the
	// checks do not already explain the block.
	if strings.EqualFold(payload.MergeStateStatus, "BLOCKED") && state.Checks != model.RemoteChecksFailing && state.Checks != model.RemoteChecksPending {
		state.Blocked = "BLOCKED (required review or branch protection)"
	}
	return state, nil
}

// ghCheckOutcome folds a CheckRun or a legacy StatusContext into one outcome.
func ghCheckOutcome(check ghCheck) string {
	if check.Typename == "StatusContext" {
		switch strings.ToUpper(check.State) {
		case "SUCCESS":
			return model.RemoteChecksPassing
		case "PENDING", "EXPECTED":
			return model.RemoteChecksPending
		default:
			return model.RemoteChecksFailing
		}
	}
	if !strings.EqualFold(check.Status, "COMPLETED") {
		return model.RemoteChecksPending
	}
	switch strings.ToUpper(check.Conclusion) {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return model.RemoteChecksPassing
	default:
		return model.RemoteChecksFailing
	}
}
