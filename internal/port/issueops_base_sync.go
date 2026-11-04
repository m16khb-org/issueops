package port

import (
	"context"

	"issueops/internal/contract/issueops"
)

// BaseSyncPushEffects is the side-effect surface for pushing the synced branch: Push runs the push
// and returns the exit code and output, and AppendEvent records the sync-base event on the
// execution record.
type BaseSyncPushEffects interface {
	Push(context.Context, string, string) (int, string)
	AppendEvent(context.Context, string, issueops.ExecutionSyncBaseEvent) error
}

// BaseSyncAbortEffects is the side-effect surface for abandoning a base sync: abort the in-progress
// merge (exit code and output) and clear the stored conflict resolution.
type BaseSyncAbortEffects interface {
	AbortMerge(context.Context, string) (int, string)
	ClearResolution(context.Context, string) error
}

// BaseSyncFinalizeEffects is the side-effect surface for finishing a conflict resolution: it
// inspects conflicts and staged changes, commits, reads the new head, and pushes through the
// embedded push effects.
type BaseSyncFinalizeEffects interface {
	BaseSyncPushEffects
	ConflictCount(context.Context, string) int
	UnmergedPaths(context.Context, string) []string
	CheckStaged(context.Context, string) (int, string)
	Commit(context.Context, string) (int, string)
	Head(context.Context, string) (int, string)
	Now() string
}

// BaseSyncApplyEffects is the side-effect surface for applying a base sync: predict conflicts,
// merge, record or clear an unresolved-conflict resolution, abort the merge, read the head, and
// push through the embedded push effects.
type BaseSyncApplyEffects interface {
	BaseSyncPushEffects
	PredictConflicts(context.Context, string, string, string) ([]string, error)
	Merge(context.Context, string, string) (int, string)
	UnmergedPaths(context.Context, string) []string
	StartResolution(context.Context, string, []string) error
	AbortMerge(context.Context, string) (int, string)
	Head(context.Context, string) (int, string)
	Now() string
}
