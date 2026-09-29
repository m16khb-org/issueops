package trace

import (
	"issueops/internal/domain/policy"
	traceclassification "issueops/internal/domain/traceclassification"
	"sort"
	"strings"
)

func NormalizeCause(cause string) string {
	switch cause {
	case "none", "model", "harness_environment", "transport", "contract_input", "unknown":
		return cause
	default:
		return "unknown"
	}
}

func NormalizeEvidence(items []Evidence) []Evidence {
	out := make([]Evidence, 0, len(items))
	for _, item := range items {
		out = append(out, Evidence{
			Cause:  NormalizeCause(item.Cause),
			Code:   policy.RedactFreeform(item.Code),
			Source: policy.RedactFreeform(item.Source),
		})
	}
	return out
}

func traceSortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqSortedTraceStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dedupeTraceFindings(findings []Finding) []Finding {
	normalized := make([]Finding, 0, len(findings))
	keys := make([]traceclassification.FindingKey, 0, len(findings))
	for _, finding := range findings {
		finding.FailureCause = NormalizeCause(finding.FailureCause)
		finding.FailureCauseEvidence = NormalizeEvidence(finding.FailureCauseEvidence)
		keys = append(keys, traceclassification.FindingKey{
			Index: len(normalized), FailureClass: finding.FailureClass, FailureCause: string(finding.FailureCause),
			RecurringPattern: finding.RecurringPattern, ProposedKnob: finding.ProposedKnob,
			OverfitRisk: finding.OverfitRisk, VerificationCommand: finding.VerificationCommand,
		})
		normalized = append(normalized, finding)
	}
	out := []Finding{}
	for _, index := range traceclassification.DeduplicateFindings(keys) {
		out = append(out, normalized[index])
	}
	return out
}
