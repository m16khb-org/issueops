package issueops

import model "issueops/internal/contract/issueops"

func PRReadinessWarnings(record model.IssueOpsRecord) []string {
	var warnings []string
	if len(record.Decisions) == 0 {
		warnings = append(warnings, "no_decision_records")
	}
	if len(record.IssueLinks) == 0 {
		warnings = append(warnings, "no_issue_graph_links")
	}
	return warnings
}
