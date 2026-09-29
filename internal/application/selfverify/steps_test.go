package selfverify

import (
	"reflect"
	"testing"
	"time"
)

func TestFailedRiskRaceCannotReplaceFullSuite(t *testing.T) {
	var fullSuiteCalls int
	deps := SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "/repo" },
		ValidateRiskQATier: func(string) RiskQAEvidence {
			return RiskQAEvidence{Step: StepResult{Label: "risk QA tier", OK: false}, CoversFullGoTest: true}
		},
		RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
			if root != "/repo" || label != "go test" || timeout != 10*time.Minute || stdin != "" || name != "go" || !reflect.DeepEqual(args, []string{"test", "./...", "-count=1"}) {
				return StepResult{Label: label, OK: false, Error: "unexpected verification command"}
			}
			fullSuiteCalls++
			return StepResult{Label: label, OK: true}
		},
	}
	var fullTest StepResult
	planned := PlannedSteps("/repo", "/tmp/issueops", 100, &fullTest, deps)
	if planned[2].Run().OK || !planned[3].Run().OK || fullSuiteCalls != 1 {
		t.Fatalf("failed race reused as full suite: result=%+v calls=%d", fullTest, fullSuiteCalls)
	}
}

func TestRiskEvidenceIsLocalToEachPlannedRun(t *testing.T) {
	fullCalls := 0
	deps := SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "/repo" },
		ValidateRiskQATier: func(string) RiskQAEvidence {
			return RiskQAEvidence{Step: StepResult{OK: true, Command: "go test -race ./... -count=1"}, CoversFullGoTest: true}
		},
		RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
			if root != "/repo" || label != "go test" || name != "go" || !reflect.DeepEqual(args, []string{"test", "./...", "-count=1"}) {
				t.Fatalf("unexpected command %s %s %s %v", root, label, name, args)
			}
			fullCalls++
			return StepResult{Label: label, OK: false, Error: "second run failed"}
		},
	}
	var firstResult, secondResult StepResult
	first := PlannedSteps("/repo", "/tmp/first", 1, &firstResult, deps)
	second := PlannedSteps("/repo", "/tmp/second", 2, &secondResult, deps)
	if !first[2].Run().OK || !first[3].Run().OK || fullCalls != 0 {
		t.Fatal("successful risk evidence was not reused in its own run")
	}
	if second[3].Run().OK || fullCalls != 1 {
		t.Fatalf("second run reused foreign evidence: %+v, calls=%d", secondResult, fullCalls)
	}
	if !first[4].Run().OK {
		t.Fatal("first run lost successful golden evidence")
	}
}
