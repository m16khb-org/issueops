package selfverify

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
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
	if !errors.Is(err, ErrSelfVerificationGateFailed) || calls != 1 || result.OK || result.TerminationEligible || len(result.Runs[0].Steps) != 5 {
		t.Fatalf("scope failure result=%+v calls=%d err=%v", result, calls, err)
	}
}

func TestExecuteLoopReportsOneBasedStepProgressUntilFirstFailure(t *testing.T) {
	reporter := &recordingProgress{}
	deps := LoopDeps{
		IssueOpsRoot: func() string { return "/repo" }, Now: time.Now,
		MkdirTemp: func() (string, error) { return "/temp", nil }, RemoveAll: func(string) error { return nil }, TempBinaryPath: func(string) string { return "/temp/issueops" },
		StepDeps: SelfVerifyStepDeps{
			ValidateHarnessInvariants: func(string) selfverifycontract.StepResult { return selfverifycontract.StepResult{OK: true} },
			ValidateGoFormat:          func(string) selfverifycontract.StepResult { return selfverifycontract.StepResult{Error: "unformatted"} },
		},
		Summarize: func(selfaugmentcontract.SelfAugmentResult, float64) selfaugmentcontract.SelfAugmentSummary {
			return selfaugmentcontract.SelfAugmentSummary{FailedSteps: 1}
		},
	}
	_, err := ExecuteLoop(LoopRequest{Reporter: reporter, TargetScore: 95}, deps)
	if !errors.Is(err, ErrSelfVerificationGateFailed) {
		t.Fatalf("err=%v", err)
	}
	var starts []selfverifycontract.ProgressEvent
	for _, event := range reporter.events {
		if event.Event == "step_start" {
			starts = append(starts, event)
		}
	}
	if len(starts) != 2 || starts[0].StepIndex != 1 || starts[0].Step != "harness invariants" || starts[1].StepIndex != 2 || starts[1].Step != "gofmt" || starts[1].StepCount != len(selfverifydomain.StepOrder()) {
		t.Fatalf("step progress=%+v", starts)
	}
}

func TestExecuteLoopVerboseReportsElapsedSecondsOnSuccess(t *testing.T) {
	clock := 0
	deps := allPassingLoopDeps()
	deps.Now = func() time.Time {
		clock++
		if clock == 1 {
			return time.Unix(100, 0)
		}
		return time.Unix(100, 0).Add(2500 * time.Millisecond)
	}
	var printed []string
	deps.Printf = func(format string, args ...any) (int, error) {
		printed = append(printed, fmt.Sprintf(format, args...))
		return 0, nil
	}
	deps.PrintStep = func(selfverifycontract.StepResult) {}
	result, err := ExecuteLoop(LoopRequest{Verbose: true, TargetScore: 95}, deps)
	if err != nil || !result.OK || result.ElapsedMS != 2500 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if got := strings.Join(printed, ""); !strings.Contains(got, "Self-verification pipeline passed one evidence pass in 2.5s.") {
		t.Fatalf("verbose output=%q", got)
	}
}

// allPassingLoopDeps fills every step dependency with a passing fake, so a whole evidence pass succeeds.
func allPassingLoopDeps() LoopDeps {
	ok := selfverifycontract.StepResult{OK: true}
	var step SelfVerifyStepDeps
	stepValue := reflect.ValueOf(&step).Elem()
	for _, field := range stepValue.Fields() {
		if field.Kind() != reflect.Func {
			continue
		}
		outType := field.Type().Out(0)
		field.Set(reflect.MakeFunc(field.Type(), func([]reflect.Value) []reflect.Value {
			switch outType {
			case reflect.TypeFor[string]():
				return []reflect.Value{reflect.ValueOf("/repo")}
			case reflect.TypeFor[RiskQAEvidence]():
				return []reflect.Value{reflect.ValueOf(RiskQAEvidence{Step: ok, CoversFullGoTest: true})}
			}
			return []reflect.Value{reflect.ValueOf(ok)}
		}))
	}
	step.RunCommandStep = func(_, label string, _ time.Duration, _, _ string, _ ...string) selfverifycontract.StepResult {
		result := ok
		if label == "binary drift" {
			result.Stdout = `{"checks":[{"name":"binary_drift","healthy":true}]}`
		}
		return result
	}
	return LoopDeps{
		IssueOpsRoot: func() string { return "/repo" }, Now: time.Now,
		MkdirTemp: func() (string, error) { return "/temp", nil }, RemoveAll: func(string) error { return nil }, TempBinaryPath: func(string) string { return "/temp/issueops" },
		StepDeps: step,
		Summarize: func(selfaugmentcontract.SelfAugmentResult, float64) selfaugmentcontract.SelfAugmentSummary {
			return selfaugmentcontract.SelfAugmentSummary{TerminationEligible: true}
		},
	}
}
