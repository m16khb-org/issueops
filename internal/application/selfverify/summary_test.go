package selfverify

import (
	"encoding/json"
	cause "issueops/internal/contract/failurecause"
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
	"slices"
	"strings"
	"testing"
)

func TestSummaryCarriesReusedMarkersWithoutLosingCoverage(t *testing.T) {
	// Given
	var input augment.Result
	if err := json.Unmarshal([]byte(`{"ok":true,"runs":[{"iteration":1,"seed":123,"steps":[
		{"label":"go test","ok":true,"duration_ms":137},
		{"label":"go test","ok":true,"duration_ms":0,"reused":true},
		{"label":"contract golden tests","ok":true,"duration_ms":0,"reused":true}
	]}]}`), &input); err != nil {
		t.Fatal(err)
	}

	// When
	summary := SummarizeSelfVerification(input, 95)

	// Then
	if summary.TotalSteps != 3 || summary.PassedSteps != 3 || len(summary.SlowestSteps) != 1 || summary.SlowestSteps[0].DurationMS != 137 {
		t.Errorf("reuse evidence was lost or timed: %+v", summary)
	}
	data, err := json.Marshal(summary.StepDurationStats)
	if err != nil {
		t.Fatal(err)
	}
	var stats []struct {
		Label         string `json:"label"`
		Count         int    `json:"count"`
		ReusedCount   int    `json:"reused_count"`
		P95DurationMS int64  `json:"p95_duration_ms"`
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 || stats[0].Label != "contract golden tests" || stats[0].Count != 0 || stats[0].ReusedCount != 1 ||
		stats[1].Label != "go test" || stats[1].Count != 1 || stats[1].ReusedCount != 1 || stats[1].P95DurationMS != 137 {
		t.Fatalf("summary stats = %s", data)
	}
}

func TestSummaryComposesCoverageFailureEvidenceAndRecoveryHints(t *testing.T) {
	steps := []verify.StepResult{}
	for _, label := range domain.StepOrder() {
		steps = append(steps, verify.StepResult{Label: label, OK: true, DurationMS: 5})
	}
	input := augment.Result{OK: true, Iterations: 1, BaseSeed: 123, Runs: []augment.Iteration{{Iteration: 1, Seed: 123, Steps: steps}}}
	success := SummarizeSelfVerification(input, 95)
	if !success.TerminationEligible || success.MinimumGoalScore != 100 || len(success.CoverageGaps) != 0 || success.TotalSteps != 27 || success.FailedSteps != 0 {
		t.Fatalf("complete evidence was lost: %+v", success)
	}
	if success.FailureCause != cause.None || success.FailureCauseReason != "no_failed_steps" {
		t.Fatalf("passing evidence classified a failure cause: %+v", success)
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
	empty := SummarizeSelfVerification(augment.Result{}, 95)
	if empty.MinimumGoalScore != 0 || empty.TerminationEligible || empty.TotalSteps != 0 || len(empty.CoverageGaps) == 0 {
		t.Fatalf("empty evidence passed: %+v", empty)
	}
}

func TestSummaryMarksStepsAndGoalsSkippedByFailFast(t *testing.T) {
	// Given: fail-fast stopped at "Python script tests" (third planned step).
	order := domain.StepOrder()
	input := augment.Result{Iterations: 1, BaseSeed: 100, Runs: []augment.Iteration{{Iteration: 1, Seed: 100, Steps: []verify.StepResult{
		{Label: order[0], OK: true},
		{Label: order[1], OK: true},
		{Label: order[2], OK: false},
	}}}}

	// When
	summary := SummarizeSelfVerification(input, 95)

	// Then: the unexecuted plan is explicit and distinct from the failed step.
	if summary.PlannedSteps != len(order) || summary.FailedStep != "Python script tests" || len(summary.NotRunSteps) != len(order)-3 || summary.NotRunSteps[0] != order[3] {
		t.Fatalf("planned=%d failed=%q not_run=%v", summary.PlannedSteps, summary.FailedStep, summary.NotRunSteps)
	}
	if slices.Contains(summary.NotRunSteps, "Python script tests") {
		t.Fatalf("a failed step is not a not-run step: %v", summary.NotRunSteps)
	}
	goals := map[string]verify.SelfVerificationGoalScore{}
	for _, goal := range summary.GoalScores {
		goals[goal.Name] = goal
	}
	// test_suite ran one of its checks (it failed) -> scored, not "not run"; build_release never ran.
	if goals["test_suite"].NotRun || goals["format_parity"].NotRun || goals["format_parity"].Score != 100 {
		t.Fatalf("executed goals: %+v", goals)
	}
	if !goals["build_release"].NotRun || goals["build_release"].Score != 0 || goals["build_release"].Passed {
		t.Fatalf("skipped goal must keep score 0/passed false and add not_run: %+v", goals["build_release"])
	}
	coverage := map[string]verify.SelfVerificationCoverage{}
	for _, item := range summary.Coverage {
		coverage[item.Claim] = item
	}
	if !coverage["release build artifact"].NotRun || coverage["core repository invariants"].NotRun {
		t.Fatalf("coverage markers: %+v", coverage)
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"planned_steps":27`, `"not_run_steps":["Go test match guard"`, `"not_run":true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("summary JSON lacks %s: %s", want, data)
		}
	}
}

func TestSummaryOfCompletePassReportsNoNotRunSteps(t *testing.T) {
	steps := []verify.StepResult{}
	for _, label := range domain.StepOrder() {
		steps = append(steps, verify.StepResult{Label: label, OK: true})
	}
	summary := SummarizeSelfVerification(augment.Result{OK: true, Iterations: 1, Runs: []augment.Iteration{{Iteration: 1, Steps: steps}}}, 95)
	if summary.PlannedSteps != len(steps) || summary.NotRunSteps == nil || len(summary.NotRunSteps) != 0 {
		t.Fatalf("planned=%d not_run=%#v", summary.PlannedSteps, summary.NotRunSteps)
	}
	for _, goal := range summary.GoalScores {
		if goal.NotRun {
			t.Fatalf("goal %s marked not run after a complete pass", goal.Name)
		}
	}
}
