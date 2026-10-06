package mcpsmoke

import (
	"errors"
	"fmt"
	verifydomain "issueops/internal/domain/selfverify"
	"strings"
	"testing"
	"time"

	selfverify "issueops/internal/contract/selfverify"
)

func TestMCPSmokeExpectedMarkers(t *testing.T) {
	markers := MCPSmokeExpectedMarkers()
	if len(markers) == 0 || !MCPSmokeHasExpectedMarkers(strings.Join(markers, "\n")) {
		t.Fatal("expected marker set to satisfy marker check")
	}
	if MCPSmokeHasExpectedMarkers("atomic_commit_preflight only") {
		t.Fatal("partial marker output should fail")
	}
}

func TestValidateMCPSmokeContract(t *testing.T) {
	stdout := validMCPSmokeStdout()
	step := selfverify.StepResult{OK: true, Stdout: stdout}
	ValidateMCPSmokeContract(&step)
	if !step.OK || step.Error != "" {
		t.Fatalf("valid contract failed: %#v", step)
	}
	for _, tc := range []struct {
		name   string
		stdout string
		want   string
	}{
		{name: "wrong count", stdout: `{}`, want: "expected 11 MCP SDK results"},
		{name: "bad json", stdout: strings.Repeat(`{}`+"\n", 10) + `not json`, want: "invalid JSON"},
		{name: "missing markers", stdout: strings.Repeat(`{}`+"\n", 11), want: "expected tool/resource"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := selfverify.StepResult{OK: true, Stdout: tc.stdout}
			ValidateMCPSmokeContract(&step)
			if step.OK || !strings.Contains(step.Error, tc.want) {
				t.Fatalf("got %#v, want error containing %q", step, tc.want)
			}
		})
	}
}

func TestValidateMCPWithDepsRunsSmokeAndCleanup(t *testing.T) {
	var removed []string
	deps := MCPValidationDeps{
		MkdirTemp: func(_, pattern string) (string, error) {
			return "/tmp/" + strings.TrimSuffix(pattern, "*") + "x", nil
		},
		RemoveAll: func(path string) error {
			removed = append(removed, path)
			return nil
		},
		RunSDKSmoke: func(dir, name string, env []string, timeout time.Duration) selfverify.StepResult {
			if dir != "/repo" || name != "bin" {
				return selfverify.StepResult{OK: false, Error: "unexpected command"}
			}
			return selfverify.StepResult{OK: true, Stdout: validMCPSmokeStdout()}
		},
	}
	step := ValidateMCPWithDeps("bin", "/repo", deps)
	if !step.OK || step.Error != "" {
		t.Fatalf("ValidateMCPWithDeps failed: %#v", step)
	}
	if len(removed) != 1 {
		t.Fatalf("cleanup not called as expected: removed=%#v", removed)
	}
	if !step.StdoutTruncated || step.StdoutBytes <= aggregateOutputBudgetBytes {
		t.Fatalf("expected truncated aggregate stdout, got bytes=%d truncated=%v", step.StdoutBytes, step.StdoutTruncated)
	}
}

func TestValidateMCPWithDepsFailurePaths(t *testing.T) {
	step := ValidateMCPWithDeps("bin", "/repo", MCPValidationDeps{
		MkdirTemp: func(_, _ string) (string, error) { return "", errors.New("mkdir failed") },
	})
	if step.OK || !strings.Contains(step.Error, "mkdir failed") {
		t.Fatalf("expected mkdir failure, got %#v", step)
	}
	step = ValidateMCPWithDeps("bin", "/repo", MCPValidationDeps{
		MkdirTemp: func(_, pattern string) (string, error) { return "/tmp/" + pattern, nil },
		RemoveAll: func(string) error { return nil },
		RunSDKSmoke: func(string, string, []string, time.Duration) selfverify.StepResult {
			return selfverify.StepResult{OK: false, Error: "mcp failed"}
		},
	})
	if step.OK || step.Error != "mcp failed" {
		t.Fatalf("expected command failure, got %#v", step)
	}
}

func TestDepsDefaultsAndSmallHelpers(t *testing.T) {
	deps := (MCPValidationDeps{}).withDefaults()
	if deps.MkdirTemp == nil || deps.RemoveAll == nil || deps.RunSDKSmoke == nil {
		t.Fatal("defaults should populate dependencies")
	}
	step := verifydomain.FailedStep("label", errors.New("boom"))
	if step.OK || step.Label != "label" || !strings.Contains(step.Error, "boom") {
		t.Fatalf("unexpected failed step: %#v", step)
	}
	stdout, truncated, bytes := verifydomain.TailWithBudget("abcdef", 3)
	if stdout != "[tr" || !truncated || bytes != 6 {
		t.Fatalf("unexpected tail budget result stdout=%q truncated=%v bytes=%d", stdout, truncated, bytes)
	}
	if lines := splitLines(" a \n\n b \n"); len(lines) != 3 {
		t.Fatalf("splitLines preserves non-trimmed internal blank shape, got %#v", lines)
	}
}

func validMCPSmokeStdout() string {
	markers := strings.Join(MCPSmokeExpectedMarkers(), " ") + strings.Repeat("x", 800)
	var b strings.Builder
	for i := 1; i <= 11; i++ {
		fmt.Fprintf(&b, `{"id":%d,"text":%q}`+"\n", i, markers)
	}
	return b.String()
}
