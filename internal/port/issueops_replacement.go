package port

import (
	"context"
	model "issueops/internal/contract/issueops"
	basesync "issueops/internal/port/issueopsbasesync"
)

type ReplacementRecords interface {
	Load(string) (model.IssueOpsRecord, error)
	WithinLock(context.Context, string, func(context.Context) error) error
	Persist(context.Context, model.IssueOpsRecord, *model.NativeActor) (model.IssueOpsRecord, error)
}
type ReplacementWorkspace interface {
	WorkspaceSnapshot(model.Workspace) (string, error)
	SamePath(string, string) bool
	WorkspaceProcesses(string, map[int]bool) ([]model.ReplacementWorkspaceProcess, error)
}
type ReplacementProcessSnapshot interface {
	Inspect(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)
	AncestryPIDs(int) map[int]bool
	HasAncestor(int, map[int]bool) bool
}
type ReplacementArtifacts interface {
	Cleanup(model.IssueOpsRecord) error
	WorkspaceAbsent(string) bool
	CreateToken(model.IssueOpsRecord) (string, string, error)
}

type ReplacementInvocation struct {
	OrcaOwner ExecutionOrcaOwnerInspector
	BaseSync  basesync.Inspector
}
type ExecutionReplaceHandler func(context.Context, string, model.ExecutionReplaceRequest, ReplacementInvocation) (model.ExecutionReplaceResult, error)
