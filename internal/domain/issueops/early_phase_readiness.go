package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func PlanReadinessMissing(record model.IssueOpsRecord) []string {
	missing := IntentMissing(record)
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if PlanPrepGateApplies(record) {
		missing = append(missing, PlanPrepMissing(record.PlanPrep)...)
	}
	return missing
}

func GrillReadinessMissing(record model.IssueOpsRecord) []string {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if PlanPrepGateApplies(record) {
		missing = append(missing, PlanPrepMissing(record.PlanPrep)...)
	}
	missing = append(missing, SplitDecisionMissing(record)...)
	return append(missing, DomainReviewMissing(record)...)
}
