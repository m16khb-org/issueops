package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// Retitle runs `gh issue|pr edit --title` and reads the title back. A
// GitHub draft is a separate flag, so the title carries no draft marker.
func (Provider) Retitle(ctx context.Context, repo, targetURL, kind, title string) (string, error) {
	noun, err := ghArtifactCommand(kind)
	if err != nil {
		return "", err
	}
	targetURL = strings.TrimSpace(targetURL)
	if noun == "pr" && !validCanonicalGitHubPullRequestURL(targetURL) {
		return "", fmt.Errorf("pull request url must be a canonical https://<host>/<owner>/<repo>/pull/<number> url")
	}
	if noun == "issue" {
		if _, _, _, err := parseGitHubIssueURL(targetURL); err != nil {
			return "", err
		}
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", noun, "edit", targetURL, "--title", title); err != nil {
		return "", fmt.Errorf("gh %s edit --title failed: %w", noun, err)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", noun, "view", targetURL, "--json", "title")
	if err != nil {
		return "", fmt.Errorf("title not read back: %w", err)
	}
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gh %s title: %w", noun, err)
	}
	return payload.Title, nil
}
