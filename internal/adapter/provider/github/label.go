package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// AddLabels runs `gh issue|pr edit --add-label` (add only) and reads the
// labels back.
func (Provider) AddLabels(ctx context.Context, repo, targetURL, kind string, labels []string) ([]string, error) {
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
	for _, label := range labels {
		args = append(args, "--add-label", label)
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", args...); err != nil {
		return nil, fmt.Errorf("gh %s edit --add-label failed: %w", noun, err)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", noun, "view", targetURL, "--json", "labels")
	if err != nil {
		return nil, fmt.Errorf("labels not read back: %w", err)
	}
	var payload struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gh %s labels: %w", noun, err)
	}
	names := make([]string, 0, len(payload.Labels))
	for _, label := range payload.Labels {
		names = append(names, label.Name)
	}
	return names, nil
}
