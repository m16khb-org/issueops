package trace

import (
	"fmt"
	"strings"

	"issueops/internal/domain/policy"
	"issueops/internal/domain/traceclassification"
)

func selfVerifySummaryFindings(summary Summary) []Finding {
	failedSteps, failedStep, failureClass := summary.FailedSteps, summary.FailedStep, summary.FailureClass
	if failedSteps == 0 && failedStep == "" && failureClass == "" {
		return nil
	}
	if failureClass == "" {
		failureClass = "self_verification_failure"
	}
	pattern := failedStep
	if pattern == "" {
		pattern = "self-verification reported failed steps"
	}
	parts := []string{}
	for _, cluster := range summary.Clusters {
		if cluster.Step != "" {
			parts = append(parts, fmt.Sprintf("%s failed %d time(s)", policy.RedactFreeform(cluster.Step), cluster.Count))
		}
	}
	if len(parts) > 0 {
		pattern = strings.Join(parts, "; ")
	}
	command := classification.DefaultVerificationCommand(failedStep)
	if len(summary.RerunCommands) > 0 {
		command = policy.RedactFreeform(summary.RerunCommands[0])
	}
	return []Finding{{
		FailureClass:         policy.RedactFreeform(failureClass),
		FailureCause:         summary.Cause,
		FailureCauseEvidence: summary.Evidence,
		RecurringPattern:     policy.RedactFreeform(pattern),
		ProposedKnob:         classification.ProposedKnobForStep(failedStep),
		OverfitRisk:          classification.OverfitRiskForClass(failureClass),
		VerificationCommand:  command,
	}}
}

func traceTypesForJSON(doc Document) []string {
	types := []string{}
	if doc.HasSummary || doc.TopFailureClass != "" || doc.TopFailedSteps > 0 {
		types = append(types, "self_verify_summary")
	}
	if doc.HasGuard {
		types = append(types, "guard_result")
	}
	if doc.Upkeep.Kind != "" || doc.Upkeep.Summary != "" {
		types = append(types, "doc_upkeep_json")
	}
	return types
}
