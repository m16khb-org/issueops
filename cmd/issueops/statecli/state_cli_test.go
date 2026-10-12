package statecli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/testsupport"
)

func TestRunStateRoutesUsageAndTextRoundtrip(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	stderr := captureStateCLIStderr(t, func() error {
		return Run(testDependencies(), nil)
	})
	if !strings.Contains(stderr, "issueops state write") {
		t.Fatalf("state usage missing write command:\n%s", stderr)
	}
	stderr = captureStateCLIStderr(t, func() error {
		return Run(testDependencies(), []string{"unknown"})
	})
	if strings.Contains(stderr, "issueops state migrate") {
		t.Fatalf("state usage still advertises retired migrate command:\n%s", stderr)
	}
	if err := Run(testDependencies(), []string{"migrate"}); err == nil || !strings.Contains(err.Error(), "unknown state subcommand") {
		t.Fatalf("retired migrate command error=%v", err)
	}

	input := filepath.Join(t.TempDir(), "state.txt")
	if err := os.WriteFile(input, []byte("state from file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeOut := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--key", "text-key", "--input", input})
	})
	if !strings.Contains(writeOut, `state "text-key" written`) || !strings.Contains(writeOut, "16 bytes") {
		t.Fatalf("unexpected state write text output:\n%s", writeOut)
	}
	readOut := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"read", "text-key"})
	})
	if readOut != "state from file\n" {
		t.Fatalf("unexpected state read text output %q", readOut)
	}
	listOut := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"list"})
	})
	if !strings.Contains(listOut, "text-key\n") {
		t.Fatalf("unexpected state list text output:\n%s", listOut)
	}
}

func TestRunStateWriteReadAndPruneErrorsStaySurfaced(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	for _, args := range [][]string{
		{"write", "--key", "missing-source"},
		{"write", "--key", "too-many", "--value", "one", "--input", filepath.Join(t.TempDir(), "missing.txt")},
	} {
		if err := Run(testDependencies(), args); err == nil || !strings.Contains(err.Error(), "provide exactly one content source") {
			t.Fatalf("runState(testDependencies(), %v) error=%v, want content source error", args, err)
		}
	}
	if err := Run(testDependencies(), []string{"write", "--key", "missing-input", "--input", filepath.Join(t.TempDir(), "missing.txt")}); err == nil {
		t.Fatal("expected missing input file error")
	}
	if err := Run(testDependencies(), []string{"read", "--key", "missing"}); err == nil {
		t.Fatal("expected missing read key error")
	}
	if err := Run(testDependencies(), []string{"prune", "--max-age", "0s"}); err == nil {
		t.Fatal("expected invalid prune max-age error")
	}
}

func TestRunStatePruneAndDoctorTextBranches(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	if _, err := statestore.NewService().Write(context.Background(), "old", "old content"); err != nil {
		t.Fatalf("write old state: %v", err)
	}
	old, err := statestore.NewService().Read("old")
	if err != nil {
		t.Fatalf("read old state: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	writeStateCLIRecord(t, stateDir, "old", old.Record)

	dryPrune := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"prune", "--max-age", "1h"})
	})
	if !strings.Contains(dryPrune, "would prune 1 state records") || !strings.Contains(dryPrune, "old\n") {
		t.Fatalf("unexpected dry-run prune text:\n%s", dryPrune)
	}
	confirmedPrune := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"prune", "--max-age", "1h", "--confirm"})
	})
	if !strings.Contains(confirmedPrune, "pruned 1 state records") || !strings.Contains(confirmedPrune, "old\n") {
		t.Fatalf("unexpected confirmed prune text:\n%s", confirmedPrune)
	}

	writeRawStateCLIRow(t, stateDir, "corrupt", "{bad json\n")
	doctorOut := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"doctor"})
	})
	if !strings.Contains(doctorOut, "state doctor found 1 issues") || !strings.Contains(doctorOut, "error invalid_state") {
		t.Fatalf("unexpected doctor text:\n%s", doctorOut)
	}
	doctor, err := statestore.Doctor(statestore.StateDir())
	if err != nil {
		t.Fatalf("state doctor: %v", err)
	}
	if !stateDoctorHasIssueCode(doctor.Issues, "invalid_state") || stateDoctorHasIssueCode(doctor.Issues, "missing") {
		t.Fatalf("stateDoctorHasIssueCode mismatch for issues: %#v", doctor.Issues)
	}

}

func writeStateCLIRecord(t *testing.T, dir, key string, record statecontract.RecordEnvelope) {
	t.Helper()
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeRawStateCLIRow(t, dir, key, string(b)+"\n")
}

func writeRawStateCLIRow(t *testing.T, dir, key, raw string) {
	t.Helper()
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("state", key, []byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func captureStateCLIStderr(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := testsupport.CaptureStderrAndError(t, fn)
	if err == nil {
		t.Fatal("expected captured call to fail")
	}
	return out
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func stateDoctorHasIssueCode(issues []statecontract.StateDoctorIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}

func TestRunStateWriteContentSourceAndKeyResolution(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	out := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--key", "flag-key", "--value", "abc"})
	})
	if !strings.Contains(out, `state "flag-key" written (3 bytes)`) {
		t.Fatalf("--value write output:\n%s", out)
	}
	out = captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--value", "abcd", "positional-key"})
	})
	if !strings.Contains(out, `state "positional-key" written (4 bytes)`) {
		t.Fatalf("positional key write output:\n%s", out)
	}
	out = captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--key", "flag-key", "--value=xy", "ignored-positional"})
	})
	if !strings.Contains(out, `state "flag-key" written (2 bytes)`) {
		t.Fatalf("explicit --key must win over positional:\n%s", out)
	}
	if err := Run(testDependencies(), []string{"write", "--key", "k", "--value", "v", "--stdin"}); err == nil || !strings.Contains(err.Error(), "exactly one content source") {
		t.Fatalf("multiple content sources must be rejected: %v", err)
	}
}

func TestRunStateWriteAcceptsEverySyntaxTheFlagPackageAcceptsForValue(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, args := range [][]string{
		{"write", "--key", "k", "-value=xy"},
		{"write", "--key", "k", "--value=xy"},
		{"write", "--key", "k", "-value", "xy"},
	} {
		out := captureStatusVerifyStdout(t, func() error { return Run(testDependencies(), args) })
		if !strings.Contains(out, `state "k" written (2 bytes)`) {
			t.Fatalf("args %v output:\n%s", args, out)
		}
	}
	out := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--key", "empty", "-value="})
	})
	if !strings.Contains(out, `state "empty" written (0 bytes)`) {
		t.Fatalf("an explicitly empty -value= is still a content source:\n%s", out)
	}
}

func TestRunStateWriteReadsContentFromStdinAsTheOnlySource(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("from-stdin"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = original
		reader.Close()
	})

	out := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"write", "--key", "piped", "--stdin"})
	})
	if !strings.Contains(out, `state "piped" written (10 bytes)`) {
		t.Fatalf("--stdin write output:\n%s", out)
	}
}

func TestRunStateReadExplicitKeyWinsOverPositional(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for key, value := range map[string]string{"first": "one", "second": "two"} {
		if err := Run(testDependencies(), []string{"write", "--key", key, "--value", value}); err != nil {
			t.Fatal(err)
		}
	}
	out := captureStatusVerifyStdout(t, func() error {
		return Run(testDependencies(), []string{"read", "--key", "first", "second"})
	})
	if out != "one" {
		t.Fatalf("read output %q, want explicit --key content", out)
	}
}
