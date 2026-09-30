package selfverify

import (
	"reflect"
	"strings"
	"testing"
	"time"

	augment "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

func TestPythonScriptTestsUseRepositoryDiscoveryAndPropagateFailure(t *testing.T) {
	for _, ok := range []bool{false, true} {
		t.Run(map[bool]string{false: "fail", true: "pass"}[ok], func(t *testing.T) {
			calls := 0
			deps := SelfVerifyStepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
				calls++
				if root != "/repo" || label != "Python script tests" || timeout != 5*time.Minute || stdin != "" || name != "python3" || len(args) < 3 || args[0] != "-c" || !reflect.DeepEqual(args[2:], []string{"-m", "unittest", "discover", "-s", "scripts", "-p", "*_test.py"}) {
					t.Fatalf("unexpected Python command: %s %s %s %s %v", root, label, timeout, name, args)
				}
				if !strings.Contains(args[1], "sys.version_info") || !strings.Contains(args[1], "os.execv(sys.executable") {
					t.Fatalf("runtime check must reuse the checked interpreter: %s", args[1])
				}
				return StepResult{Label: label, OK: ok, DurationMS: 12, Stderr: "discovery evidence", Error: map[bool]string{false: "exit status 1", true: ""}[ok]}
			}}
			var goTest StepResult
			planned := PlannedSteps("/repo", "/tmp/issueops", 100, &goTest, deps)
			if planned[2].Label != "Python script tests" {
				t.Fatalf("Python must precede risk QA: %v", planned[2].Label)
			}
			step := planned[2].Run()
			if calls != 1 || step.OK != ok || step.DurationMS != 12 || step.Stderr != "discovery evidence" {
				t.Fatal(step)
			}
			steps := []StepResult{}
			for _, label := range domain.StepOrder() {
				steps = append(steps, StepResult{Label: label, OK: true})
			}
			steps[2] = step
			summary := SummarizeSelfVerification(augment.SelfAugmentResult{OK: ok, Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: steps}}}, 95)
			if summary.TerminationEligible != ok || (ok && summary.MinimumGoalScore != 100) || (!ok && summary.FailedStep != "Python script tests") {
				t.Fatal(summary)
			}
			for _, goal := range summary.GoalScores {
				if goal.Name == "test_suite" && (goal.TotalChecks != 3 || goal.Passed != ok) {
					t.Fatal(goal)
				}
			}
		})
	}
}
