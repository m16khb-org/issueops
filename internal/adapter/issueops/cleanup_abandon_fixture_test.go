package issueops

import (
	"context"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type CleanupAbandonDeps struct {
	Git           func(string, ...string) (int, string)
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
	Orca          port.ExecutionOrcaProvisioner
	OrcaOwner     port.ExecutionOrcaOwnerInspector
	Remote        port.IssueProvider
}

func CleanupAbandon(ctx context.Context, root string, req CleanupAbandonRequest, deps CleanupAbandonDeps) (CleanupAbandonResult, error) {
	return abandonExecutorForTests(root, deps).Run(ctx, req)
}
func abandonExecutorForTests(root string, deps CleanupAbandonDeps) cleanupapp.AbandonExecutor {
	runtime := CleanupAbandonRuntime{StateRoot: root, Git: deps.Git, Processes: deps.Processes, OrcaTerminals: deps.OrcaTerminals}
	return cleanupapp.AbandonExecutor{
		Records: CleanupRecordStore{StateRoot: root}, Acquire: (CleanupLifetimeLock{StateRoot: root}).Acquire,
		Provider: func(string) (port.IssueProvider, error) { return deps.Remote, nil },
		Observe: func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupAbandonRequest, _ port.IssueProvider) (model.CleanupAbandonRequest, error) {
			return r, nil
		},
		Plan: abandonPreviewerForTests(runtime, deps).Plan, NewAttempt: NewCleanupAttempt, Stop: runtime.Stop, Directory: (CleanupFinishEnvironment{}).Directory, Git: runtime.Command, Now: time.Now,
	}
}

func abandonPreviewerForTests(runtime CleanupAbandonRuntime, deps CleanupAbandonDeps) cleanupapp.AbandonPreviewer {
	return cleanupapp.AbandonPreviewer{
		Environment: runtime, ReadChild: runtime.ReadChild, Workspace: runtime.Workspace,
		Orca:   cleanupapp.AbandonOrcaObserver{ReadIntent: runtime.ReadIntent, InspectionRequest: runtime.InspectionRequest, Orca: deps.Orca, Owner: deps.OrcaOwner},
		Remote: cleanupapp.AbandonRemoteObserver{RemoteRef: (LinkedBranchRemoteRef{RunGit: runtime.Command}).Observe},
	}
}
