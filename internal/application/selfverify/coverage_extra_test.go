package selfverify

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	statecontract "issueops/internal/contract/state"
	selfverifydomain "issueops/internal/domain/selfverify"
)

func TestApplyLLMEvalRendersPromptAndNeverPassesGateMode(t *testing.T) {
	input := augment.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: augment.SelfAugmentSummary{TerminationEligible: true}}
	deps := LLMEvalDeps{
		BuildPrompt: func(augment.SelfAugmentResult) (string, int, error) { return "evaluate this", 13, nil },
		BoundError:  func(string, error, string) string { t.Fatal("no error expected"); return "" },
	}

	advisory, err := ApplyLLMEval(input, verify.LLMEvalOptions{Enabled: true, Mode: " ADVISORY ", TargetScore: 95}, deps)
	if err != nil || !advisory.OK || !advisory.TerminationEligible {
		t.Fatalf("advisory mode must not block: ok=%v eligible=%v err=%v", advisory.OK, advisory.TerminationEligible, err)
	}
	eval := advisory.LLMEval
	if eval == nil || eval.Mode != "advisory" || eval.Prompt != "evaluate this" || eval.EvidencePacketBytes != 13 || !eval.ReadOnly || eval.ExecutionClass != "foreground_blocking" || !strings.Contains(eval.Error, "was removed") {
		t.Fatalf("eval = %+v", eval)
	}

	// Blank mode defaults to advisory; gate mode has no recorded verdict, so it can never pass.
	defaulted, err := ApplyLLMEval(input, verify.LLMEvalOptions{Enabled: true, TargetScore: 95}, deps)
	if err != nil || defaulted.LLMEval.Mode != "advisory" {
		t.Fatalf("default mode = %+v err=%v", defaulted.LLMEval, err)
	}
	gated, err := ApplyLLMEval(input, verify.LLMEvalOptions{Enabled: true, Mode: "gate", TargetScore: 95}, deps)
	if err == nil || !strings.Contains(err.Error(), "LLM evaluation gate failed") || gated.OK || gated.TerminationEligible || gated.Summary.TerminationEligible {
		t.Fatalf("gate mode must block termination: %+v err=%v", gated, err)
	}
	if gated.LLMEval == nil || gated.LLMEval.Prompt != "evaluate this" {
		t.Fatalf("gate run must still render the prompt: %+v", gated.LLMEval)
	}
}

func TestSaveSummaryRecordsCheckpointFromStateWrite(t *testing.T) {
	now := time.Date(2026, 10, 12, 3, 4, 5, 0, time.UTC)
	var wroteKey, wroteBody string
	result := augment.SelfAugmentResult{OK: true, LoopKind: "self_verification", BaseSeed: 7}
	err := SaveSummary(&result, "", SaveSummaryDeps{
		Now: func() time.Time { return now },
		Encode: func(snapshot augment.SelfAugmentStateSnapshot) ([]byte, error) {
			if snapshot.GeneratedAt != now.Format(time.RFC3339Nano) || snapshot.BaseSeed != 7 || snapshot.LoopKind != "self_verification" {
				t.Fatalf("snapshot = %+v", snapshot)
			}
			return []byte("encoded"), nil
		},
		Write: func(key, body string) (statecontract.StateResult, error) {
			wroteKey, wroteBody = key, body
			return statecontract.StateResult{StateDir: "/state", Path: "/state/k.json", Record: statecontract.RecordEnvelope{Key: "canonical-key", Bytes: 7}}, nil
		},
		StateDir: func() string { t.Fatal("StateDir is only for failures"); return "" },
	})
	if err != nil || wroteKey != "self-verify-latest" || wroteBody != "encoded" {
		t.Fatalf("err=%v key=%q body=%q", err, wroteKey, wroteBody)
	}
	want := &augment.SelfAugmentStateCheckpoint{OK: true, Key: "canonical-key", StateDir: "/state", Path: "/state/k.json", Bytes: 7}
	if !reflect.DeepEqual(result.StateCheckpoint, want) {
		t.Fatalf("checkpoint = %+v, want %+v", result.StateCheckpoint, want)
	}
}

func TestValidateFormatReportsGofmtToolFailure(t *testing.T) {
	step := ValidateFormat("/repo", FormatDeps{
		Now:                time.Now,
		ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return []string{"a.go"}, nil },
		ListUnformatted:    func(context.Context, string, []string) ([]string, error) { return nil, errors.New("gofmt crashed") },
	})
	if step.OK || step.Command != FormatCommand || !strings.Contains(step.Error, "gofmt -l: gofmt crashed") {
		t.Fatalf("step = %+v", step)
	}
}

func TestBinaryDriftReportsUnhealthySummary(t *testing.T) {
	for _, test := range []struct{ name, body, want string }{
		{"with summary", `{"checks":[{"name":"binary_drift","healthy":false,"summary":"bin/issueops is stale"}]}`, "binary drift: bin/issueops is stale"},
		{"without summary", `{"checks":[{"name":"binary_drift","healthy":false}]}`, "binary drift: unhealthy"},
		{"missing boolean", `{"checks":[{"name":"binary_drift"}]}`, "missing healthy boolean"},
	} {
		got := binaryDriftEvidence(verify.StepResult{OK: true, Stdout: test.body})
		if got.OK || got.Error != test.want && !strings.Contains(got.Error, test.want) {
			t.Errorf("%s: %+v", test.name, got)
		}
	}
	if got := binaryDriftEvidence(verify.StepResult{OK: true, Stdout: `{"checks":[{"name":"binary_drift","healthy":true}]}`, StdoutTruncated: true}); got.OK || !strings.Contains(got.Error, "truncated") {
		t.Errorf("truncated doctor output must fail: %+v", got)
	}
}

func TestRequireGoldenEvidenceNamesTheMissingProofAndRejectsFailures(t *testing.T) {
	events := func(tests map[string][]string, packagePass ...string) string {
		var out strings.Builder
		for test, actions := range tests {
			i := strings.LastIndex(test, "/")
			pkg, name := test[:i], test[i+1:]
			for _, action := range actions {
				fmt.Fprintf(&out, `{"Action":%q,"Package":%q,"Test":%q}`+"\n", action, pkg, name)
			}
		}
		for _, pkg := range packagePass {
			fmt.Fprintf(&out, `{"Action":"pass","Package":%q}`+"\n", pkg)
		}
		return out.String()
	}
	const golden, app = "issueops/cmd/issueops/contractgolden", "issueops/cmd/issueops/issueopsapp"
	all := map[string][]string{
		golden + "/TestCLIUsageGolden": {"run", "pass"}, golden + "/TestMCPToolsGolden": {"run", "pass"},
		golden + "/TestMCPResourcesGolden": {"run", "pass"}, app + "/TestResponseContractsGolden": {"run", "pass"},
	}
	if err := requireGoldenEvidence(verify.StepResult{Stdout: events(all, golden, app)}); err != nil {
		t.Fatal(err)
	}

	if err := requireGoldenEvidence(verify.StepResult{Stdout: events(all, golden)}); err == nil || err.Error() != "package did not pass: "+app {
		t.Fatalf("missing package pass: %v", err)
	}
	failedPackage := events(all, golden) + `{"Action":"fail","Package":"` + app + `"}` + "\n"
	if err := requireGoldenEvidence(verify.StepResult{Stdout: failedPackage}); err == nil || !strings.HasPrefix(err.Error(), "fail: "+app) {
		t.Fatalf("failed package: %v", err)
	}
	skipped := events(all, golden, app) + `{"Action":"skip","Package":"` + golden + `","Test":"TestCLIUsageGolden"}` + "\n"
	if err := requireGoldenEvidence(verify.StepResult{Stdout: skipped}); err == nil || !strings.HasPrefix(err.Error(), "skip: ") {
		t.Fatalf("skipped test: %v", err)
	}
	passWithoutRun := map[string][]string{}
	maps.Copy(passWithoutRun, all)
	passWithoutRun[app+"/TestResponseContractsGolden"] = []string{"pass"}
	if err := requireGoldenEvidence(verify.StepResult{Stdout: events(passWithoutRun, golden, app)}); err == nil || err.Error() != "required test did not run and pass: "+app+"/TestResponseContractsGolden" {
		t.Fatalf("pass without run: %v", err)
	}
	if err := requireGoldenEvidence(verify.StepResult{Stdout: "{", StdoutTruncated: false}); err == nil || !strings.Contains(err.Error(), "invalid go test JSON") {
		t.Fatalf("malformed JSON: %v", err)
	}
}

func stepLabels(result augment.SelfAugmentResult) []string {
	var labels []string
	for _, step := range result.Runs[0].Steps {
		labels = append(labels, step.Label)
	}
	return labels
}

type loopStepLog struct {
	labels   []string
	printed  []string
	printf   []string
	removed  []string
	progress []verify.ProgressEvent
	ended    []int
}

func (l *loopStepLog) SetStarted(time.Time)            {}
func (l *loopStepLog) Emit(event verify.ProgressEvent) { l.progress = append(l.progress, event) }
func (l *loopStepLog) EmitStepEnd(_ string, _, _ int, _ int64, index, _ int, _ verify.StepResult) {
	l.ended = append(l.ended, index)
}

// loopDeps wires every planned step to a recorder; failing maps a step label to a failure.
func loopDeps(log *loopStepLog, failing map[string]bool, summary augment.SelfAugmentSummary) LoopDeps {
	step := func(label string) verify.StepResult {
		log.labels = append(log.labels, label)
		return verify.StepResult{Label: label, OK: !failing[label], Error: map[bool]string{true: label + " broke"}[failing[label]]}
	}
	return LoopDeps{
		IssueOpsRoot: func() string { return "/repo" },
		Now:          func() time.Time { return time.Unix(0, 0) },
		MkdirTemp:    func() (string, error) { return "/tmp/sv", nil },
		RemoveAll:    func(path string) error { log.removed = append(log.removed, path); return nil },
		TempBinaryPath: func(dir string) string {
			return dir + "/issueops"
		},
		PrintStep: func(s verify.StepResult) { log.printed = append(log.printed, s.Label) },
		Printf: func(format string, args ...any) (int, error) {
			log.printf = append(log.printf, fmt.Sprintf(format, args...))
			return 0, nil
		},
		Summarize: func(augment.SelfAugmentResult, float64) augment.SelfAugmentSummary { return summary },
		StepDeps: SelfVerifyStepDeps{
			IssueOpsRoot: func() string { return "/repo" },
			RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, args ...string) verify.StepResult {
				// binary drift needs a doctor JSON that proves a fresh binary
				result := step(label)
				if label == "binary drift" {
					result.Stdout = `{"checks":[{"name":"binary_drift","healthy":true}]}`
				}
				return result
			},
			ValidateHarnessInvariants: func(string) verify.StepResult { return step("harness invariants") },
			ValidateGoFormat:          func(string) verify.StepResult { return step("gofmt") },
			ValidateRiskQATier: func(string) RiskQAEvidence {
				return RiskQAEvidence{Step: step("risk QA tier")}
			},
			ValidateInspect:               func(string, string) verify.StepResult { return step("inspect smoke") },
			ValidateDocsIndex:             func(string, string) verify.StepResult { return step("docs index smoke") },
			ValidateSelfVerifyCandidate:   func(string, string, int64) verify.StepResult { return step("candidate export") },
			ValidateStepBudgetBaseline:    func(string, string, int64) verify.StepResult { return step("step budget baseline") },
			ValidateInstallDryRunSmoke:    func(string, string, int64) verify.StepResult { return step("install dry-run smoke") },
			ValidateCommandPolicy:         func(string, string) verify.StepResult { return step("command policy smoke") },
			ValidateCommandAudit:          func(string, string, int64) verify.StepResult { return step("command audit smoke") },
			ValidateContractCheck:         func(string, string) verify.StepResult { return step("contract check") },
			ValidateToolConformance:       func(string, string) verify.StepResult { return step("tool contract conformance") },
			ValidateWorkerLifecycle:       func(string, string, int64) verify.StepResult { return step("worker lifecycle smoke") },
			ValidateMCP:                   func(string, string) verify.StepResult { return step("MCP smoke") },
			ValidateStateRoundtrip:        func(string, string, int64) verify.StepResult { return step("state roundtrip") },
			ValidateParallelTempIsolation: func(string, string, int64) verify.StepResult { return step("parallel isolation") },
			ValidatePreflightFuzz:         func(string, string, int64) verify.StepResult { return step("preflight fuzz") },
			ValidateWebFetchBattery:       func(string, string, int64) verify.StepResult { return step("web fetch battery") },
			ValidateNativeIntegration:     func(string) verify.StepResult { return step("native integration") },
			ValidateRedactionAudit:        func(string) verify.StepResult { return step("redaction audit") },
			ValidateQAGate:                func(string) verify.StepResult { return step("QA gate") },
		},
	}
}

func TestExecuteLoopPassingPassRunsEveryStepInPlanOrder(t *testing.T) {
	log := &loopStepLog{}
	result, err := ExecuteLoop(LoopRequest{BaseSeed: 5, TargetScore: 95, Verbose: true, Reporter: log},
		loopDeps(log, nil, augment.SelfAugmentSummary{TerminationEligible: true}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.TerminationEligible || len(result.Runs) != 1 || len(result.Runs[0].Steps) != len(selfverifydomain.StepOrder()) {
		t.Fatalf("result = ok:%v eligible:%v runs:%d", result.OK, result.TerminationEligible, len(result.Runs))
	}
	// the golden step is satisfied by the passing full suite, so it runs no command.
	if len(log.labels) != len(selfverifydomain.StepOrder())-1 || log.labels[0] != "harness invariants" || log.labels[len(log.labels)-1] != "QA gate" {
		t.Fatalf("steps ran as %v", log.labels)
	}
	if len(log.printed) != len(selfverifydomain.StepOrder()) || !slices.Equal(log.printed, selfverifydomain.StepOrder()) {
		t.Fatalf("verbose mode must print every step in order: %v", log.printed)
	}
	if len(log.printf) != 2 || !strings.Contains(log.printf[0], "seed=5") || !strings.Contains(log.printf[1], "passed one evidence pass") {
		t.Fatalf("printf = %q", log.printf)
	}
	if len(log.ended) != len(selfverifydomain.StepOrder()) || log.ended[0] != 1 || log.ended[len(log.ended)-1] != len(log.ended) {
		t.Fatalf("step end indexes = %v", log.ended)
	}
	if !reflect.DeepEqual(log.removed, []string{"/tmp/sv"}) {
		t.Fatalf("temp workspace must be removed: %v", log.removed)
	}
	events := log.progress
	if events[0].Event != "loop_start" || events[len(events)-1].Event != "loop_end" || events[len(events)-1].OK == nil || !*events[len(events)-1].OK {
		t.Fatalf("progress = %+v", events)
	}
}

func TestExecuteLoopIneligibleSummaryIsNotOKEvenWhenEveryStepPasses(t *testing.T) {
	log := &loopStepLog{}
	result, err := ExecuteLoop(LoopRequest{TargetScore: 95}, loopDeps(log, nil, augment.SelfAugmentSummary{TerminationEligible: false}))
	if err != nil || result.OK || result.TerminationEligible {
		t.Fatalf("scores below target must withhold termination: ok=%v eligible=%v err=%v", result.OK, result.TerminationEligible, err)
	}
}

func TestExecuteLoopFailFastStopsAtFirstFailureWhileCollectAllContinues(t *testing.T) {
	failing := map[string]bool{"gofmt": true, "go build": true}

	fast := &loopStepLog{}
	result, err := ExecuteLoop(LoopRequest{TargetScore: 95, Reporter: fast}, loopDeps(fast, failing, augment.SelfAugmentSummary{FailedSteps: 1}))
	if !errors.Is(err, ErrSelfVerificationGateFailed) || !strings.Contains(err.Error(), "gofmt failed: gofmt broke") || result.OK {
		t.Fatalf("fail fast: ok=%v err=%v", result.OK, err)
	}
	if !reflect.DeepEqual(fast.labels, []string{"harness invariants", "gofmt"}) {
		t.Fatalf("fail-fast must stop at the first failure: %v", fast.labels)
	}
	last := fast.progress[len(fast.progress)-1]
	if last.Event != "loop_end" || last.OK == nil || *last.OK {
		t.Fatalf("progress end = %+v", last)
	}
	if !reflect.DeepEqual(fast.removed, []string{"/tmp/sv"}) {
		t.Fatalf("temp workspace leaked on failure: %v", fast.removed)
	}

	all := &loopStepLog{}
	result, err = ExecuteLoop(LoopRequest{TargetScore: 95, CollectAllSteps: true}, loopDeps(all, failing, augment.SelfAugmentSummary{FailedSteps: 2}))
	if !errors.Is(err, ErrSelfVerificationGateFailed) || !strings.Contains(err.Error(), "gofmt failed") || strings.Contains(err.Error(), "go build failed") {
		t.Fatalf("collect-all must report the first failure: %v", err)
	}
	if len(result.Runs[0].Steps) != len(selfverifydomain.StepOrder()) || result.OK {
		t.Fatalf("collect-all must gather every step without granting success: ran=%d ok=%v", len(result.Runs[0].Steps), result.OK)
	}
}
