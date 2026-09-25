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
		return PlanPrepItem{}, fmt.Errorf("plan_prep %s: provide evidence or a waive reason", name)
	}
	if waiveReason != "" {
		return PlanPrepItem{Status: "waived", WaiveReason: policy.RedactFreeform(waiveReason)}, nil
	}
	return PlanPrepItem{Status: "evidence", Evidence: evidence}, nil
}
