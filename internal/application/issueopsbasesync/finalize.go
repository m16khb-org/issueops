package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

type FinalizeRequest struct {
	Push PushRequest
}

type FinalizeResult struct {
	ConflictFiles     []string
	Missing           string
	MergeCommit       string
	Merged            bool
	Pushed            bool
	PushRetryRequired bool
	FailedStep        string
	DirectError       bool
}

func Finalize(ctx context.Context, request FinalizeRequest, effects port.BaseSyncFinalizeEffects) (FinalizeResult, error) {
	if effects == nil {
		return FinalizeResult{}, fmt.Errorf("base sync finalize effects are required")
	}
	root := request.Push.Root
	conflictCount := effects.ConflictCount(ctx, root)
	if unmerged := effects.UnmergedPaths(ctx, root); len(unmerged) > 0 {
		return FinalizeResult{ConflictFiles: unmerged, Missing: "conflict_resolution_complete", DirectError: true},
			fmt.Errorf("execution sync-base finalize is blocked by unresolved paths: %s", strings.Join(unmerged, ", "))
	}
	if code, out := effects.CheckStaged(ctx, root); code != 0 {
		return FinalizeResult{Missing: "conflict_markers_absent", DirectError: true},
			fmt.Errorf("conflict markers remain in the staged merge result: %s", strings.TrimSpace(out))
	}
	if code, out := effects.Commit(ctx, root); code != 0 {
		return FinalizeResult{FailedStep: "merge_commit"}, fmt.Errorf("git commit --no-edit: %s", strings.TrimSpace(out))
	}
	code, head := effects.Head(ctx, root)
	if code != 0 || strings.TrimSpace(head) == "" {
		return FinalizeResult{FailedStep: "head"}, fmt.Errorf("git rev-parse HEAD: %s", strings.TrimSpace(head))
	}
	result := FinalizeResult{MergeCommit: strings.TrimSpace(head), Merged: true}
	request.Push.MergeCommit = result.MergeCommit
	request.Push.ConflictFiles = conflictCount
	request.Push.At = effects.Now()
	push, err := PushAndRecord(ctx, request.Push, effects)
	result.Pushed, result.PushRetryRequired, result.FailedStep = push.Pushed, push.PushRetryRequired, push.FailedStep
	return result, err
}
