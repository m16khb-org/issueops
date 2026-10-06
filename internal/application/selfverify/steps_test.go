package selfverify

import (
	contract "issueops/internal/contract/selfverify"
	"reflect"
	"testing"
	"time"
)

func TestFailedRiskRaceCannotReplaceFullSuite(t *testing.T) {
	var fullSuiteCalls int
	deps := SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "/repo" },
		ValidateRiskQATier: func(string) RiskQAEvidence {
			return RiskQAEvidence{Step: contract.StepResult{Label: "risk QA tier", OK: false}, CoversFullGoTest: true}
		},
		RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
			if root != "/repo" || label != "go test" || timeout != 10*time.Minute || stdin != "" || name != "go" || !reflect.DeepEqual(args, []string{"test", "./...", "-count=1"}) {
				return contract.StepResult{Label: label, OK: false, Error: "unexpected verification command"}
			}
			fullSuiteCalls++
			return contract.StepResult{Label: label, OK: true}
		},
	}
	var fullTest contract.StepResult
	planned := PlannedSteps("/repo", "/tmp/issueops", 100, &fullTest, deps)
	if planned[4].Run().OK || !planned[5].Run().OK || fullSuiteCalls != 1 {
		t.Fatalf("failed race reused as full suite: result=%+v calls=%d", fullTest, fullSuiteCalls)
	}
	if fullTest.Reused {
		t.Fatalf("fresh full suite marked reused: %+v", fullTest)
	}
}

func TestFailedGoldenCommandKeepsMeasuredEvidence(t *testing.T) {
	// Given
	want := contract.StepResult{Label: "contract golden tests", Command: "go test", DurationMS: 149, Error: "fixture failure", Stderr: "failed"}
	calls := 0
	deps := SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "/repo" },
		RunCommandStep: func(_ string, _ string, _ time.Duration, _ string, _ string, _ ...string) contract.StepResult {
			calls++
			return want
		},
	}

	// When
	got := CachedContractGoldenStep(contract.StepResult{OK: false}, deps)

	// Then
	if calls != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("failed measured child changed: got=%+v want=%+v calls=%d", got, want, calls)
	}
}

func TestRiskEvidenceIsLocalToEachPlannedRun(t *testing.T) {
	fullCalls := 0
	deps := SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "/repo" },
		ValidateRiskQATier: func(string) RiskQAEvidence {
			return RiskQAEvidence{Step: contract.StepResult{OK: true, Command: "go test -race ./... -count=1"}, CoversFullGoTest: true}
		},
		RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
			if root != "/repo" || label != "go test" || name != "go" || !reflect.DeepEqual(args, []string{"test", "./...", "-count=1"}) {
				t.Fatalf("unexpected command %s %s %s %v", root, label, name, args)
			}
			fullCalls++
			return contract.StepResult{Label: label, OK: false, Error: "second run failed"}
		},
	}
	var firstResult, secondResult contract.StepResult
	first := PlannedSteps("/repo", "/tmp/first", 1, &firstResult, deps)
	second := PlannedSteps("/repo", "/tmp/second", 2, &secondResult, deps)
	if !first[4].Run().OK || !first[5].Run().OK || fullCalls != 0 {
		t.Fatal("successful risk evidence was not reused in its own run")
	}
	if second[5].Run().OK || fullCalls != 1 {
		t.Fatalf("second run reused foreign evidence: %+v, calls=%d", secondResult, fullCalls)
	}
	if !first[6].Run().OK {
		t.Fatal("first run lost successful golden evidence")
	}
}
