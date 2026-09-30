package issueops

import (
	"fmt"
	"os/exec"
	"strings"
)

type IssueGraphPoster struct{}

func (IssueGraphPoster) Post(provider, issueURL, body string) (string, error) {
	tool, action, bodyFlag := "gh", "comment", "--body"
	if provider == "gitlab" {
		tool, action, bodyFlag = "glab", "note", "--message"
	}
	if _, err := exec.LookPath(tool); err != nil {
		return "", fmt.Errorf("%s CLI is not installed", tool)
	}
	out, err := exec.Command(tool, "issue", action, issueURL, bodyFlag, body).Output()
	if err != nil {
		stderr := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("%s issue %s failed: %s", tool, action, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}
