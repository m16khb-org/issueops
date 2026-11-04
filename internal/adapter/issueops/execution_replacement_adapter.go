package issueops

import (
	"context"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// ReplacementRecords gives lease replacement access to cycle records under StateRoot.
type ReplacementRecords struct{ StateRoot string }

// Load reads the cycle record for id.
func (s ReplacementRecords) Load(id string) (issueops.Record, error) {
	return ReadIssueOps(s.StateRoot, id)
}

// WithinLock runs fn while holding the per-cycle lock for id.
func (s ReplacementRecords) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, fn)
}

// Persist saves the replacement transition of record; previous is the actor that held
// the lease before the transition.
func (s ReplacementRecords) Persist(ctx context.Context, record issueops.Record, previous *issueops.NativeActor) (issueops.Record, error) {
	return persistExecutionTransition(ctx, s.StateRoot, record, previous)
}

// ReplacementWorkspace observes the workspace during lease replacement: its snapshot
// fingerprint and the processes that occupy it.
type ReplacementWorkspace struct {
	Snapshot         LeaseWorkspaceSnapshot
	inspectWorkspace executionWorkspaceProcessInspector
}
type executionWorkspaceProcessInspector func(string, map[int]bool) ([]workspaceProcess, error)

// WorkspaceSnapshot returns the fingerprint of workspace.
func (s ReplacementWorkspace) WorkspaceSnapshot(workspace issueops.Workspace) (string, error) {
	return s.Snapshot.Snapshot(workspace)
}

// SamePath reports whether a and b name the same location.
func (s ReplacementWorkspace) SamePath(a, b string) bool { return samePath(a, b) }

// WorkspaceProcesses lists the processes occupying the worktree at root, skipping the
// PIDs in excluded.
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

// ObserveReplacementProcesses captures one process-table snapshot for replacement
// decisions. If the table cannot be read the snapshot is empty, and every lookup against it
// fails closed as an absent process.
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

// ReplacementArtifacts manages the claim token files that replacement creates and revokes.
type ReplacementArtifacts struct{}

// Cleanup removes the claim token file of the not-yet-committed replacement generation
// for record.
func (s ReplacementArtifacts) Cleanup(record issueops.Record) error {
	return cleanupReplacementGeneration(record)
}

// WorkspaceAbsent reports whether the canonical worktree at root no longer exists.
func (s ReplacementArtifacts) WorkspaceAbsent(root string) bool { return workspaceRootAbsent(root) }

// CreateToken writes a new claim token file for record and returns the token SHA-256
// and the file path; the raw token stays on disk only.
func (s ReplacementArtifacts) CreateToken(record issueops.Record) (string, string, error) {
	token, path, err := createClaimToken(record)
	if err != nil {
		return "", "", err
	}
	return tokenSHA256(token), path, nil
}
