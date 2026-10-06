package selfverify

import (
	"errors"
	contract "issueops/internal/contract/selfverify"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	augment "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

func TestGoMatchGuardOrderCommandAndFailurePropagation(t *testing.T) {
	want := contract.StepResult{Label: "Go test match guard", Command: "bash scripts/verify-go-test-match-test.sh", Error: "exit status 23", Stdout: "match evidence", Stderr: "fixture failure"}
	calls := 0
	var goTest contract.StepResult
	planned := PlannedSteps("/repo", "/tmp/issueops", 100, &goTest, SelfVerifyStepDeps{
		RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
			calls++
			if root != "/repo" || label != want.Label || timeout != 30*time.Second || stdin != "" || name != "bash" || !reflect.DeepEqual(args, []string{"scripts/verify-go-test-match-test.sh"}) {
				t.Fatalf("unexpected guard command: %s %s %s %s %v", root, label, timeout, name, args)
			}
			return want
		},
	})
	count := 0
	for _, step := range planned {
		if step.Label == want.Label {
			count++
		}
	}
	if count != 1 || planned[2].Label != "Python script tests" || planned[3].Label != want.Label || planned[4].Label != "risk QA tier" {
		t.Fatalf("guard missing or misplaced: count=%d steps=%v", count, domain.StepOrder())
	}
	if got := planned[3].Run(); !reflect.DeepEqual(got, want) || calls != 1 || goTest.OK {
		t.Fatalf("guard evidence changed or replaced full Go tests: %+v calls=%d", got, calls)
	}
}

func TestGoMatchGuardMissingOrFailedEvidencePreventsCompletion(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing", false: "failed"}[missing], func(t *testing.T) {
			steps := []contract.StepResult{}
			for _, label := range domain.StepOrder() {
				if label == "Go test match guard" && missing {
					continue
				}
				steps = append(steps, contract.StepResult{Label: label, OK: label != "Go test match guard"})
			}
			summary := SummarizeSelfVerification(augment.SelfAugmentResult{OK: missing, Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: steps}}}, 95)
			if summary.TerminationEligible {
				t.Fatalf("incomplete guard evidence allowed completion: %+v", summary)
			}
			if missing && (len(summary.CoverageGaps) != 1 || !strings.Contains(summary.CoverageGaps[0], "Go test match guard")) {
				t.Fatalf("missing guard coverage not reported: %+v", summary)
			}
			if !missing && (summary.FailedStep != "Go test match guard" || len(summary.RerunCommands) != 2 || summary.RerunCommands[0] != "bash scripts/verify-go-test-match-test.sh") {
				t.Fatalf("guard recovery lost: %+v", summary)
			}
			for _, goal := range summary.GoalScores {
				if goal.Name == "test_suite" && (goal.TotalChecks != 4 || goal.Passed) {
					t.Fatalf("guard did not gate test suite: %+v", goal)
				}
			}
		})
	}
}

func TestGoMatchGuardFailureStopsLoop(t *testing.T) {
	commandCalls := 0
	result, err := ExecuteLoop(LoopRequest{TargetScore: 95}, LoopDeps{
		IssueOpsRoot: func() string { return "/repo" }, Now: time.Now,
		MkdirTemp:      func() (string, error) { return "/temp", nil },
		RemoveAll:      func(string) error { return nil },
		TempBinaryPath: func(string) string { return "/temp/issueops" },
		Summarize:      SummarizeSelfVerification,
		StepDeps: SelfVerifyStepDeps{
			ValidateHarnessInvariants: func(string) contract.StepResult { return contract.StepResult{Label: "harness invariants", OK: true} },
			ValidateGoFormat:          func(string) contract.StepResult { return contract.StepResult{Label: "gofmt", OK: true} },
			RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) contract.StepResult {
				commandCalls++
				return contract.StepResult{Label: label, OK: label == "Python script tests", Error: "exit status 23", Stderr: "guard failure"}
			},
			ValidateRiskQATier: func(string) RiskQAEvidence { t.Fatal("risk QA ran after guard failure"); return RiskQAEvidence{} },
		},
	})
	if !errors.Is(err, ErrSelfVerificationGateFailed) || commandCalls != 2 || result.OK || result.TerminationEligible || result.Summary.FailedStep != "Go test match guard" {
		t.Fatalf("guard failure did not stop completion: result=%+v calls=%d err=%v", result, commandCalls, err)
	}
}

func TestGoMatchGuardRunsRealScriptFixtures(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	var goTest contract.StepResult
	planned := PlannedSteps(root, "/tmp/issueops", 100, &goTest, SelfVerifyStepDeps{RunCommandStep: evidenceRunner})
	if planned[3].Label != "Go test match guard" {
		t.Fatalf("guard missing: %s", planned[3].Label)
	}
	step := planned[3].Run()
	if !step.OK || !strings.Contains(step.Stdout, "verify-go-test-match fixtures passed") || !strings.Contains(step.Stdout, `"Test":"TestWanted"`) {
		t.Fatalf("real match fixtures failed: %+v", step)
	}
}
