package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AddLabels sends add_labels (add only) and reads the labels from the
// updated issue or merge request the API returns.
func (Provider) AddLabels(ctx context.Context, repo, targetURL, kind string, labels []string) ([]string, error) {
	targetURL = strings.TrimSpace(targetURL)
	var hostname, endpoint string
	switch kind {
	case "issue":
		host, project, iid, err := parseGitLabIssueURL(targetURL)
		if err != nil {
			return nil, err
		}
		hostname, endpoint = host, "projects/"+url.PathEscape(project)+"/issues/"+iid
	case "pr":
		host, mergeRequest, err := gitLabMergeRequestEndpoint(targetURL)
		if err != nil {
			return nil, err
		}
		hostname, endpoint = host, mergeRequest
	default:
		return nil, fmt.Errorf("unsupported gitlab label target %q (want issue|pr)", kind)
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint, "--method", "PUT", "-f", "add_labels="+strings.Join(labels, ","))
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
