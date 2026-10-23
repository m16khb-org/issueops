package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"

	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

func TestMCPExecutionDependenciesPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueopscontract.RemotePullRequestReconcileHandler(func(context.Context, string, issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
		invoked++
		return issueopscontract.ExecutionReconcileResult{}, nil
	})

	deps := issueOpsExecutionActionDependencies(MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Publication: PublicationHandlers{Reconcile: handler}})
	if deps.RemoteReconcile == nil {
		t.Fatal("publication reconcile handler was not propagated")
	}
	if reflect.ValueOf(deps.RemoteReconcile).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("publication reconcile handler changed during MCP dependency mapping")
	}
	if invoked != 0 {
		t.Fatalf("publication reconcile handler invoked during propagation: %d", invoked)
	}
}

type recordingBaseSync struct{ basesyncport.Inspector }

func TestMCPExecutionDependenciesMatchEveryCoreActionDependency(t *testing.T) {
	core := reflect.TypeFor[port.ExecutionActionDependencies]()
	mcp := reflect.TypeFor[MCPDependencies]()
	for field := range core.Fields() {
		if field.Name == "RemoteReconcile" {
			continue
		}
		matching, ok := mcp.FieldByName(field.Name)
		if !ok || matching.Type != field.Type {
			t.Errorf("MCPDependencies lacks core action dependency %s %s", field.Name, field.Type)
		}
	}
	inspector := &recordingBaseSync{}
	deps := issueOpsExecutionActionDependencies(MCPDependencies{BaseSync: inspector})
	if deps.BaseSync != inspector {
		t.Fatalf("base sync inspector was not propagated: %#v", deps.BaseSync)
	}
}

func TestMCPExecutionDependenciesPropagateCompletionWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueopscontract.ExecutionCompleteHandler(func(context.Context, string, issueopscontract.ExecutionCompleteRequest) (issueopscontract.ExecutionResult, error) {
		invoked++
		return issueopscontract.ExecutionResult{}, nil
	})
	deps := issueOpsExecutionActionDependencies(MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Complete: handler})
	if deps.Complete == nil || reflect.ValueOf(deps.Complete).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("completion handler was not propagated unchanged")
	}
	if invoked != 0 {
		t.Fatalf("completion handler invoked during propagation: %d", invoked)
	}
}

func TestMCPPublicationReconcilePreservesToolErrorClassification(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record, receipt := publicationReconcileMCPRecord(t, issueOpsStateRootForTest())
	for _, test := range []struct {
		name        string
		handlerErr  error
		wantIsError bool
	}{
		{name: "success"},
		{name: "structured failure", handlerErr: errors.New("remote reconcile found multiple candidates; intent retained"), wantIsError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			code := "remote_reconcile_adopted"
			if test.handlerErr != nil {
				code = "remote_reconcile_multiple"
			}
			outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
				"action": "reconcile", "id": record.ID, "confirm": true,
				"host": "codex", "session_id": "publication-mcp-session",
				"session_pid": float64(receipt.PID), "session_started_at": receipt.StartedAt,
				"session_executable": receipt.Executable, "cwd": record.Execution.Workspace.Root,
			}, MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Publication: PublicationHandlers{Reconcile: func(_ context.Context, _ string, request issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
				calls++
				if request.Snapshot == nil || request.Snapshot.ID != record.ID {
					t.Fatalf("publication reconcile snapshot=%#v", request.Snapshot)
				}
				return issueopscontract.ExecutionReconcileResult{OK: test.handlerErr == nil, ID: record.ID, Code: code}, test.handlerErr
			}}})
			if calls != 1 || !outcome.Handled || outcome.IsError != test.wantIsError || outcome.Err != nil {
				t.Fatalf("calls=%d outcome=%#v", calls, outcome)
			}
			if test.wantIsError {
				payload, ok := outcome.Payload.(map[string]any)
				if !ok || payload["ok"] != false || payload["error"] != test.handlerErr.Error() {
					t.Fatalf("error payload=%#v", outcome.Payload)
				}
			} else if result, ok := outcome.Payload.(issueopscontract.ExecutionReconcileResult); !ok || !result.OK || result.ID != record.ID {
				t.Fatalf("success payload=%#v", outcome.Payload)
			}
		})
	}
}

func publicationReconcileMCPRecord(t *testing.T, stateRoot string) (issueopscontract.IssueOpsRecord, issueopscontract.NativeProcessReceipt) {
	t.Helper()
	ancestry, err := issueops.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	var receipt issueopscontract.NativeProcessReceipt
	for _, candidate := range ancestry {
		if candidate.PID == os.Getpid() {
			receipt = candidate
			break
		}
	}
	if receipt.PID == 0 {
		t.Fatalf("current process receipt missing from ancestry: %#v", ancestry)
	}
	repo, worktree := t.TempDir(), t.TempDir()
	actor := issueopscontract.NativeActor{Host: "codex", SessionID: "publication-mcp-session", SessionProcess: &receipt}
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID: (issueops.CycleStartIdentity{}).StableID(repo, "195-publication-mcp"), Repo: repo, Branch: "195-publication-mcp",
		Phase: issueopscontract.IssueOpsPhasePR, WorktreePath: worktree,
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: "195-publication-mcp", BaseHead: strings.Repeat("a", 40), LinkedAt: "2026-08-01T00:00:00Z"},
			Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-08-01T00:00:00Z"},
			Pending:   &issueopscontract.ExternalIntent{OperationID: "0123456789abcdef0123456789abcdef", Kind: "remote_pr_create", Marker: "<!-- issueops:issueops-v1 operation=0123456789abcdef0123456789abcdef -->", StartedAt: "2026-08-01T00:00:00Z"},
		},
		CreatedAt: "2026-08-01T00:00:00Z",
		UpdatedAt: "2026-08-01T00:00:00Z",
	}
	written, err := (issueops.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written, receipt
}

func TestExecutionActionRequestFromMCPMapsPrepare(t *testing.T) {
	wantAncestry := []issueopscontract.NativeProcessReceipt{{
		PID: 42, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex",
	}}
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "prepare", "id": "io-aaaaaaaaaaaa", "confirm": true,
	}, wantAncestry)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "prepare" || req.ID != "io-aaaaaaaaaaaa" || !req.Confirm {
		t.Fatalf("MCP prepare request drifted: %#v", req)
	}
	if len(req.Actor.ProcessAncestry) != 1 || req.Actor.ProcessAncestry[0] != wantAncestry[0] {
		t.Fatalf("MCP execution adapter did not preserve observed process ancestry: %#v", req.Actor.ProcessAncestry)
	}
}

func TestExecutionActionRequestFromMCPMapsCurrentClaimToken(t *testing.T) {
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "claim", "id": "io-aaaaaaaaaaaa", "claim_current_token": true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !req.ClaimCurrentToken || req.TokenFile != "" {
		t.Fatalf("MCP current-token claim request drifted: %#v", req)
	}
}

func TestExecutionActionRequestFromMCPMapsClaim(t *testing.T) {
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "claim", "id": "io-aaaaaaaaaaaa",
		"generation":         float64(3),
		"host":               "codex",
		"session_id":         "session-claim",
		"session_pid":        float64(42),
		"session_started_at": "2026-07-30T00:00:00Z",
		"session_executable": "/usr/local/bin/codex",
		"cwd":                "/repo.worktrees/claim",
		"claim_token_file":   "/tmp/token",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "claim" || req.ID != "io-aaaaaaaaaaaa" || req.Generation != 3 ||
		req.Actor.Host != "codex" || req.Actor.SessionID != "session-claim" ||
		req.Actor.SessionProcess == nil || req.Actor.SessionProcess.PID != 42 ||
		req.CWD != "/repo.worktrees/claim" || req.TokenFile != "/tmp/token" {
		t.Fatalf("MCP claim request drifted: %#v", req)
	}
}

func TestExecutionActionRequestFromMCPMapsCompletionGeneration(t *testing.T) {
	req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
		"action": "replace", "id": "io-aaaaaaaaaaaa", "replace_action": "reseed",
		"expected_generation": float64(5), "completion_generation": float64(4),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.ExpectedGeneration != 5 || req.CompletionGeneration != 4 {
		t.Fatalf("MCP reseed provenance request drifted: %#v", req)
	}
}

func TestSDKCallToolRoutesClaimToInjectedHandler(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	params, err := json.Marshal(MCPToolCall{Name: "issueops_execution", Arguments: map[string]any{
		"action": "claim", "id": "io-aaaaaaaaaaaa", "generation": float64(3), "claim_current_token": true,
		"host": "codex", "session_id": "session-claim", "session_pid": float64(42),
		"session_started_at": "2026-07-31T00:00:00Z", "session_executable": "/usr/local/bin/codex",
		"cwd": "/repo.worktrees/claim",
	}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	response, rpcErr := callSDKTool(t, params, MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Claim: func(_ context.Context, stateRoot string, request issueopscontract.ExecutionClaimRequest) (issueopscontract.ExecutionResult, error) {
		calls++
		if stateRoot == "" || request.ID != "io-aaaaaaaaaaaa" || request.Generation != 3 || request.CWD != "/repo.worktrees/claim" || !request.ClaimCurrentToken {
			t.Fatalf("claim handler request=%+v state_root=%q", request, stateRoot)
		}
		return issueopscontract.ExecutionResult{OK: true, ID: request.ID}, nil
	}})
	if rpcErr != nil || calls != 1 {
		t.Fatalf("claim MCP rpc_err=%v calls=%d", rpcErr, calls)
	}
	payload, ok := response.(map[string]any)
	if !ok {
		t.Fatalf("claim MCP response type=%T", response)
	}
	content, ok := payload["content"].([]map[string]any)
	if !ok || len(content) != 1 || !strings.Contains(content[0]["text"].(string), `"id":"io-aaaaaaaaaaaa"`) {
		t.Fatalf("claim MCP response=%#v", response)
	}
}
