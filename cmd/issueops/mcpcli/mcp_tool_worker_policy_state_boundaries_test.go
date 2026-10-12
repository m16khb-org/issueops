package mcpcli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	auditapp "issueops/internal/application/audit"
	workerapp "issueops/internal/application/worker"
	auditcontract "issueops/internal/contract/audit"
	statecontract "issueops/internal/contract/state"
	webfetchcontract "issueops/internal/contract/webfetch"
)

type failingAuditWriter struct{ err error }

func (w failingAuditWriter) Path() (string, error) { return "", w.err }
func (failingAuditWriter) Append(string, auditcontract.CommandAuditRecord) error {
	return nil
}

// failingWorkerDirEffects는 Dir 조회가 실패하는 worker 저장소를 흉내 낸다.
type failingWorkerDirEffects struct {
	workerapp.Effects
	err error
}

func (e failingWorkerDirEffects) Dir() (string, error) { return "", e.err }

func requireProtocolFailure(t *testing.T, outcome MCPToolOutcome, code int64, message string) {
	t.Helper()
	if !outcome.Handled || outcome.Err == nil || outcome.Err.Code != code || outcome.Err.Message != message {
		t.Fatalf("outcome = %#v, want handled error code %d message %q", outcome, code, message)
	}
}

func TestAssistantWorkerMCPToolFailuresUseServerErrorCode(t *testing.T) {
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	root := t.TempDir()
	missingRepo := filepath.Join(t.TempDir(), "missing-repo")
	deps := testTransportServices()
	deps.Fetch = func(context.Context, webfetchcontract.Request) (webfetchcontract.Result, error) {
		return webfetchcontract.Result{}, errors.New("fetch backend down")
	}
	failingDir := deps
	failingDir.Worker = workerapp.Service{Effects: failingWorkerDirEffects{Effects: testWorkerStore(), err: errors.New("worker dir unavailable")}}

	tests := []struct {
		name    string
		deps    MCPDependencies
		call    MCPToolCall
		message string
	}{
		{"commit suggest", deps, MCPToolCall{Name: "commit_suggest", Arguments: map[string]any{"repo": missingRepo}}, "commit_suggest failed"},
		{"lint diagnose", deps, MCPToolCall{Name: "lint_diagnose", Arguments: map[string]any{"repo": missingRepo, "command_argv": []any{"git", "status"}}}, "lint_diagnose failed"},
		{"worker list", failingDir, MCPToolCall{Name: "worker_list", Arguments: map[string]any{}}, "worker_list failed"},
		{"worker cancel", deps, MCPToolCall{Name: "worker_cancel", Arguments: map[string]any{"id": "../bad"}}, "worker_cancel failed"},
		{"web fetch", deps, MCPToolCall{Name: "web_fetch_resilient", Arguments: map[string]any{"url": "https://example.test/"}}, "web_fetch_resilient failed"},
		{"worker run read only", deps, MCPToolCall{Name: "worker_run_read_only", Arguments: map[string]any{"kind": "", "workspace_root": root, "cwd": root, "argv": []any{"git", "status"}}}, "worker_run_read_only failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handleAssistantWorkerMCPToolCall(context.Background(), tc.call, tc.deps)
			requireProtocolFailure(t, outcome, -32000, tc.message)
		})
	}
}

func TestPolicyStateMCPToolFailuresUseDocumentedErrorCodes(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	boom := errors.New("backend failure")
	state := publicStateForTest()
	state.List = func() (statecontract.StateListResult, error) { return statecontract.StateListResult{}, boom }
	state.Doctor = func() (statecontract.StateDoctorResult, error) { return statecontract.StateDoctorResult{}, boom }
	state.Maintain = func() (statecontract.StateMaintainResult, error) { return statecontract.StateMaintainResult{}, boom }
	state.Prune = func(context.Context, time.Duration, bool) (statecontract.StatePruneResult, error) {
		return statecontract.StatePruneResult{}, boom
	}
	deps := MCPDependencies{
		Policy: testPolicyService(),
		Audit:  auditapp.Service{Evaluator: testPolicyService(), Writer: failingAuditWriter{err: boom}, Clock: testAuditService().Clock},
		State:  state,
	}

	tests := []struct {
		name    string
		call    MCPToolCall
		code    int64
		message string
	}{
		{"audit", MCPToolCall{Name: "command_policy_audit", Arguments: map[string]any{"workspace_root": root, "cwd": root, "argv": []any{"git", "status"}}}, -32000, "command_policy_audit failed"},
		{"state write", MCPToolCall{Name: "state_write", Arguments: map[string]any{"key": "", "content": "x"}}, -32602, "State write failed"},
		{"state list", MCPToolCall{Name: "state_list", Arguments: map[string]any{}}, -32000, "State list failed"},
		{"state prune", MCPToolCall{Name: "state_prune", Arguments: map[string]any{"max_age": "1h"}}, -32602, "State prune failed"},
		{"state doctor", MCPToolCall{Name: "state_doctor", Arguments: map[string]any{}}, -32000, "State doctor failed"},
		{"state maintain", MCPToolCall{Name: "state_maintain", Arguments: map[string]any{}}, -32000, "State maintain failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := handlePolicyStateMCPToolCall(context.Background(), tc.call, deps)
			requireProtocolFailure(t, outcome, tc.code, tc.message)
		})
	}
}
