package issueops

import (
	"context"
	app "issueops/internal/application/issueopsreplacement"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesync "issueops/internal/port/issueopsbasesync"
	"os"
	"time"
)

type ExecutionReplaceRequest = model.ExecutionReplaceRequest
type ExecutionReplaceDependencies struct {
	OrcaOwner        port.ExecutionOrcaOwnerInspector
	BaseSync         basesync.Inspector
	ReadIssue        ExecutionIssueSnapshotReadFunc
	inspectWorkspace executionWorkspaceProcessInspector
}

func ReplaceExecutionWithDependencies(ctx context.Context, stateRoot string, req model.ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (model.ExecutionReplaceResult, error) {
	service := app.Service{
		PID: os.Getpid(), OrcaOwner: deps.OrcaOwner, ObserveProcesses: ObserveReplacementProcesses,
		Records:   ReplacementRecords{StateRoot: stateRoot},
		Workspace: ReplacementWorkspace{Snapshot: LeaseWorkspaceSnapshot{GitCmd: GitCmd, GitCmdRaw: GitCmdRaw}, inspectWorkspace: deps.inspectWorkspace},
		Artifacts: ReplacementArtifacts{}, ResealOwner: ownerContextForTest(stateRoot, deps.ReadIssue).Reseal,
		BaseSync: deps.BaseSync, InspectProcess: inspectNativeProcessReceipt,
		Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}
	return service.Run(ctx, req)
}
