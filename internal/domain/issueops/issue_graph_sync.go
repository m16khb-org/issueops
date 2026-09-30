package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type IssueGraphSyncPlan struct {
	Preview, Noop                bool
	Provider, URL, Body, Message string
	LinkCount                    int
}

func PlanIssueGraphSync(record model.IssueOpsRecord, confirm bool) (IssueGraphSyncPlan, error) {
	if !confirm {
		return IssueGraphSyncPlan{Preview: true, LinkCount: len(record.IssueLinks), Message: fmt.Sprintf("[dry-run] would sync %d issue graph links to remote issue %s", len(record.IssueLinks), record.IssueURL)}, nil
	}
	url := strings.TrimSpace(record.IssueURL)
	if url == "" {
		return IssueGraphSyncPlan{}, fmt.Errorf("cannot sync graph: no issue_url on cycle")
	}
	if len(record.IssueLinks) == 0 {
		return IssueGraphSyncPlan{Noop: true, Message: "no issue graph links to sync"}, nil
	}
	provider := ResolveRecordProvider(record)
	if provider == "" {
		return IssueGraphSyncPlan{}, fmt.Errorf("cannot determine provider from cycle")
	}
	if provider != "github" && provider != "gitlab" {
		return IssueGraphSyncPlan{}, fmt.Errorf("sync not supported for provider %q", provider)
	}
	rendered := RenderIssueGraphComment(record)
	count := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- **") {
			count++
		}
	}
	return IssueGraphSyncPlan{Provider: provider, URL: url, Body: rendered, LinkCount: count}, nil
}

// RenderIssueGraphComment lists the cycle's typed issue links for readers of
// the parent issue, in Korean and without harness values (#513).
func RenderIssueGraphComment(record model.IssueOpsRecord) string {
	var body strings.Builder
	body.WriteString("## 관련 이슈\n\n")
	for _, link := range record.IssueLinks {
		body.WriteString(fmt.Sprintf("- **%s**: %s", linkTypeLabel(link.Type), link.URL))
		if link.Title != "" {
			body.WriteString(fmt.Sprintf(" (%s)", link.Title))
		}
		body.WriteString("\n")
	}
	return body.String()
}

func linkTypeLabel(linkType string) string {
	switch linkType {
	case "depends-on":
		return "선행 이슈"
	case "blocks":
		return "이 이슈가 막는 이슈"
	case "supersedes":
		return "대체하는 이슈"
	case "follows-up":
		return "후속 이슈"
	case "duplicates":
		return "중복 이슈"
	case "splits-from":
		return "나뉘어 나온 원래 이슈"
	case "implements":
		return "구현하는 이슈"
	case "child":
		return "하위 작업"
	default:
		return linkType
	}
}
