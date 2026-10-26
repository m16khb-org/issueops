package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Retitle sets the title through the issue or merge request PUT and reads it
// from the response. GitLab marks a draft MR by its title prefix, so a draft
// keeps that prefix: renaming must not silently mark the MR ready (that is
// `remote mark-ready`). The returned title has the prefix removed.
func (Provider) Retitle(ctx context.Context, repo, targetURL, kind, title string) (string, error) {
	targetURL = strings.TrimSpace(targetURL)
	var hostname, endpoint string
	switch kind {
	case "issue":
		host, project, iid, err := parseGitLabIssueURL(targetURL)
		if err != nil {
			return "", err
		}
		hostname, endpoint = host, "projects/"+url.PathEscape(project)+"/issues/"+iid
	case "pr":
		if gitLabDraftTitle.MatchString(title) {
			return "", fmt.Errorf("the title starts with a draft marker; draft state is not changed by a rename (mark-ready clears it)")
		}
		host, mergeRequest, err := gitLabMergeRequestEndpoint(targetURL)
		if err != nil {
			return "", err
		}
		hostname, endpoint = host, mergeRequest
		current, err := runGlabAPIContext(ctx, repo, hostname, endpoint)
		if err != nil {
			return "", err
		}
		state, _, err := parseGlabMergeState(current)
		if err != nil {
			return "", err
		}
		if state.Draft {
			title = "Draft: " + title
		}
	default:
		return "", fmt.Errorf("unsupported gitlab title target %q (want issue|pr)", kind)
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint, "--method", "PUT", "-f", "title="+title)
	if err != nil {
		return "", err
	}
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gitlab %s title: %w", kind, err)
	}
	if kind == "pr" {
		return gitLabDraftTitle.ReplaceAllString(payload.Title, ""), nil
	}
	return payload.Title, nil
}
