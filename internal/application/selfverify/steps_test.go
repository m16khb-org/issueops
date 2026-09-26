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
