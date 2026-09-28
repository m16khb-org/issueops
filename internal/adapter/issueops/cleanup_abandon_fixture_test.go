package issueops

import (
	"context"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type CleanupAbandonDeps = CleanupAbandonRuntime

func CleanupAbandon(ctx context.Context, root string, req CleanupAbandonRequest, deps CleanupAbandonDeps) (CleanupAbandonResult, error) {
	return abandonExecutorForTests(root, deps).Run(ctx, req)
}
func abandonExecutorForTests(root string, deps CleanupAbandonDeps) cleanupapp.AbandonExecutor {
	deps.StateRoot = root
	return cleanupapp.AbandonExecutor{
		Records: CleanupRecordStore{StateRoot: root}, Acquire: (CleanupLifetimeLock{StateRoot: root}).Acquire,
		Provider: func(string) (port.IssueProvider, error) { return deps.Remote, nil },
		Observe: func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupAbandonRequest, _ port.IssueProvider) (model.CleanupAbandonRequest, error) {
			return r, nil
		},
		Plan: deps.Plan, NewAttempt: NewCleanupAttempt, Stop: deps.Stop, Directory: (CleanupFinishEnvironment{}).Directory, Git: deps.Command, Now: time.Now,
	}
}
