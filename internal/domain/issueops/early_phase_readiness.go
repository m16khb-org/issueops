package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// PlanReadinessMissing lists the gates blocking the plan phase: the intent
// contract, the linked issue URL, and, where the plan-prep gate applies, the
// plan-prep items.
func PlanReadinessMissing(record model.Record) []string {
	missing := IntentMissing(record)
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if PlanPrepGateApplies(record) {
		missing = append(missing, PlanPrepMissing(record.PlanPrep)...)
	}
	return missing
}

// GrillReadinessMissing lists the gates blocking the grill phase: the issue
// URL, the branch, the plan-prep items where required, the split decision,
// and the domain review.
func GrillReadinessMissing(record model.Record) []string {
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
	missing = append(missing, splitDecisionMissing(record)...)
	return append(missing, domainReviewMissing(record)...)
}

// BranchEvidenceMissing lists the branch evidence still absent from record:
// the issue URL, the branch, the branch preparation, and its verified issue
// link.
func BranchEvidenceMissing(record model.Record) []string {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if record.BranchPrepare == nil {
		missing = append(missing, "branch_prepare")
	} else if !record.BranchPrepare.LinkVerified {
		missing = append(missing, "branch_link_verified")
	}
	return missing
}
