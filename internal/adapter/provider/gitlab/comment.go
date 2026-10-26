package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Comment posts one note through the notes API and builds the note URL from
// the id the API returned.
func (Provider) Comment(ctx context.Context, repo, targetURL, kind, body string) (string, error) {
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
		host, mergeRequest, err := gitLabMergeRequestEndpoint(targetURL)
		if err != nil {
			return "", err
		}
		hostname, endpoint = host, mergeRequest
	default:
		return "", fmt.Errorf("unsupported gitlab comment target %q (want issue|pr)", kind)
	}
	out, err := runGlabAPIContext(ctx, repo, hostname, endpoint+"/notes", "--method", "POST", "-f", "body="+body)
	if err != nil {
		return "", err
	}
	var note struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(out, &note); err != nil || note.ID <= 0 {
		return "", fmt.Errorf("gitlab note response has no id; check the %s before posting again", kind)
	}
	return fmt.Sprintf("%s#note_%d", targetURL, note.ID), nil
}
