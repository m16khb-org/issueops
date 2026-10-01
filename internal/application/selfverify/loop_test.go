package selfverify

import (
	"errors"
	"strings"
	"testing"
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

func TestLoopContractDescribesSinglePassAndOptionalLLMEvaluation(t *testing.T) {
	result := NewLoopResult(1, 100, 95, "/repo")
	body := strings.Join(result.LoopContract, "\n")
	for _, want := range []string{"one deterministic evidence pass", "opt-in", "read-only evaluator prompt", "collect-all-steps"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing current loop contract %q", want)
		}
	}
	for _, retired := range []string{"quick mode", "full mode", "10 seeded iterations", "final LLM gate"} {
		if strings.Contains(body, retired) {
			t.Errorf("retired loop contract %q", retired)
		}
	}
}

type recordingProgress struct {
	events []selfverifycontract.ProgressEvent
}

func (*recordingProgress) SetStarted(time.Time) {}
func (r *recordingProgress) Emit(event selfverifycontract.ProgressEvent) {
	r.events = append(r.events, event)
}
func (*recordingProgress) EmitStepEnd(string, int, int, int64, int, int, selfverifycontract.StepResult) {
}

func TestExecuteLoopTempWorkspaceFailurePreservesFailureResult(t *testing.T) {
	wantErr := errors.New("temp unavailable")
	reporter := &recordingProgress{}
	result, err := ExecuteLoop(LoopRequest{BaseSeed: 37, TargetScore: 95, Reporter: reporter}, LoopDeps{
		IssueOpsRoot: func() string { return "/repo" },
		Now:          time.Now,
		MkdirTemp:    func() (string, error) { return "", wantErr },
		FailedStep: func(label string, err error) selfverifycontract.StepResult {
			return selfverifycontract.StepResult{Label: label, Error: err.Error()}
		},
		Summarize: func(result selfaugmentcontract.SelfAugmentResult, target float64) selfaugmentcontract.SelfAugmentSummary {
			if len(result.Runs) != 1 || len(result.Runs[0].Steps) != 1 || target != 95 {
				t.Fatalf("summary input=%+v target=%v", result, target)
			}
			return selfaugmentcontract.SelfAugmentSummary{FailedSteps: 1}
		},
	})
	if !errors.Is(err, wantErr) || result.OK || result.Summary.FailedSteps != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(reporter.events) != 2 || reporter.events[0].Event != "loop_start" || reporter.events[1].Event != "loop_end" || reporter.events[1].OK == nil || *reporter.events[1].OK {
		t.Fatalf("progress=%+v", reporter.events)
	}
}

func TestExecuteLoopForwardsScopeAndKeepsScopeFailureNonPassing(t *testing.T) {
	calls := 0
	deps := LoopDeps{
		IssueOpsRoot: func() string { return "/repo" }, Now: time.Now,
		MkdirTemp: func() (string, error) { return "/temp", nil }, RemoveAll: func(string) error { return nil }, TempBinaryPath: func(string) string { return "/temp/issueops" },
		StepDeps: SelfVerifyStepDeps{
			ValidateHarnessInvariants: func(string) selfverifycontract.StepResult { return selfverifycontract.StepResult{OK: true} },
			ValidateGoFormat:          func(string) selfverifycontract.StepResult { return selfverifycontract.StepResult{OK: true} },
			RunCommandStep: func(string, string, time.Duration, string, string, ...string) selfverifycontract.StepResult {
				return selfverifycontract.StepResult{OK: true}
			},
			ValidateRiskQATier: func(string) RiskQAEvidence { t.Fatal("unscoped dependency used"); return RiskQAEvidence{} },
			ValidateRiskQATierWithScope: func(root, ref string) RiskQAEvidence {
				calls++
				if root != "/repo" || ref != "sealed-base" {
					t.Fatalf("scope=(%q,%q)", root, ref)
				}
				return RiskQAEvidence{Step: selfverifycontract.StepResult{Label: "risk QA tier", Error: "base unavailable"}, CoversFullGoTest: false}
			},
		},
		Summarize: func(selfaugmentcontract.SelfAugmentResult, float64) selfaugmentcontract.SelfAugmentSummary {
			return selfaugmentcontract.SelfAugmentSummary{FailedSteps: 1}
		},
	}
	result, err := ExecuteLoop(LoopRequest{BaseRef: "sealed-base", TargetScore: 95}, deps)
	if !errors.Is(err, ErrSelfVerificationGateFailed) || calls != 1 || result.OK || result.TerminationEligible || len(result.Runs[0].Steps) != 4 {
		t.Fatalf("scope failure result=%+v calls=%d err=%v", result, calls, err)
	}
}
