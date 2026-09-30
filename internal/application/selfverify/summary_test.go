package selfverify

import (
	cause "issueops/internal/contract/failurecause"
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
	"strings"
	"testing"
)

func TestSummaryComposesCoverageFailureEvidenceAndRecoveryHints(t *testing.T) {
	steps := []verify.StepResult{}
	for _, label := range domain.StepOrder() {
		steps = append(steps, verify.StepResult{Label: label, OK: true, DurationMS: 5})
	}
	input := augment.SelfAugmentResult{OK: true, Iterations: 1, BaseSeed: 123, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Seed: 123, Steps: steps}}}
	success := SummarizeSelfVerification(input, 95)
	if !success.TerminationEligible || success.MinimumGoalScore != 100 || len(success.CoverageGaps) != 0 || success.TotalSteps != 27 || success.FailedSteps != 0 {
		t.Fatalf("complete evidence was lost: %+v", success)
	}
	equal := SummarizeSelfVerification(input, 100)
	if equal.TerminationEligible {
		t.Fatal("score equal to target passed")
	}
	input.OK = false
	// Successful steps can carry diagnostic evidence; only failed steps contribute to cause classification.
	input.Runs[0].Steps[0].FailureEvidence = []cause.Evidence{{Cause: "harness", Code: "ignore-success", Source: "fixture"}}
	for i := range input.Runs[0].Steps {
		if input.Runs[0].Steps[i].Label == "go test" {
			input.Runs[0].Steps[i].OK = false
			input.Runs[0].Steps[i].FailureEvidence = []cause.Evidence{{Cause: "model", Code: "failed-check", Source: "fixture"}}
		}
	}
	failure := SummarizeSelfVerification(input, 95)
	if failure.TerminationEligible || failure.FailedSteps != 1 || failure.PassedSteps != 26 || failure.FailedStep != "go test" || failure.FailedSeed != 123 || failure.FailureCause != "model" || len(failure.FailureCauseEvidence) != 1 || failure.FailureCauseEvidence[0].Code != "failed-check" {
		t.Fatalf("failure composition drift: %+v", failure)
	}
	if len(failure.RerunCommands) != 2 || failure.RerunCommands[0] != "go test ./... -count=1" || !strings.Contains(failure.RerunCommands[1], "--seed=123 --target-score=95") {
		t.Fatal(failure.RerunCommands)
	}
	empty := SummarizeSelfVerification(augment.SelfAugmentResult{}, 95)
	if empty.MinimumGoalScore != 0 || empty.TerminationEligible || empty.TotalSteps != 0 || len(empty.CoverageGaps) == 0 {
		t.Fatalf("empty evidence passed: %+v", empty)
	}
}
