package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// SetMilestone runs `gh issue|pr edit --milestone` (gh refuses a milestone
// the repository lacks) and reads the milestone back.
func (Provider) SetMilestone(ctx context.Context, repo, targetURL, kind, title string) (string, error) {
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
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", noun, "edit", targetURL, "--milestone", title); err != nil {
		return "", fmt.Errorf("gh %s edit --milestone failed: %w", noun, err)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", noun, "view", targetURL, "--json", "milestone")
	if err != nil {
		return "", fmt.Errorf("milestone not read back: %w", err)
	}
	var payload struct {
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gh %s milestone: %w", noun, err)
	}
	if payload.Milestone == nil {
		return "", nil
	}
	return payload.Milestone.Title, nil
}
