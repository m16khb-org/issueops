package stateroundtrip

import (
	"context"
	selfverify "issueops/internal/contract/selfverify"

	"strings"
	"testing"
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
)

func TestStateRoundtripDetectsDryRunWriteInProbeState(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	deps := testStateDependencies(stateRoundtripTestDeps(t, 456))
	deps.stateRead = testValidator().StateRead
	run := deps.run
	deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) selfverify.StepResult {
		if label == "self verify promote dry-run" {
			var probeRoot string
			for _, value := range env {
				if strings.HasPrefix(value, "ISSUEOPS_STATE_DIR=") {
					probeRoot = strings.TrimPrefix(value, "ISSUEOPS_STATE_DIR=")
				}
			}
			if probeRoot == "" {
				t.Fatal("missing isolated probe state")
			}
			_, err := statestore.WriteStateRecord(context.Background(), probeRoot, "self-verify-456-promoted-baseline", statecontract.RecordEnvelope{SchemaVersion: 1, Key: "self-verify-456-promoted-baseline", Content: "unexpected write", UpdatedAt: "2026-09-30T00:00:00Z", Bytes: 16})
			if err != nil {
				t.Fatal(err)
			}
		}
		return run(root, label, timeout, input, env, command...)
	}
	step := validateStateRoundtripWithDeps("fake-binary", t.TempDir(), 456, deps)
	if step.OK || !strings.Contains(step.Error, "dry-run wrote baseline unexpectedly") {
		t.Fatalf("dry-run write escaped verification: ok=%v error=%q", step.OK, step.Error)
	}
	if strings.Contains(step.Command, "--allow-failed-source --confirm") {
		t.Fatal("confirmed promotion ran after dry-run mutation")
	}
}

func TestStateRoundtripIgnoresUnrelatedParentState(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", parent)
	_, err := statestore.WriteStateRecord(context.Background(), parent, "self-verify-456-promoted-baseline", statecontract.RecordEnvelope{SchemaVersion: 1, Key: "self-verify-456-promoted-baseline", Content: "parent baseline", UpdatedAt: "2026-09-30T00:00:00Z", Bytes: 15})
	if err != nil {
		t.Fatal(err)
	}
	deps := testStateDependencies(stateRoundtripTestDeps(t, 456))
	deps.stateRead = testValidator().StateRead
	step := validateStateRoundtripWithDeps("fake-binary", t.TempDir(), 456, deps)
	if !step.OK {
		t.Fatalf("parent state contaminated probe: %s", step.Error)
	}
}
