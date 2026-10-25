package issueopsintent

import (
	"fmt"
	"strings"

	"issueops/internal/domain/policy"
)

type PlanPrepItem struct {
	Status      string
	Evidence    []string
	WaiveReason string
}

func BuildPlanPrepItem(name string, evidence []string, waiveReason string) (PlanPrepItem, error) {
	evidence = CleanTextValues(evidence)
	waiveReason = strings.TrimSpace(waiveReason)
	if len(evidence) > 0 && waiveReason != "" {
		return PlanPrepItem{}, fmt.Errorf("plan_prep %s: evidence and waive_reason are mutually exclusive", name)
	}
	if len(evidence) == 0 && waiveReason == "" {
		// Probe: the snake_case name did not say which flags to pass.
		flag := map[string]string{"decisions": "decisions", "related_issues": "related", "web_research": "web-research", "codebase_survey": "codebase-survey"}[name]
		evidenceFlag := "--" + flag + "-evidence"
		if flag == "related" {
			evidenceFlag = "--related-score-ref"
		}
		if flag == "" {
			return PlanPrepItem{}, fmt.Errorf("plan_prep %s: provide evidence or a waive reason", name)
		}
		return PlanPrepItem{}, fmt.Errorf("plan_prep %s: provide evidence or a waive reason (%s TEXT or --%s-waive TEXT)", name, evidenceFlag, flag)
	}
	if waiveReason != "" {
		return PlanPrepItem{Status: "waived", WaiveReason: policy.RedactFreeform(waiveReason)}, nil
	}
	return PlanPrepItem{Status: "evidence", Evidence: evidence}, nil
}
