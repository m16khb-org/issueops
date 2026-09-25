package issueops

import (
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

// ValidateRecordInvariants checks relationships between fields of a persisted cycle.
func ValidateRecordInvariants(record issueopscontract.IssueOpsRecord) error {
	if record.PlanPrep != nil {
		for _, item := range []struct {
			name  string
			value issueopscontract.IssueOpsPlanPrepItem
		}{
			{name: "prior_decisions", value: record.PlanPrep.PriorDecisions},
			{name: "related_issues", value: record.PlanPrep.RelatedIssues},
			{name: "web_research", value: record.PlanPrep.WebResearch},
			{name: "codebase_survey", value: record.PlanPrep.CodebaseSurvey},
		} {
			if err := validatePlanPrepItem(item.name, item.value); err != nil {
				return err
			}
		}
	}
	if record.IssueCreateIntent != nil {
		if err := ValidateIssueCreateIntentInvariants(*record.IssueCreateIntent); err != nil {
			return err
		}
	}
	if record.IssueCreateIntent != nil &&
		record.IssueCreateIntent.Status == issueopscontract.IssueCreateIntentCompleted &&
		strings.TrimSpace(record.IssueURL) != strings.TrimSpace(record.IssueCreateIntent.CanonicalURL) {
		return fmt.Errorf("completed issue create intent canonical_url must match issue_url")
	}
	return nil
}

func validatePlanPrepItem(name string, item issueopscontract.IssueOpsPlanPrepItem) error {
	switch item.Status {
	case "evidence":
		if len(item.Evidence) == 0 || strings.TrimSpace(item.WaiveReason) != "" {
			return fmt.Errorf("issueops plan_prep %s evidence is invalid", name)
		}
	case "waived":
		if len(item.Evidence) != 0 || strings.TrimSpace(item.WaiveReason) == "" {
			return fmt.Errorf("issueops plan_prep %s waiver is invalid", name)
		}
	default:
		return fmt.Errorf("issueops plan_prep %s status %q is invalid", name, item.Status)
	}
	return nil
}
