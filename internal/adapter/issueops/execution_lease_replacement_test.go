package issueops

import (
	"context"
	"strings"
	"testing"

	contractissueops "issueops/internal/contract/issueops"
)

func TestExecutionReplacementRecoversDeadOwner(t *testing.T) {
	t.Parallel()

	stateRoot, record := replacementExecutionFixture(t)
	requester := executionActor("codex", "replacement-owner")
	dependencies := ExecutionReplaceDependencies{inspectWorkspace: quiescentWorkspaceInspector()}

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatalf("dead owner preview: %v", err)
	}
	if preview.InventoryFingerprint == "" {
		t.Fatal("dead owner preview did not return an inventory fingerprint")
	}

	revoked, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner process is gone",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies)
	if err != nil {
		t.Fatalf("revoke dead owner lease: %v", err)
	}
	if revoked.Execution.Lease.Generation != 2 || revoked.Execution.Lease.Status != contractissueops.LeaseStatusRevoking {
		t.Fatalf("revoke rotated the lease incorrectly: %#v", revoked.Execution.Lease)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "duplicate revoke",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies); err == nil || !strings.Contains(err.Error(), "stale lease generation") {
		t.Fatalf("duplicate revoke did not fail generation CAS: %v", err)
	}

	finalizePreview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatalf("preview dead owner finalization: %v", err)
	}
	if finalizePreview.QuiescenceFingerprint == "" {
		t.Fatal("dead owner finalization did not return a quiescence fingerprint")
	}

	finalized, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: finalizePreview.QuiescenceFingerprint,
		Actor:                 requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies)
	if err != nil {
		t.Fatalf("finalize dead owner lease: %v", err)
	}
	if finalized.Execution.Lease.Generation != 2 || finalized.Execution.Lease.Status != contractissueops.LeaseStatusClaimable || finalized.Execution.Lease.Holder != nil {
		t.Fatalf("finalized replacement lease is not claimable: %#v", finalized.Execution.Lease)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Generation != 2 || persisted.Execution.Lease.Status != contractissueops.LeaseStatusClaimable || persisted.Execution.Lease.Holder != nil {
		t.Fatalf("durable finalize lease=%#v", persisted.Execution.Lease)
	}
}

func TestExecutionReplacementRefusesFinalizeWhileOwnerIsLive(t *testing.T) {
	t.Parallel()

	stateRoot, record := replacementExecutionFixture(t)
	liveOwner := executionActor("codex", "live-owner")
	record.Execution.Lease.Holder = &liveOwner
	if _, err := writeIssueOps(context.Background(), stateRoot, record); err != nil {
		t.Fatal(err)
	}
	requester := executionActor("codex", "replacement-owner")
	dependencies := ExecutionReplaceDependencies{inspectWorkspace: quiescentWorkspaceInspector()}

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "live owner check",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies); err != nil {
		t.Fatal(err)
	}

	_, err = ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err == nil || !strings.Contains(err.Error(), "old holder process is still live") {
		t.Fatalf("a live old holder must block finalization: %v", err)
	}
}

func replacementExecutionFixture(t *testing.T) (string, contractissueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "270-dead-owner")
	record := fixture.record
	record.IssueURL = "https://github.com/example/issueops/issues/270"
	record.BranchPrepare.IssueURL = record.IssueURL
	record.Execution.Lease = contractissueops.WriteLease{
		Generation: 1, Status: contractissueops.LeaseStatusActive,
		Holder: &contractissueops.NativeActor{
			Host: "codex", SessionID: "dead-owner",
			SessionProcess: &contractissueops.NativeProcessReceipt{
				PID: 999999, StartedAt: "2026-08-03T00:00:00Z", Executable: "/usr/bin/codex",
			},
		},
		ClaimedAt: "2026-08-03T00:00:00Z",
	}
	written, err := writeIssueOps(context.Background(), stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}

// 워크스페이스 quiescence 관측은 시스템 전역 lsof다. lease 상태 기계를 검증하는
// 테스트가 호스트의 열린 파일 목록과 그 프로브 상한에 묶이면 부하에서 깨지므로,
// finalize 경로는 주입된 관측자를 써야 한다.
func TestExecutionFinalizePreviewUsesInjectedWorkspaceInspector(t *testing.T) {
	t.Parallel()

	stateRoot, record := replacementExecutionFixture(t)
	requester := executionActor("codex", "replacement-owner")
	observed := 0
	dependencies := ExecutionReplaceDependencies{
		inspectWorkspace: func(string, map[int]bool) ([]workspaceProcess, error) {
			observed++
			return []workspaceProcess{{
				PID: 999998, Command: "vim", FD: "cwd", Access: "r",
				Path: record.Execution.Workspace.Root,
			}}, nil
		},
	}

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "injected inspector",
		Actor: requester, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, dependencies); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	_, err = ReplaceExecutionWithDependencies(context.Background(), stateRoot, contractissueops.ExecutionReplaceRequest{
		ID: record.ID, Action: contractissueops.ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: record.Execution.Workspace.Root,
	}, dependencies)
	if err == nil || !strings.Contains(err.Error(), "workspace process is not quiescent: pid=999998") {
		t.Fatalf("injected workspace occupancy did not block finalization: %v", err)
	}
	if observed == 0 {
		t.Fatal("finalize preview did not use the injected workspace inspector")
	}
}
