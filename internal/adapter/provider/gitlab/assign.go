package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// AddAssignees runs `glab issue|mr update IID -R <project URL> --assignee
// +USER` (the + prefix adds; an unprefixed value would replace every
// assignee) and reads the assignees back through the API.
func (Provider) AddAssignees(ctx context.Context, repo, targetURL, kind string, users []string) ([]string, error) {
	targetURL = strings.TrimSpace(targetURL)
	var hostname, project, iid, noun, endpoint string
	var err error
	switch kind {
	case "issue":
		hostname, project, iid, err = parseGitLabIssueURL(targetURL)
		noun, endpoint = "issue", "projects/"+url.PathEscape(project)+"/issues/"+iid
	case "pr":
		hostname, project, iid, err = parseGitLabMergeRequestURL(targetURL)
		noun, endpoint = "mr", "projects/"+url.PathEscape(project)+"/merge_requests/"+iid
	default:
		return nil, fmt.Errorf("unsupported gitlab assignee target %q (want issue|pr)", kind)
	}
	if err != nil {
		return nil, err
	}
	args := []string{noun, "update", iid, "-R", "https://" + hostname + "/" + project}
	for _, user := range users {
		args = append(args, "--assignee", "+"+user)
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "glab", args...); err != nil {
		return nil, fmt.Errorf("glab %s update --assignee failed: %w", noun, err)
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint)
	if err != nil {
		return nil, fmt.Errorf("assignees not read back: %w", err)
	}
	var payload struct {
		Assignees []struct {
			Username string `json:"username"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab %s assignees: %w", kind, err)
	}
	names := make([]string, 0, len(payload.Assignees))
	for _, assignee := range payload.Assignees {
		names = append(names, assignee.Username)
	}
	return names, nil
}
