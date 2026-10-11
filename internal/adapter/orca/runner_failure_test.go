package orca

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

func TestExecRunnerRejectsEmptyArgvWithoutStartingAProcess(t *testing.T) {
	output, err := (ExecRunner{}).Run(context.Background(), "", time.Second, nil)
	orcaErr := requireOrcaCode(t, err, "invalid_argv")
	if orcaErr.Invoked || output.Invoked {
		t.Fatalf("empty argv reported invoked: %#v %#v", orcaErr, output)
	}
}

func TestExecRunnerReportsUnstartableBinaryAsNotInvoked(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-orca")
	output, err := (ExecRunner{}).Run(context.Background(), "", 5*time.Second, []string{missing, "status"})
	orcaErr := requireOrcaCode(t, err, "command_start_failed")
	if orcaErr.Invoked || output.Invoked {
		t.Fatalf("a binary that never started must not count as invoked: %#v %#v", orcaErr, output)
	}
}

func TestExecRunnerTimeoutIsMarkedAndInvoked(t *testing.T) {
	sleep, err := (ExecRunner{}).LookPath("sleep")
	if err != nil {
		t.Fatalf("sleep binary required for the timeout fixture: %v", err)
	}
	output, err := (ExecRunner{}).Run(context.Background(), "", 50*time.Millisecond, []string{sleep, "30"})
	orcaErr := requireOrcaCode(t, err, "command_timeout")
	if !orcaErr.Timeout || !orcaErr.Invoked || !output.Invoked {
		t.Fatalf("timeout = %#v output=%#v", orcaErr, output)
	}
}

func TestExecRunnerLookPathUsesPATH(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "orca-fixture")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binary, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := (ExecRunner{}).LookPath("orca-fixture")
	if err != nil || got != binary {
		t.Fatalf("LookPath = %q, %v", got, err)
	}
	if _, err := (ExecRunner{}).LookPath("definitely-not-installed"); err == nil {
		t.Fatal("a binary outside PATH was found")
	}
}

func TestBoundedStreamBufferKeepsOnlyTheLimitAndFlagsOverflow(t *testing.T) {
	buffer := boundedStreamBuffer{limit: 4}
	if n, err := buffer.Write([]byte("abc")); n != 3 || err != nil || buffer.exceeded {
		t.Fatalf("under-limit write n=%d err=%v exceeded=%t", n, err, buffer.exceeded)
	}
	// The caller must see a full write so the child never blocks on a short write.
	if n, err := buffer.Write([]byte("defg")); n != 4 || err != nil || !buffer.exceeded {
		t.Fatalf("straddling write n=%d err=%v exceeded=%t", n, err, buffer.exceeded)
	}
	if got := string(buffer.Bytes()); got != "abcd" {
		t.Fatalf("retained bytes = %q", got)
	}
	if n, err := buffer.Write([]byte("hij")); n != 3 || err != nil || string(buffer.Bytes()) != "abcd" {
		t.Fatalf("write after the limit n=%d err=%v retained=%q", n, err, buffer.Bytes())
	}
}

func TestBoundedStreamBufferDoesNotFlagAnEmptyWriteAtTheLimit(t *testing.T) {
	buffer := boundedStreamBuffer{limit: 2}
	if _, err := buffer.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Write(nil); err != nil || buffer.exceeded {
		t.Fatalf("an empty write at the exact limit flagged overflow: err=%v exceeded=%t", err, buffer.exceeded)
	}
}

func TestCommandFailureDiagnosticCombinesStreams(t *testing.T) {
	for _, tt := range []struct {
		name           string
		stdout, stderr string
		want           string
	}{
		{"both", "out", "err", "stdout: out; stderr: err"},
		{"stdout only", "out", " ", "stdout: out"},
		{"stderr only", "", "err", "err"},
		{"neither", "", "", ""},
	} {
		if got := commandFailureDiagnostic(CommandOutput{Stdout: []byte(tt.stdout), Stderr: []byte(tt.stderr)}); got != tt.want {
			t.Errorf("%s: diagnostic = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestBoundedDiagnosticTruncatesLongOutput(t *testing.T) {
	got := boundedDiagnostic(strings.Repeat("x", 5000))
	if want := strings.Repeat("x", 1024) + "..."; got != want {
		t.Fatalf("diagnostic length %d, want %d", len(got), len(want))
	}
	if short := boundedDiagnostic("  short  "); short != "short" {
		t.Fatalf("short diagnostic = %q", short)
	}
}

func TestShouldRetryWithoutInheritedRelayRequiresEveryCondition(t *testing.T) {
	full := []string{"ORCA_RELAY_DIR=/d", "ORCA_RELAY_SOCKET_PATH=/s", "ORCA_RELAY_CREDENTIAL_FILE=/c"}
	ownerless := CommandOutput{Stderr: []byte("error: No owning Orca client is connected to the relay")}
	failed := &port.OrcaError{Code: "command_failed"}
	for _, tt := range []struct {
		name    string
		environ []string
		output  CommandOutput
		err     error
		want    bool
	}{
		{"all conditions", full, ownerless, failed, true},
		{"relay variable missing", full[:2], ownerless, failed, false},
		{"relay variable empty", []string{"ORCA_RELAY_DIR=/d", "ORCA_RELAY_SOCKET_PATH=/s", "ORCA_RELAY_CREDENTIAL_FILE="}, ownerless, failed, false},
		{"different stderr", full, CommandOutput{Stderr: []byte("other")}, failed, false},
		{"different error code", full, ownerless, &port.OrcaError{Code: "command_timeout"}, false},
		{"no error", full, ownerless, nil, false},
		{"foreign error", full, ownerless, context.Canceled, false},
	} {
		if got := shouldRetryWithoutInheritedRelay(tt.environ, tt.output, tt.err); got != tt.want {
			t.Errorf("%s: retry = %t, want %t", tt.name, got, tt.want)
		}
	}
}
