package remoteverification

import (
	"fmt"
	model "issueops/internal/contract/remoteverification"
	policydomain "issueops/internal/domain/policy"
	"issueops/internal/domain/remoteparse"
	"net/url"
	"strings"
)

func ChildTarget(childURL string) (model.ChildTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(childURL))
	if err != nil {
		return model.ChildTarget{}, err
	}
	target := model.ChildTarget{URL: strings.TrimSpace(childURL), Hostname: parsed.Hostname()}
	if parsed.Hostname() == "github.com" {
		target.Provider = "github"
		return target, nil
	}
	if strings.Contains(parsed.Hostname(), "gitlab") || strings.Contains(parsed.EscapedPath(), "/-/issues/") || strings.Contains(parsed.EscapedPath(), "/-/work_items/") {
		parts := remoteparse.SplitGitLabIssuePath(parsed.EscapedPath())
		if parts.Project == "" || parts.IID == "" {
			return model.ChildTarget{}, fmt.Errorf("child_url must be a GitLab issue or work item URL")
		}
		target.Provider = "gitlab"
		target.Project = parts.Project
		target.IID = parts.IID
		target.Kind = parts.Kind
		if target.Kind == "" {
			target.Kind = "issues"
		}
	}
	return target, nil
}
func ValidateTask(iid string, metadata model.TaskMetadata) error {
	if metadata.Empty {
		return fmt.Errorf("gitlab issues/%s fallback did not return task metadata", iid)
	}
	if strings.EqualFold(metadata.Type, "TASK") || strings.EqualFold(metadata.IssueType, "task") {
		return nil
	}
	return fmt.Errorf("gitlab issues/%s fallback did not return a Task work item: type=%q issue_type=%q", iid, policydomain.BoundedDiagnostic(metadata.Type, 256), policydomain.BoundedDiagnostic(metadata.IssueType, 256))
}
