package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const executionIssueSnapshotBodyLimit = 1 << 19

type gitLabExecutionIssueIdentity struct {
	authority string
	project   string
	iid       string
}

func ValidateExecutionSnapshotAction(req issueops.ExecutionActionRequest) error {
	switch req.Action {
	case issueops.ExecutionActionPrepare, issueops.ExecutionActionClaim:
		return nil
	case issueops.ExecutionActionReconcile:
		if req.Confirm && !req.Preview {
			return nil
		}
	case issueops.ExecutionActionReplace:
		if req.ReplaceAction == issueops.ExecutionReplaceFinalize || req.ReplaceAction == issueops.ExecutionReplaceReseed {
			return nil
		}
	}
	return fmt.Errorf("issue_snapshot is not supported for execution action %q", req.Action)
}

func ValidateExecutionSnapshotRecord(req issueops.ExecutionActionRequest, record issueops.IssueOpsRecord) error {
	if req.Action != issueops.ExecutionActionReconcile {
		return nil
	}
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca ||
		record.Execution.Pending == nil || record.Execution.Pending.Kind != "worktree_create" {
		return fmt.Errorf("issue_snapshot is supported for execution reconcile only when confirm resumes a pending worktree_create intent")
	}
	return nil
}

func ValidateGitLabExecutionSnapshot(linkedURL, snapshotURL, body, state string) error {
	if !SameGitLabExecutionIssueIdentity(linkedURL, snapshotURL) {
		return fmt.Errorf("GitLab issue snapshot URL does not match the linked issue")
	}
	if strings.TrimSpace(body) == "" || len([]byte(body)) > executionIssueSnapshotBodyLimit {
		return fmt.Errorf("GitLab issue snapshot body must be non-empty and at most %d bytes", executionIssueSnapshotBodyLimit)
	}
	if state != "opened" && state != "closed" {
		return fmt.Errorf("GitLab issue snapshot state must be opened or closed")
	}
	return nil
}

func SameGitLabExecutionIssueIdentity(left, right string) bool {
	leftIdentity, leftErr := parseGitLabExecutionIssueIdentity(left)
	rightIdentity, rightErr := parseGitLabExecutionIssueIdentity(right)
	return leftErr == nil && rightErr == nil &&
		leftIdentity.authority == rightIdentity.authority &&
		leftIdentity.project == rightIdentity.project &&
		leftIdentity.iid == rightIdentity.iid
}

func parseGitLabExecutionIssueIdentity(raw string) (gitLabExecutionIssueIdentity, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must be canonical")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must be canonical HTTPS without userinfo, query, or fragment")
	}
	path := parsed.EscapedPath()
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL path is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 5 || parts[len(parts)-3] != "-" ||
		(parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue URL must end with /-/issues/:iid or /-/work_items/:iid")
	}
	projectParts := parts[:len(parts)-3]
	for _, part := range projectParts {
		decoded, decodeErr := url.PathUnescape(part)
		if decodeErr != nil || part == "" || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, `/\`) || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
			return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue project path is invalid")
		}
	}
	iid := parts[len(parts)-1]
	number, err := strconv.Atoi(iid)
	if err != nil || number <= 0 || strconv.Itoa(number) != iid {
		return gitLabExecutionIssueIdentity{}, fmt.Errorf("GitLab issue IID must be a canonical positive integer")
	}
	return gitLabExecutionIssueIdentity{
		authority: strings.ToLower(parsed.Host),
		project:   strings.Join(projectParts, "/"),
		iid:       iid,
	}, nil
}

func ValidateExecutionSnapshotEvidence(req issueops.ExecutionActionRequest, record issueops.IssueOpsRecord) error {
	if err := ValidateExecutionSnapshotRecord(req, record); err != nil {
		return err
	}
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.Provider) != "gitlab" {
		return fmt.Errorf("issue_snapshot provider does not match the linked IssueOps provider")
	}
	if !SameGitLabExecutionIssueIdentity(record.IssueURL, record.BranchPrepare.IssueURL) {
		return fmt.Errorf("linked IssueOps GitLab identity is inconsistent")
	}
	evidence := *req.IssueSnapshot
	if evidence.Provider != "gitlab" {
		return fmt.Errorf("issue_snapshot provider must be gitlab")
	}
	if evidence.Source != "glab_mcp" {
		return fmt.Errorf("issue_snapshot source must be glab_mcp")
	}
	if err := ValidateGitLabExecutionSnapshot(record.IssueURL, evidence.WebURL, evidence.Body, evidence.State); err != nil {
		return fmt.Errorf("invalid issue_snapshot: %w", err)
	}
	return nil
}
