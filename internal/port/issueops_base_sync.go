package port

import (
	"context"

	"issueops/internal/contract/issueops"
)

type BaseSyncPushEffects interface {
	Push(context.Context, string, string) (int, string)
	AppendEvent(context.Context, string, issueops.ExecutionSyncBaseEvent) error
}

type BaseSyncAbortEffects interface {
	AbortMerge(context.Context, string) (int, string)
	ClearResolution(context.Context, string) error
}

type BaseSyncFinalizeEffects interface {
	BaseSyncPushEffects
	ConflictCount(context.Context, string) int
	UnmergedPaths(context.Context, string) []string
	CheckStaged(context.Context, string) (int, string)
	Commit(context.Context, string) (int, string)
	Head(context.Context, string) (int, string)
	Now() string
}
