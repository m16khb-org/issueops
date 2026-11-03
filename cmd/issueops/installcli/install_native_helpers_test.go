package installcli

import (
	"path/filepath"
	"strings"
	"testing"

	installdomain "issueops/internal/domain/install"

	"issueops/internal/port"
)

func TestPrintInstallNativeResultCoversDryRunAndProjectLocalModes(t *testing.T) {
	result := port.NativeInstallResult{
		OK:           true,
		Root:         "/repo/harness",
		Home:         "/home/user",
		CodexHome:    "/home/user/.codex",
		BinPath:      "/repo/harness/bin/issueops",
		ProjectLocal: false,
		DryRun:       true,
		Messages:     []string{"planned install"},
		CommandPath: &port.ManagedCommandPathResult{
			Path: "/home/user/.local/bin/issueops", Target: "/repo/harness/bin/issueops", BackupPath: "/home/user/.local/bin/.backup",
			AdoptionApproved: true, WouldAdopt: true, RollbackAvailable: true,
		},
	}

	dryRunOut := captureStatusVerifyStdout(t, func() error {
		printInstallNativeResult(result)
		return nil
	})
	for _, want := range []string{
		"Dry-run plan for issueops native integrations:",
		"- mode: user/global only",
		"- Project-local repo files: unchanged by default",
		"- planned install",
		"managed command adoption: /home/user/.local/bin/issueops -> /repo/harness/bin/issueops (would adopt)",
		"rollback_available=true",
	} {
		if !strings.Contains(dryRunOut, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, dryRunOut)
		}
	}

	result.ProjectLocal = true
	result.DryRun = false
	installedOut := captureStatusVerifyStdout(t, func() error {
		printInstallNativeResult(result)
		return nil
	})
	for _, want := range []string{
		"Installed issueops native integrations:",
		"- mode: user/global + explicit project-local",
		"- Project-local Claude MCP config: " + filepath.Join(result.Root, ".mcp.json"),
	} {
		if !strings.Contains(installedOut, want) {
			t.Fatalf("installed output missing %q:\n%s", want, installedOut)
		}
	}
	if strings.Contains(installedOut, "Project-local Claude skills") {
		t.Fatalf("project-local install must not advertise repo-local skill links:\n%s", installedOut)
	}
}

func TestNativeActivationStepAcceptsOnlyInternalTransitionModes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dryRun bool
		raw    string
		want   string
		valid  bool
	}{
		{name: "normal", valid: true},
		{name: "begin", raw: "begin", want: "begin", valid: true},
		{name: "seal", raw: "seal", want: "seal", valid: true},
		{name: "abort", raw: "abort", want: "abort", valid: true},
		{name: "unknown", raw: "other"},
		{name: "dry_run_transition", dryRun: true, raw: "begin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := installdomain.ActivationStep(tc.dryRun, tc.raw)
			if tc.valid && (err != nil || got != tc.want) {
				t.Fatalf("step=%q err=%v", got, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("invalid step accepted: %q", got)
			}
		})
	}
}
