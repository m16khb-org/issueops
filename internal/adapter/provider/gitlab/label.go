package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AddLabels checks that every label exists in the project, sends add_labels
// (add only), and reads the labels from the updated issue or merge request
// the API returns. GitLab creates a missing project label on add_labels, so a
// typo would add a new label to the whole project; such labels are refused.
func (Provider) AddLabels(ctx context.Context, repo, targetURL, kind string, labels []string) ([]string, error) {
	targetURL = strings.TrimSpace(targetURL)
	var hostname, project, iid, collection string
	var err error
	switch kind {
	case "issue":
		hostname, project, iid, err = parseGitLabIssueURL(targetURL)
		collection = "issues"
	case "pr":
		hostname, project, iid, err = parseGitLabMergeRequestURL(targetURL)
		collection = "merge_requests"
	default:
		return nil, fmt.Errorf("unsupported gitlab label target %q (want issue|pr)", kind)
	}
	if err != nil {
		return nil, err
	}
	projectEndpoint := "projects/" + url.PathEscape(project)
	for _, label := range labels {
		if _, err := runGlabAPIContext(ctx, repo, hostname, projectEndpoint+"/labels/"+url.PathEscape(label)); err != nil {
			return nil, fmt.Errorf("label %q is not defined in the project (add_labels would create it): %w", label, err)
		}
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, projectEndpoint+"/"+collection+"/"+iid, "--method", "PUT", "-f", "add_labels="+strings.Join(labels, ","))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab %s labels: %w", kind, err)
	}
	return payload.Labels, nil
}
