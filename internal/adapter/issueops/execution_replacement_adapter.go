package issueops

import (
	"context"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ReplacementRecords struct{ StateRoot string }

func (s ReplacementRecords) Load(id string) (issueops.IssueOpsRecord, error) {
	return ReadIssueOps(s.StateRoot, id)
}
func (s ReplacementRecords) WithinLock(ctx context.Context, id string, fn func() error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, func(context.Context) error { return fn() })
}
func (s ReplacementRecords) Persist(record issueops.IssueOpsRecord, previous *issueops.NativeActor) (issueops.IssueOpsRecord, error) {
	return persistExecutionTransition(s.StateRoot, record, previous)
}

type ReplacementWorkspace struct {
	Snapshot         LeaseWorkspaceSnapshot
	inspectWorkspace executionWorkspaceProcessInspector
}
type executionWorkspaceProcessInspector func(string, map[int]bool) ([]workspaceProcess, error)

func (s ReplacementWorkspace) WorkspaceSnapshot(workspace issueops.Workspace) (string, error) {
	return s.Snapshot.Snapshot(workspace)
}
func (s ReplacementWorkspace) SamePath(a, b string) bool { return samePath(a, b) }
func (s ReplacementWorkspace) WorkspaceProcesses(root string, excluded map[int]bool) ([]issueops.ReplacementWorkspaceProcess, error) {
	inspect := s.inspectWorkspace
	if inspect == nil {
		inspect = inspectWorkspaceProcesses
	}
	observed, err := inspect(root, excluded)
	if err != nil {
		return nil, err
	}
	result := make([]issueops.ReplacementWorkspaceProcess, len(observed))
	for i, item := range observed {
		result[i] = issueops.ReplacementWorkspaceProcess(item)
	}
	return result, nil
}

type replacementProcessSnapshot struct {
	entries map[int]nativeProcessSnapshotEntry
}

func ObserveReplacementProcesses() port.ReplacementProcessSnapshot {
	entries, _ := observeNativeProcessSnapshot()
	return replacementProcessSnapshot{entries: entries}
}
func (s replacementProcessSnapshot) Inspect(receipt issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error) {
	return inspectNativeProcessReceiptForRollover(receipt, s.entries)
}
func (s replacementProcessSnapshot) AncestryPIDs(pid int) map[int]bool {
	return nativeProcessAncestryPIDsFromSnapshot(s.entries, pid)
}
func (s replacementProcessSnapshot) HasAncestor(pid int, owners map[int]bool) bool {
	return processHasAncestorInSnapshot(s.entries, pid, owners)
}

type ReplacementArtifacts struct{}

func (s ReplacementArtifacts) Cleanup(record issueops.IssueOpsRecord) error {
	return cleanupReplacementGeneration(record)
}
func (s ReplacementArtifacts) WorkspaceAbsent(root string) bool { return workspaceRootAbsent(root) }
func (s ReplacementArtifacts) CreateToken(record issueops.IssueOpsRecord) (string, string, error) {
	token, path, err := createClaimToken(record)
	if err != nil {
		return "", "", err
	}
	return tokenSHA256(token), path, nil
}
