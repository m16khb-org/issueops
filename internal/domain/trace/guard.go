package trace

import (
	"fmt"
	"sort"

	"issueops/internal/domain/policy"
)

func guardFindings(observed []string) []Finding {
	if len(observed) == 0 {
		return nil
	}
	byRule := map[string]int{}
	for _, rule := range observed {
		if rule == "" {
			rule = "guard_finding"
		}
		byRule[rule]++
	}
	rules := make([]string, 0, len(byRule))
	for rule := range byRule {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	out := []Finding{}
	for _, rule := range rules {
		out = append(out, Finding{
			FailureClass:        "guard_" + policy.RedactFreeform(rule),
			RecurringPattern:    fmt.Sprintf("%s reported %d time(s)", policy.RedactFreeform(rule), byRule[rule]),
			ProposedKnob:        "adjust guard rule documentation or source pattern only if repeated false positives are confirmed",
			OverfitRisk:         "medium: guard changes can overfit to one file; verify with fixture coverage",
			VerificationCommand: "go test ./internal/domain/guard ./internal/application/guard ./internal/adapter/guard -count=1",
		})
	}
	return out
}
