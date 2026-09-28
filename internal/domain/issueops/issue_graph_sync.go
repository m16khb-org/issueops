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
	var body strings.Builder
	body.WriteString("## Related Issue Graph\n\nThis issue graph was recorded by issueops IssueOps:\n\n")
	for _, link := range record.IssueLinks {
		fmt.Fprintf(&body, "- **%s**: %s", issueGraphLinkLabel(link.Type), link.URL)
		if link.Title != "" {
			fmt.Fprintf(&body, " (%s)", link.Title)
		}
		body.WriteString("\n")
	}
	fmt.Fprintf(&body, "\nCycle: `%s`", record.ID)
	rendered := body.String()
	count := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- **") {
			count++
		}
	}
	return IssueGraphSyncPlan{Provider: provider, URL: url, Body: rendered, LinkCount: count}, nil
}

func issueGraphLinkLabel(kind string) string {
	switch kind {
	case "depends-on":
		return "Depends on"
	case "blocks":
		return "Blocks"
	case "supersedes":
		return "Supersedes"
	case "follows-up":
		return "Follows up"
	case "duplicates":
		return "Duplicates"
	case "splits-from":
		return "Splits from"
	case "implements":
		return "Implements"
	default:
		return kind
	}
}
