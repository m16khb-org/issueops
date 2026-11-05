package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"net/url"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	"issueops/internal/port"
)

// ReadIssueSnapshot reads the description, web URL and state of req.URL with
// `glab api` and returns them tagged with Source "glab_cli". It fails when the
// returned web URL does not identify the same issue as the request.
func (Provider) ReadIssueSnapshot(ctx context.Context, req executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return executionissue.Snapshot{}, err
	}
	hostname, projectPath, iid, _, err := splitGitLabIssueURL(req.URL)
	if err != nil {
		return executionissue.Snapshot{}, err
	}
	endpoint := "projects/" + url.PathEscape(projectPath) + "/issues/" + iid
	args := []string{"api", endpoint}
	if hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, req.Repo, "glab", args...)
	if err != nil {
		return executionissue.Snapshot{}, fmt.Errorf("glab issue snapshot read failed: %w", err)
	}
	var payload struct {
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return executionissue.Snapshot{}, fmt.Errorf("parse glab issue snapshot: %w", err)
	}
	issueURL := strings.TrimSpace(req.URL)
	if !sameGitLabIssueSnapshotIdentity(issueURL, payload.WebURL) {
		return executionissue.Snapshot{}, fmt.Errorf("glab issue snapshot URL does not match the linked issue")
	}
	return executionissue.Snapshot{URL: issueURL, Body: payload.Description, State: payload.State, Source: "glab_cli"}, nil
}

func sameGitLabIssueSnapshotIdentity(left, right string) bool {
	leftHost, leftProject, leftIID, _, leftErr := splitGitLabIssueURL(left)
	rightHost, rightProject, rightIID, _, rightErr := splitGitLabIssueURL(right)
	return leftErr == nil && rightErr == nil &&
		strings.EqualFold(leftHost, rightHost) &&
		strings.EqualFold(gitLabSnapshotAuthority(left), gitLabSnapshotAuthority(right)) &&
		leftProject == rightProject &&
		leftIID == rightIID
}

func gitLabSnapshotAuthority(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Host == "" && !strings.Contains(trimmed, "://") {
		parsed, err = url.Parse("https://" + trimmed)
		if err != nil {
			return ""
		}
	}
	return parsed.Host
}

var _ port.ExecutionIssueSnapshotReader = Provider{}
