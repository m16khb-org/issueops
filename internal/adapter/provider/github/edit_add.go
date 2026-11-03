package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// addEdit runs `gh issue|pr edit <flag> <value>...` (add only) and reads the
// artifact's `field` back through `gh <noun> view --json field`, decoding the
// response into P and projecting it with names.
func addEdit[P any](ctx context.Context, repo, targetURL, kind, flag, field string, values []string, names func(P) []string) ([]string, error) {
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
	for _, value := range values {
		args = append(args, flag, value)
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", args...); err != nil {
		return nil, fmt.Errorf("gh %s edit %s failed: %w", noun, flag, err)
	}
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", noun, "view", targetURL, "--json", field)
	if err != nil {
		return nil, fmt.Errorf("%s not read back: %w", field, err)
	}
	var payload P
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gh %s %s: %w", noun, field, err)
	}
	return names(payload), nil
}
