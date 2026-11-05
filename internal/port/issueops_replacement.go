package port

import (
	"context"
	model "issueops/internal/contract/issueops"
	basesync "issueops/internal/port/issueopsbasesync"
)

// ReplacementRecords is the record access an execution replacement needs: load a record, run a
// function under the per-record lock, and persist the replaced record with the previous native
// actor.
type ReplacementRecords interface {
	Load(string) (model.Record, error)
	WithinLock(context.Context, string, func(context.Context) error) error
	Persist(context.Context, model.Record, *model.NativeActor) (model.Record, error)
}

// ReplacementWorkspace inspects the workspace being replaced: a snapshot identity of it, path
// comparison, and the processes still running inside it.
type ReplacementWorkspace interface {
	WorkspaceSnapshot(model.Workspace) (string, error)
	SamePath(string, string) bool
	WorkspaceProcesses(string, map[int]bool) ([]model.ReplacementWorkspaceProcess, error)
}

// ReplacementProcessSnapshot inspects process receipts and ancestry so replacement can tell the
// requesting process apart from the processes it replaces.
type ReplacementProcessSnapshot interface {
	Inspect(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)
	AncestryPIDs(int) map[int]bool
	HasAncestor(int, map[int]bool) bool
}

// ReplacementArtifacts handles the on-disk artifacts of a replaced generation: cleaning them up,
// checking the workspace root is gone, and creating the claim token (returning its hash and path).
type ReplacementArtifacts interface {
	Cleanup(model.Record) error
	WorkspaceAbsent(string) bool
	CreateToken(model.Record) (string, string, error)
}

// ReplacementInvocation carries the per-call collaborators an execution replace handler needs
// beyond its request, namely the base sync inspector.
type ReplacementInvocation struct {
	BaseSync basesync.Inspector
}

// ExecutionReplaceHandler replaces an execution for the state root, request and invocation given,
// and returns the replace result.
type ExecutionReplaceHandler func(context.Context, string, model.ExecutionReplaceRequest, ReplacementInvocation) (model.ExecutionReplaceResult, error)
