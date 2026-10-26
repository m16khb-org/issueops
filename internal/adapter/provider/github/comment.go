package github

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// Comment posts one comment with `gh issue|pr comment` and requires the URL gh
// prints to address the target, so a comment the CLI did not confirm is not
// reported as posted.
func (Provider) Comment(ctx context.Context, repo, targetURL, kind, body string) (string, error) {
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
	out, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "gh", noun, "comment", targetURL, "--body", body)
	if err != nil {
		return "", fmt.Errorf("gh %s comment failed: %w", noun, err)
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], targetURL+"#") {
		return "", fmt.Errorf("gh %s comment did not print the comment url (output %q); check the %s before posting again", noun, strings.TrimSpace(string(out)), noun)
	}
	return lines[len(lines)-1], nil
}
