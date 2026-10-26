package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// AddAssignees runs `gh issue|pr edit --add-assignee` (add only) and reads
// the assignees back.
func (Provider) AddAssignees(ctx context.Context, repo, targetURL, kind string, users []string) ([]string, error) {
	noun, err := ghArtifactCommand(kind)
	if err != nil {
		return nil, err
	}
	targetURL = strings.TrimSpace(targetURL)
	if noun == "pr" && !validCanonicalGitHubPullRequestURL(targetURL) {
		return nil, fmt.Errorf("pull request url must be a canonical https://<host>/<owner>/<repo>/pull/<number> url")
	}
	if noun == "issue" {
		if _, _, _, err := parseGitHubIssueURL(targetURL); err != nil {
			return nil, err
		}
	}
	args := []string{noun, "edit", targetURL}
	for _, user := range users {
		args = append(args, "--add-assignee", user)
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", args...); err != nil {
		return nil, fmt.Errorf("gh %s edit --add-assignee failed: %w", noun, err)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", noun, "view", targetURL, "--json", "assignees")
	if err != nil {
		return nil, fmt.Errorf("assignees not read back: %w", err)
	}
	var payload struct {
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gh %s assignees: %w", noun, err)
	}
	logins := make([]string, 0, len(payload.Assignees))
	for _, assignee := range payload.Assignees {
		logins = append(logins, assignee.Login)
	}
	return logins, nil
}
