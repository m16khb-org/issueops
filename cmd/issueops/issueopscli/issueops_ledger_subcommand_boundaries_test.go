package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsSubcommandsPrintUsageForHelpArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"regress --help", []string{"regress", "--help"}, "Usage: issueops regress"},
		{"regress -h", []string{"regress", "-h"}, "Usage: issueops regress"},
		{"regress help", []string{"regress", "help"}, "Usage: issueops regress"},
		{"feedback resolve --help", []string{"feedback", "resolve", "--help"}, "Usage: issueops feedback resolve"},
		{"feedback resolve -h", []string{"feedback", "resolve", "-h"}, "Usage: issueops feedback resolve"},
		{"feedback resolve help", []string{"feedback", "resolve", "help"}, "Usage: issueops feedback resolve"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
			out, err := captureStdoutAndErrorForIssueOps(t, func() error { return runIssueOps(test.args) })
			if err != nil {
				t.Fatalf("help must not fail: %v", err)
			}
			if !strings.Contains(out, test.want) {
				t.Fatalf("stdout = %q, want %q", out, test.want)
			}
		})
	}
}

func TestIssueOpsSubcommandsSurfaceFlagParseErrorsWithoutRunning(t *testing.T) {
	for _, args := range [][]string{
		{"regress", "--bogus"},
		{"link-related", "--bogus"},
		{"decision", "add", "--bogus"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
			err := runIssueOps(args)
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("err = %v, want flag parse failure", err)
			}
		})
	}
}

func TestIssueOpsFeedbackResolveRequiresExplicitIndex(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	for _, body := range []string{"first", "second"} {
		if err := runIssueOps([]string{"feedback", "add", "--id", id, "--source", "review", "--body", body, "--classification", "defect"}); err != nil {
			t.Fatalf("add feedback: %v", err)
		}
	}
	if err := runIssueOps([]string{"feedback", "resolve", "--id", id, "--resolution", "valid-defect"}); err == nil {
		t.Fatal("resolve without --index must error instead of resolving a default item")
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"status", "--id", id, "--json"})
	})
	if strings.Contains(out, "valid-defect") {
		t.Fatalf("no feedback item may be resolved without --index: %s", out)
	}
}

func TestIssueOpsPhaseJSONOmitsTrackedMaterialsWhenNothingWasCopied(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	recordIssueOpsCLIIntentForTest(t, id)
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"phase", "--id", id, "--to", "grill", "--json"})
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("phase JSON: %v\n%s", err, out)
	}
	if payload["phase"] != "grill" {
		t.Fatalf("phase = %v, want grill: %s", payload["phase"], out)
	}
	if _, present := payload["tracked_materials"]; present {
		t.Fatalf("tracked_materials must be omitted when no materials were written: %s", out)
	}
}

func TestIssueOpsArtifactStageJSONReturnsRuntimeError(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	id := startLedgerCLICycle(t)
	file := filepath.Join(t.TempDir(), "artifact.md")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"artifact", "stage", "--id", id, "--name", "no-such-artifact", "--file", file, "--json"})
	})
	if err == nil {
		t.Fatalf("staging an unknown artifact name must fail: %s", out)
	}
	assertIssueOpsStructuredFailure(t, out, "")
}
