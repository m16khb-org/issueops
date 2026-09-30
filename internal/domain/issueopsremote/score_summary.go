package remote

import (
	"fmt"
	"strings"
)

func RenderScoreSummary(result IssueOpsRemoteScoringResult) string {
	parts := []string{fmt.Sprintf("threshold %.2f", result.Threshold)}
	if len(result.SelectedRelatedIssues) > 0 {
		parts = append(parts, "선택 관련 이슈: "+joinScoredItems(result.SelectedRelatedIssues))
	}
	if len(result.RejectedRelatedIssues) > 0 {
		parts = append(parts, "거절 관련 이슈: "+joinScoredItems(result.RejectedRelatedIssues))
	}
	if len(result.SelectedLabels) > 0 {
		parts = append(parts, "선택 라벨: "+joinScoredItems(result.SelectedLabels))
	}
	if len(result.RejectedLabels) > 0 {
		parts = append(parts, "거절 라벨: "+joinScoredItems(result.RejectedLabels))
	}
	return strings.Join(parts, "\n")
}

func joinScoredItems(items []IssueOpsRemoteScoredItem) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmpty(item.Name, item.ID, item.Title, item.URL)
		if name == "" {
			name = "unknown"
		}
		out = append(out, fmt.Sprintf("%s(%.2f)", name, item.Score))
	}
	return strings.Join(out, ", ")
}
