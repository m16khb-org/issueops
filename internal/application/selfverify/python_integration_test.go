package selfverify

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/verification"
	augment "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

func TestPythonDiscoveryRuntimeAndEarlyFailure(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	python, err = filepath.Abs(python)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"pass", "fail", "missing", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git fixture: %s %v", out, err)
			}
			if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
				t.Fatal(err)
			}
			copyPythonSuiteRunner(t, root)
			body := "import unittest\nfrom pathlib import Path\nPath('discovery-ran').write_text('yes')\nclass DiscoveryTest(unittest.TestCase):\n    def test_result(self):\n        self.assertTrue(" + map[bool]string{true: "True", false: "False"}[mode == "pass"] + ")\n"
			if err := os.WriteFile(filepath.Join(root, "scripts", "fixture_test.py"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "missing" || mode == "unsupported" {
				tools := t.TempDir()
				if mode == "unsupported" {
					// Execute the real runtime-check source with a controlled unsupported version.
					shim := "#!" + python + "\nimport sys\nsys.version_info=(3,9,0)\nsys.version='3.9.0 fixture'\nexec(sys.argv[2])\n"
					if err := os.WriteFile(filepath.Join(tools, "python3"), []byte(shim), 0700); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("PATH", tools)
			}
			deps := SelfVerifyStepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
				return verification.Run(root, label, timeout, stdin, 4096, name, args...)
			}}
			var goTest StepResult
			step := PlannedSteps(root, "unused", 100, &goTest, deps)[2].Run()
			wantOK := mode == "pass"
			if step.OK != wantOK || step.Label != "Python script tests" {
				t.Fatalf("%s: %+v", mode, step)
			}
			_, markerErr := os.Stat(filepath.Join(root, "discovery-ran"))
			ran := markerErr == nil
			if ran != (mode == "pass" || mode == "fail") {
				t.Fatalf("discovery ran=%v: %+v", ran, step)
			}
			switch mode {
			case "missing":
				if !strings.Contains(step.Error, "executable file not found") {
					t.Fatal(step)
				}
			case "unsupported":
				if !strings.Contains(step.Stderr, "require Python 3.10+; found 3.9") {
					t.Fatal(step)
				}
			case "fail":
				if !strings.Contains(step.Stderr, "FAILED (failures=1)") {
					t.Fatal(step)
				}
			case "pass":
				if !strings.Contains(step.Stderr, "OK") || !strings.Contains(step.Stdout, "Python runtime:") {
					t.Fatal(step)
				}
			}
			steps := []StepResult{}
			for _, label := range domain.StepOrder() {
				steps = append(steps, StepResult{Label: label, OK: true})
			}
			steps[2] = step
			summary := SummarizeSelfVerification(augment.SelfAugmentResult{OK: wantOK, Iterations: 1, Runs: []augment.SelfAugmentIteration{{Iteration: 1, Steps: steps}}}, 95)
			if summary.TerminationEligible != wantOK {
				t.Fatal(summary)
			}
			if !wantOK {
				longCalls := 0
				deps.ValidateHarnessInvariants = func(string) StepResult { return StepResult{Label: "harness invariants", OK: true} }
				deps.ValidateGoFormat = func(string) StepResult { return StepResult{Label: "gofmt", OK: true} }
				deps.ValidateRiskQATier = func(string) RiskQAEvidence {
					longCalls++
					t.Fatal("early failure ran risk QA")
					return RiskQAEvidence{}
				}
				result, err := ExecuteLoop(LoopRequest{BaseSeed: 100, TargetScore: 95}, LoopDeps{IssueOpsRoot: func() string { return root }, StepDeps: deps, Now: time.Now, MkdirTemp: func() (string, error) { return t.TempDir(), nil }, RemoveAll: os.RemoveAll, TempBinaryPath: func(string) string { return "unused" }, Summarize: SummarizeSelfVerification})
				if !errors.Is(err, ErrSelfVerificationGateFailed) || result.OK || result.TerminationEligible || result.Summary.TerminationEligible || len(result.Runs[0].Steps) != 3 || longCalls != 0 {
					t.Fatalf("early failure: %+v %v", result, err)
				}
			}
			t.Logf("%s: step_ok=%v termination=%v discovery_ran=%v duration_ms=%d", mode, step.OK, summary.TerminationEligible, ran, step.DurationMS)
		})
	}
}
