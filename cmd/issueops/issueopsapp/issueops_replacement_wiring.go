package issueopsapp

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	app "issueops/internal/application/issueopsreplacement"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"os"
	"time"
)

func newIssueOpsReplacementHandler() port.ExecutionReplaceHandler {
	snapshot := adapter.LeaseWorkspaceSnapshot{GitCmd: preflight.GitCmd, GitCmdRaw: preflight.GitCmdRaw}
	return func(ctx context.Context, stateRoot string, req model.ExecutionReplaceRequest, invocation port.ReplacementInvocation) (model.ExecutionReplaceResult, error) {
		service := app.Service{
			PID: os.Getpid(), OrcaOwner: invocation.OrcaOwner, ObserveProcesses: adapter.ObserveReplacementProcesses,
			Records:   adapter.ReplacementRecords{StateRoot: stateRoot},
			Workspace: adapter.ReplacementWorkspace{Snapshot: snapshot},
			Artifacts: adapter.ReplacementArtifacts{}, ResealOwner: newIssueOpsOwnerContext(stateRoot, req.ReadIssue).Reseal,
			BaseSync: invocation.BaseSync, InspectProcess: adapter.InspectNativeProcessReceipt,
			Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		}
		return service.Run(ctx, req)
	}
}
