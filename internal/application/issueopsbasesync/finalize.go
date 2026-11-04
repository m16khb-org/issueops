package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

// FinalizeRequest carries the push parameters used after a conflict-resolved
// merge is committed.
type FinalizeRequest struct {
	Push PushRequest
}

// FinalizeResult reports the outcome of Finalize: remaining conflict files or
// the Missing precondition when blocked, the merge commit, push state, and the
// failed step. DirectError marks blocked-precondition errors that callers
// report as-is rather than as a failed step.
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

// Finalize commits a conflict-resolved merge and pushes it via PushAndRecord.
// It refuses while unmerged paths or conflict markers in the staged result
// remain, then commits, reads HEAD, and records the sync event with the
// conflict count observed before committing.
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
