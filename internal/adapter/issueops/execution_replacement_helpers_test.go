package issueops

import (
	"context"
	app "issueops/internal/application/issueopsreplacement"
	model "issueops/internal/contract/issueops"
	basesync "issueops/internal/port/issueopsbasesync"
	"os"
	"time"
)

type ExecutionReplaceDependencies struct {
	BaseSync         basesync.Inspector
	inspectWorkspace executionWorkspaceProcessInspector
}

func ReplaceExecutionWithDependencies(ctx context.Context, stateRoot string, req model.ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (model.ExecutionReplaceResult, error) {
	service := app.Service{
		PID: os.Getpid(), ObserveProcesses: ObserveReplacementProcesses,
		Records:   ReplacementRecords{StateRoot: stateRoot},
		Workspace: ReplacementWorkspace{Snapshot: LeaseWorkspaceSnapshot{GitCmd: GitCmd, GitCmdRaw: GitCmdRaw}, inspectWorkspace: deps.inspectWorkspace},
		Artifacts: ReplacementArtifacts{},
		BaseSync:  deps.BaseSync, InspectProcess: InspectNativeProcessReceipt, Verifier: NativeActorVerifier(),
		Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}
	return service.Run(ctx, req)
}
