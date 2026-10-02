package contractauditworker

import (
	selfverify "issueops/internal/contract/selfverify"

	"testing"
	"time"
)

func TestSuccessfulProbesPreserveCommandDurations(t *testing.T) {
	root := t.TempDir()
	commands := map[string]selfverify.StepResult{
		"contract check": {
			OK: true, DurationMS: 137,
			Stdout: `{"ok":true,"hash":"fixture","cli_commands":[{"name":"worker"},{"name":"contract"},{"name":"policy"}]}`,
		},
		"tool contract conformance": {
			OK: true, DurationMS: 149,
			Stdout: `{"ok":true,"case_count":10,"gate":{"decision":"baseline_passed"}}`,
		},
		"command audit smoke": {
			OK: true, DurationMS: 163,
		},
		"worker lifecycle enqueue": {
			OK: true, DurationMS: 11,
			Stdout: `{"id":"job","status":"queued","no_shell":true}`,
		},
		"worker lifecycle status": {OK: true, DurationMS: 13},
		"worker lifecycle cancel": {OK: true, DurationMS: 17},
		"worker lifecycle list":   {OK: true, DurationMS: 19},
	}
	commandResult := func(label string) selfverify.StepResult {
		result, ok := commands[label]
		if !ok {
			t.Fatalf("unexpected command label %q", label)
		}
		result.Label = label
		return result
	}
	deps := ValidationDeps{
		MkdirTemp: func(string, string) (string, error) { return root, nil },
		RemoveAll: func(string) error { return nil },
		ReadFile: func(string) ([]byte, error) {
			return []byte(`{"kind":"command_policy_audit","audit_log_id":"fixture"}`), nil
		},
		RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) selfverify.StepResult {
			return commandResult(label)
		},
		RunCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, _ ...string) selfverify.StepResult {
			return commandResult(label)
		},
	}
	tests := []struct {
		name   string
		run    func() selfverify.StepResult
		wantMs int64
	}{
		{
			name: "contract", wantMs: 137,
			run: func() selfverify.StepResult {
				return ValidateContractCheckWithDeps("issueops", root, deps)
			},
		},
		{
			name: "conformance", wantMs: 149,
			run: func() selfverify.StepResult {
				return ValidateToolConformanceWithDeps("issueops", root, deps)
			},
		},
		{
			name: "audit", wantMs: 163,
			run: func() selfverify.StepResult {
				return ValidateCommandAuditWithDeps("issueops", root, 1, deps)
			},
		},
		{
			name: "worker", wantMs: 60,
			run: func() selfverify.StepResult {
				return ValidateWorkerLifecycleWithDeps("issueops", root, 1, deps)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.run()
			if !result.OK {
				t.Fatalf("probe failed: %+v", result)
			}
			if result.DurationMS != tt.wantMs {
				t.Fatalf("duration_ms=%d, want %d", result.DurationMS, tt.wantMs)
			}
		})
	}
}
