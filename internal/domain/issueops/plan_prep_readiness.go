package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// PlanPrepGateApplies leaves the missing intent contract as the first readiness error.
func PlanPrepGateApplies(record model.Record) bool {
	return record.Intent != nil && !strings.EqualFold(strings.TrimSpace(record.Intent.IntentClass), "trivial")
}

// PlanPrepMissing lists the plan-prep items still not satisfied: prior
// decisions, related issues, web research, and codebase survey. An item
// counts when it has usable evidence or a waive reason; a nil prep misses all
// four.
func PlanPrepMissing(prep *model.PlanPrep) []string {
	if prep == nil {
		return []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"}
	}
	missing := []string{}
	if !planPrepItemValid(prep.PriorDecisions) {
		missing = append(missing, "plan_prep_decisions")
	}
	if !planPrepItemValid(prep.RelatedIssues) {
		missing = append(missing, "plan_prep_related_issues")
	}
	if !planPrepItemValid(prep.WebResearch) {
		missing = append(missing, "plan_prep_web_research")
	}
	if !planPrepItemValid(prep.CodebaseSurvey) {
		missing = append(missing, "plan_prep_codebase_survey")
	}
	return missing
}

func planPrepItemValid(item model.PlanPrepItem) bool {
	switch strings.TrimSpace(item.Status) {
	case "evidence":
		return hasUsableEvidence(item.Evidence)
	case "waived":
		return strings.TrimSpace(item.WaiveReason) != ""
	default:
		return false
	}
}

func hasUsableEvidence(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && !strings.Contains(value, "\x00") {
			return true
		}
	}
	return false
}
