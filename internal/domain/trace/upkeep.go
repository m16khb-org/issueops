package trace

import (
	"fmt"
	"strings"

	"issueops/internal/domain/policy"
)

func docUpkeepFindings(events []Upkeep) []Finding {
	if len(events) == 0 {
		return nil
	}
	byTarget := map[string]int{}
	for _, event := range events {
		target := strings.Join(event.TargetDocs, ",")
		if target == "" {
			target = event.Kind
		}
		if target == "" {
			target = "doc_upkeep"
		}
		byTarget[target]++
	}
	findings := []Finding{}
	for _, target := range traceSortedIntKeys(byTarget) {
		findings = append(findings, Finding{
			FailureClass:        "lifecycle_doc_upkeep",
			RecurringPattern:    fmt.Sprintf("%s queued %d time(s)", policy.RedactFreeform(target), byTarget[target]),
			ProposedKnob:        "route pending upkeep into completion evidence instead of leaving lifecycle queue stale",
			OverfitRisk:         "low: append-only lifecycle reminders should not alter task execution",
			VerificationCommand: "go test ./internal/adapter/lifecycle -run Lifecycle -count=1",
		})
	}
	return findings
}
