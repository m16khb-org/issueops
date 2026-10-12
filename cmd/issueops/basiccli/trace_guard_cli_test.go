package basiccli

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	auditcontract "issueops/internal/contract/audit"
	guardcontract "issueops/internal/contract/guard"
	issueopscontract "issueops/internal/contract/issueops"
	trace "issueops/internal/contract/trace"
)

func TestRunTraceRoutesUsageUnknownAndAnalyzeJSON(t *testing.T) {
	if err := RunTrace(nil); err == nil || !strings.Contains(err.Error(), "missing trace subcommand") {
		t.Fatalf("expected missing trace subcommand error, got %v", err)
	}
	if err := RunTrace([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown trace subcommand "unknown"`) {
		t.Fatalf("expected unknown trace subcommand error, got %v", err)
	}

	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"deterministic","failed_step":"go test","rerun_commands":["go test ./..."]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return RunTrace([]string{"analyze", "--input", input, "--json"})
	})
	var result trace.TraceAnalyzeResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode trace analyze JSON: %v\n%s", err, out)
	}
	if !result.OK || result.FindingCount != 1 || result.InputSource != "file" {
		t.Fatalf("unexpected trace analyze result: %#v", result)
	}
}

func TestRunTraceAnalyzeWritesTextSummary(t *testing.T) {
	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"intermittent","failed_step":"guard","rerun_commands":["issueops guard check"]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return RunTraceAnalyze([]string{input})
	})
	if !strings.Contains(out, "trace analysis: 1 finding") || !strings.Contains(out, "verify: issueops guard check") {
		t.Fatalf("unexpected trace text output:\n%s", out)
	}
}

func TestRunTraceHandoffDeliveryObserveUsesInjectedAuditProducer(t *testing.T) {
	input := filepath.Join(t.TempDir(), "delivery.json")
	writeFileForCLITest(t, input, `{"schema_version":1,"attempt_id":"manual-attempt"}`)
	command := testBasicCommand()
	command.Handoff = handoffObserverFunc(func(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
		if observation.AttemptID != "manual-attempt" {
			t.Fatalf("observation=%+v", observation)
		}
		observation.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{Location: "audit/handoff-delivery.jsonl#audit_log_id=audit-1", Digest: strings.Repeat("a", 64)}
		return auditcontract.HandoffDeliveryAuditRecord{OK: true, Kind: "handoff_delivery_observation", AuditLogID: "audit-1", Observation: observation}, nil
	})
	out := captureStatusVerifyStdout(t, func() error {
		return command.RunTrace([]string{"handoff-delivery", "--input", input, "--json"})
	})
	var result trace.HandoffDeliveryObserveResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.AuditLogID != "audit-1" || result.Observation.Receipt.Location == "" {
		t.Fatalf("result=%+v", result)
	}
}

func TestRunGuardRoutesAndChecksExplicitFiles(t *testing.T) {
	if err := RunGuard(nil); err == nil || !strings.Contains(err.Error(), "missing guard subcommand") {
		t.Fatalf("expected missing guard subcommand error, got %v", err)
	}
	if err := RunGuard([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown guard subcommand "unknown"`) {
		t.Fatalf("expected unknown guard subcommand error, got %v", err)
	}

	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "ok_test.go"), "package main\n\nfunc TestSpecificBehavior(t *testing.T) {}\n")
	out := captureStatusVerifyStdout(t, func() error {
		return RunGuard([]string{"check", "--repo", repo, "--json", "--", "ok_test.go"})
	})
	var result guardcontract.GuardCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode guard JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Mode != "files" || len(result.CheckedFiles) != 1 {
		t.Fatalf("unexpected guard result: %#v", result)
	}
}

func TestRunGuardCheckTextOutputReturnsBlockedError(t *testing.T) {
	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "slow_test.go"), "package main\n\nfunc TestSlow(t *testing.T) {\n\ttime.Sleep(1)\n}\n")

	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return RunGuardCheck([]string{"--repo", repo, "--", "slow_test.go"})
	})
	if _, ok := errors.AsType[guardcontract.GuardBlockedError](err); !ok {
		t.Fatalf("expected guard blocked error, got %T %v", err, err)
	}
	if !strings.Contains(out, "guard files: 1 file(s), block=1") || !strings.Contains(out, "sleep-in-test") {
		t.Fatalf("unexpected guard text output:\n%s", out)
	}
}

type handoffObserverFunc func(issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)

func (f handoffObserverFunc) ObserveManual(o issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	return f(o)
}

func writeHandoffObservationForCLITest(t *testing.T, name string, attemptID string, padTo int) string {
	t.Helper()
	body := `{"schema_version":1,"attempt_id":"` + attemptID + `"}`
	if padTo > len(body) {
		body += strings.Repeat(" ", padTo-len(body))
	}
	path := filepath.Join(t.TempDir(), name)
	writeFileForCLITest(t, path, body)
	return path
}

func recordingHandoffCommand(attempts *[]string) Command {
	command := testBasicCommand()
	command.Handoff = handoffObserverFunc(func(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
		*attempts = append(*attempts, observation.AttemptID)
		return auditcontract.HandoffDeliveryAuditRecord{OK: true, Kind: "handoff_delivery_observation", AuditLogID: "audit-1", Observation: observation}, nil
	})
	return command
}

func TestRunTraceHandoffDeliveryReadsPositionalInputWhenFlagMissing(t *testing.T) {
	var attempts []string
	command := recordingHandoffCommand(&attempts)
	path := writeHandoffObservationForCLITest(t, "positional.json", "positional-attempt", 0)
	captureStatusVerifyStdout(t, func() error {
		return command.RunTrace([]string{"handoff-delivery", path})
	})
	if len(attempts) != 1 || attempts[0] != "positional-attempt" {
		t.Fatalf("attempts=%v", attempts)
	}
}

func TestRunTraceHandoffDeliveryInputFlagWinsOverPositional(t *testing.T) {
	var attempts []string
	command := recordingHandoffCommand(&attempts)
	flagPath := writeHandoffObservationForCLITest(t, "flag.json", "flag-attempt", 0)
	positional := writeHandoffObservationForCLITest(t, "positional.json", "positional-attempt", 0)
	captureStatusVerifyStdout(t, func() error {
		return command.RunTrace([]string{"handoff-delivery", "--input", flagPath, positional})
	})
	if len(attempts) != 1 || attempts[0] != "flag-attempt" {
		t.Fatalf("attempts=%v", attempts)
	}
}

func TestRunTraceHandoffDeliveryRejectsClearlyOversizedInput(t *testing.T) {
	var attempts []string
	command := recordingHandoffCommand(&attempts)
	oversized := writeHandoffObservationForCLITest(t, "oversized.json", "oversized-attempt", 1024*1024)
	err := command.RunTrace([]string{"handoff-delivery", "--input", oversized})
	if err == nil || !strings.Contains(err.Error(), "empty or oversized") {
		t.Fatalf("expected oversized error, got %v", err)
	}
	if len(attempts) != 0 {
		t.Fatalf("oversized input must not reach the observer; attempts=%v", attempts)
	}
}

type recordingGuardSource struct {
	requests []guardcontract.GuardCheckRequest
}

func (*recordingGuardSource) ResolveRoot(root string) string { return root }
func (source *recordingGuardSource) TargetFiles(_ string, request guardcontract.GuardCheckRequest) []string {
	source.requests = append(source.requests, request)
	return nil
}
func (*recordingGuardSource) ExistingSymbols(string, []string) map[string][]string { return nil }
func (*recordingGuardSource) ReadFile(string, string, bool) (string, bool)         { return "", false }

func TestRunGuardCheckStagedDefaultAndOverrides(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantStaged bool
		wantAll    bool
		wantFiles  int
	}{
		{name: "no files defaults to staged", args: []string{"--repo", "r"}, wantStaged: true},
		{name: "explicit files disable staged", args: []string{"--repo", "r", "--", "a.go"}, wantFiles: 1},
		{name: "all disables staged", args: []string{"--repo", "r", "--all"}, wantAll: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := &recordingGuardSource{}
			command := testBasicCommand()
			command.Guard.Source = source
			captureStatusVerifyStdout(t, func() error { return command.RunGuard(append([]string{"check"}, tc.args...)) })
			if len(source.requests) != 1 {
				t.Fatalf("requests=%v", source.requests)
			}
			got := source.requests[0]
			if got.Staged != tc.wantStaged || got.All != tc.wantAll || len(got.Files) != tc.wantFiles {
				t.Fatalf("request=%+v want staged=%v all=%v files=%d", got, tc.wantStaged, tc.wantAll, tc.wantFiles)
			}
		})
	}
}

func TestRunGuardCheckJSONStillReturnsBlockedError(t *testing.T) {
	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "slow_test.go"), "package main\n\nfunc TestSlow(t *testing.T) {\n\ttime.Sleep(1)\n}\n")
	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return RunGuardCheck([]string{"--repo", repo, "--json", "--", "slow_test.go"})
	})
	if _, ok := errors.AsType[guardcontract.GuardBlockedError](err); !ok {
		t.Fatalf("expected guard blocked error, got %T %v", err, err)
	}
	var result guardcontract.GuardCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode guard JSON: %v\n%s", err, out)
	}
	if result.OK || result.Summary.Block != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

type findingGuardSource struct {
	recordingGuardSource
	files []string
}

func (source *findingGuardSource) TargetFiles(string, guardcontract.GuardCheckRequest) []string {
	return source.files
}
func (*findingGuardSource) ReadFile(_, rel string, _ bool) (string, bool) {
	if rel == "slow_test.go" {
		return "package main\n\nfunc TestSlow(t *testing.T) {\n\ttime.Sleep(1)\n}\n", true
	}
	return "package main\n", true
}

func TestRunGuardCheckTextFindingLocation(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  []string
	}{
		{"line number appended", []string{".env", "slow_test.go"}, []string{
			"block sleep-in-test slow_test.go:4: ",
			"block secret-like-path .env: ",
		}},
		{"file-less finding renders dash", []string{"prod.go"}, []string{"warn prod-change-without-test -: "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := testBasicCommand()
			command.Guard.Source = &findingGuardSource{files: tc.files}
			out, _ := captureTraceGuardPolicyStdout(t, func() error {
				return command.RunGuard([]string{"check", "--repo", "r"})
			})
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in output:\n%s", want, out)
				}
			}
		})
	}
}
