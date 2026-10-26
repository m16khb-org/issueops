package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
)

// SetMilestone runs `glab issue|mr update IID -R <project URL> --milestone
// TITLE` (glab resolves the title to the project milestone and refuses an
// unknown one) and reads the milestone back through the API.
func (Provider) SetMilestone(ctx context.Context, repo, targetURL, kind, title string) (string, error) {
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
		return "", fmt.Errorf("unsupported gitlab milestone target %q (want issue|pr)", kind)
	}
	if err != nil {
		return "", err
	}
	if _, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "glab", noun, "update", iid, "-R", "https://"+hostname+"/"+project, "--milestone", title); err != nil {
		return "", fmt.Errorf("glab %s update --milestone failed: %w", noun, err)
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint)
	if err != nil {
		return "", fmt.Errorf("milestone not read back: %w", err)
	}
	var payload struct {
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse gitlab %s milestone: %w", kind, err)
	}
	if payload.Milestone == nil {
		return "", nil
	}
	return payload.Milestone.Title, nil
}
