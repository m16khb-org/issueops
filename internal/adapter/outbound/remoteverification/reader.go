package remoteverification

import (
	"context"
	"encoding/json"
	"fmt"
	model "issueops/internal/contract/remoteverification"
	"net/url"
	"os/exec"
	"strings"
)

type Reader struct{}

func (Reader) Artifact(ctx context.Context, target model.Target) (model.Artifact, error) {
	switch target.Provider + ":" + target.Kind {
	case "github:issue":
		return fetchGitHubIssueArtifactContext(ctx, target.URL)
	case "github:pr":
		return fetchGitHubPullRequestArtifactContext(ctx, target.URL)
	case "gitlab:issue":
		return fetchGitLabIssueArtifactContext(ctx, target.URL)
	case "gitlab:mr":
		return fetchGitLabMergeRequestArtifactContext(ctx, target.URL)
	default:
		return model.Artifact{}, fmt.Errorf("unsupported remote artifact verification: %s:%s", target.Provider, target.Kind)
	}
}
func (Reader) GitHubChild(ctx context.Context, issueURL string) error {
	if _, err := runRemoteVerifyCommand(ctx, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "gh", "issue", "view", strings.TrimSpace(issueURL), "--json", "url,state,title")
	}); err != nil {
		return fmt.Errorf("verify GitHub child issue through gh failed: %w", commandOutputError(err))
	}
	return nil
}
func (Reader) GitLabChild(ctx context.Context, target model.ChildTarget, taskMetadata bool) (model.TaskMetadata, error) {
	endpoint := "projects/" + url.PathEscape(target.Project) + "/" + target.Kind + "/" + target.IID
	out, err := runRemoteVerifyCommand(ctx, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "glab", "api", endpoint, "--hostname", target.Hostname)
	})
	if err != nil {
		return model.TaskMetadata{}, fmt.Errorf("verify GitLab child issue through glab failed: %w", commandOutputError(err))
	}
	if !taskMetadata {
		return model.TaskMetadata{}, nil
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return model.TaskMetadata{Empty: true}, nil
	}
	var payload struct {
		Type      string `json:"type"`
		IssueType string `json:"issue_type"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return model.TaskMetadata{}, fmt.Errorf("decode GitLab issue fallback for work item %s: %w", target.IID, err)
	}
	return model.TaskMetadata{Type: payload.Type, IssueType: payload.IssueType}, nil
}
