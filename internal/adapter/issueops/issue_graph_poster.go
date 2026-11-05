package issueops

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// IssueGraphPoster posts issue-graph comments through the provider CLI.
type IssueGraphPoster struct{}

// Post adds body as a comment (gh) or note (glab) on issueURL and returns the CLI output.
// It fails when the CLI is not installed or exits non-zero.
func (IssueGraphPoster) Post(ctx context.Context, provider, issueURL, body string) (string, error) {
	tool, action, bodyFlag := "gh", "comment", "--body"
	if provider == "gitlab" {
		tool, action, bodyFlag = "glab", "note", "--message"
	}
	if _, err := exec.LookPath(tool); err != nil {
		return "", fmt.Errorf("%s CLI is not installed", tool)
	}
	out, err := exec.CommandContext(ctx, tool, "issue", action, issueURL, bodyFlag, body).Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("%s issue %s failed: %s", tool, action, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}
