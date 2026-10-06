package statuscli

import (
	"context"
	"encoding/json"
	selfverifyapp "issueops/internal/application/selfverify"
	selfcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	statuscontract "issueops/internal/contract/status"
	verifyworkcontract "issueops/internal/contract/verifywork"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/testsupport"
)

func TestBuildHarnessStatusReportsStateWorkerAndSelfVerify(t *testing.T) {
	repo := t.TempDir()
	stateDir := t.TempDir()
	workerDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", workerDir)
	if _, err := statestore.NewService().Write(context.Background(), "self-verify-latest", `{"schema_version":1,"kind":"self_verification_summary","generated_at":"2026-09-30T00:00:00Z","ok":true}`); err != nil {
		t.Fatalf("write self verify state: %v", err)
	}
	if _, err := testWorkerService().Enqueue(context.Background(), "smoke", "payload"); err != nil {
		t.Fatalf("enqueue worker job: %v", err)
	}

	status := BuildStatus(testDoctorService(), testWorkerService(), repo)

	if status.Kind != "harness_status" || status.Repo != repo {
		t.Fatalf("unexpected status identity: %#v", status)
	}
	if !status.State.OK || len(status.State.Records) != 1 {
		t.Fatalf("expected isolated state record, got %#v", status.State)
	}
	if !status.Workers.OK || len(status.Workers.Jobs) != 1 {
		t.Fatalf("expected isolated worker job, got %#v", status.Workers)
	}
	if !status.SelfVerify.Found || status.SelfVerify.LatestKey != "self-verify-latest" || status.SelfVerify.Bytes == 0 {
		t.Fatalf("expected self verify latest state, got %#v", status.SelfVerify)
	}
}

func TestRunStatusWritesTextAndJSON(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())

	text := captureStatusVerifyStdout(t, func() error {
		return RunStatus(testDoctorService(), testWorkerService(), []string{"--repo", repo})
	})
	if !strings.Contains(text, "issueops system-status:") || !strings.Contains(text, "doctor healthy:") {
		t.Fatalf("unexpected status text output:\n%s", text)
	}

	jsonText := captureStatusVerifyStdout(t, func() error {
		return RunStatus(testDoctorService(), testWorkerService(), []string{"--repo", repo, "--json"})
	})
	var decoded statuscontract.Result
	if err := json.Unmarshal([]byte(jsonText), &decoded); err != nil {
		t.Fatalf("decode status JSON: %v\n%s", err, jsonText)
	}
	if decoded.Kind != "harness_status" || decoded.Repo != repo {
		t.Fatalf("unexpected status JSON payload: %#v", decoded)
	}
}

func TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/verifywork\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	result := BuildVerifyWork(repo, false, []string{"git", "status", "--short"})
	if !result.OK {
		t.Fatalf("expected verify-work result to be ok, warnings=%v", result.Warnings)
	}

	assertEvidenceItem(t, result.EvidenceMatrix, "git_preflight", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "guard_check", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "passed")

	if len(result.Evidence) == 0 {
		t.Fatalf("expected legacy evidence strings to remain populated")
	}
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "test", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "build", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "vet", "./..."})
}

func TestBuildVerifyWorkSerializesEmptySuggestedCommands(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := BuildVerifyWork(repo, false, nil)
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "skipped")
	if len(result.SuggestedCommands) != 0 {
		t.Fatalf("expected no suggested commands without project signals, got %#v", result.SuggestedCommands)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal verify-work result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal verify-work payload: %v", err)
	}
	commands, ok := decoded["suggested_commands"].([]any)
	if !ok {
		t.Fatalf("expected suggested_commands to serialize as an array, payload=%s", string(payload))
	}
	if len(commands) != 0 {
		t.Fatalf("expected empty suggested_commands array, got %#v", commands)
	}
}

func TestBuildVerifyWorkMarksDeniedCommandFailed(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := BuildVerifyWork(repo, false, []string{"sh", "-c", "true"})
	if result.OK {
		t.Fatalf("expected denied command to fail verify-work")
	}
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "failed")
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func assertEvidenceItem(t *testing.T, items []verifyworkcontract.EvidenceItem, name string, status string) {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			if item.Status != status {
				t.Fatalf("evidence item %s status=%s, want %s", name, item.Status, status)
			}
			if item.Summary == "" {
				t.Fatalf("evidence item %s has empty summary", name)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %s in %#v", name, items)
}

func assertSuggestedCommand(t *testing.T, commands []verifyworkcontract.SuggestedCommand, want []string) {
	t.Helper()
	for _, command := range commands {
		if equalStringSlices(command.Command, want) {
			if command.Name == "" || command.Reason == "" {
				t.Fatalf("suggested command %#v must include name and reason", command)
			}
			return
		}
	}
	t.Fatalf("missing suggested command %v in %#v", want, commands)
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func TestRunStatusSelectsProductionSummaryWithoutChangingState(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	for _, fixture := range []struct {
		key string
		at  time.Time
		ok  bool
	}{
		{"self-verify-baseline", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), true},
		{"custom-failed-run", time.Date(2026, 9, 30, 0, 0, 0, 123456789, time.UTC), false},
	} {
		result := selfcontract.SelfAugmentResult{OK: fixture.ok}
		err := selfverifyapp.SaveSummary(&result, fixture.key, selfverifyapp.SaveSummaryDeps{
			Now:    func() time.Time { return fixture.at },
			Encode: func(snapshot selfcontract.SelfAugmentStateSnapshot) ([]byte, error) { return json.Marshal(snapshot) },
			Write: func(key, content string) (statecontract.StateResult, error) {
				return statestore.NewService().Write(context.Background(), key, content)
			}, StateDir: statestore.StateDir,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := statestore.NewService().Write(context.Background(), "self-verify-candidates", `{"schema_version":1,"kind":"self_verification_candidate_export","generated_at":"2030-01-01T00:00:00Z"}`); err != nil {
		t.Fatal(err)
	}
	// Re-saving an old baseline must not make it newer than the failed execution.
	baseline, err := statestore.NewService().Read("self-verify-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := statestore.NewService().Write(context.Background(), "self-verify-baseline", baseline.Record.Content); err != nil {
		t.Fatal(err)
	}
	before, err := statestore.NewService().List()
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]statecontract.RecordEnvelope{}
	for _, record := range before.Records {
		r, err := statestore.NewService().Read(record.Key)
		if err != nil {
			t.Fatal(err)
		}
		contents[record.Key] = r.Record
	}
	out := captureStatusVerifyStdout(t, func() error {
		return RunStatus(testDoctorService(), testWorkerService(), []string{"--repo", repo, "--json"})
	})
	var got statuscontract.Result
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	expected := contents["custom-failed-run"]
	if !got.SelfVerify.Found || got.SelfVerify.LatestKey != expected.Key || got.SelfVerify.UpdatedAt != expected.UpdatedAt || got.SelfVerify.Bytes != expected.Bytes {
		t.Fatalf("selfverify: %+v", got.SelfVerify)
	}
	after, err := statestore.NewService().List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("status changed state metadata")
	}
	for _, record := range after.Records {
		r, err := statestore.NewService().Read(record.Key)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(contents[record.Key], r.Record) {
			t.Fatalf("status changed %s", record.Key)
		}
	}
}
