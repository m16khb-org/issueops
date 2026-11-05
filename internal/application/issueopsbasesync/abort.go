// Package issueopsbasesync is the application layer of execution sync-base:
// it merges the base branch into a work branch, pushes it, finalizes a
// conflict-resolved merge, or aborts one, driving git through port effects and
// recording the durable sync-base event.
package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

// AbortRequest names the lifecycle record ID and worktree Root whose paused
// merge is aborted. Released reports whether a conflict resolution was recorded
// for it and must be cleared.
type AbortRequest struct {
	ID       string
	Root     string
	Released bool
}

// AbortResult reports whether the merge was aborted and, on failure, which step
// (merge_abort or clear_resolution) failed.
type AbortResult struct {
	Aborted    bool
	FailedStep string
}

// Abort runs git merge --abort in the request's worktree and, for a released
// lifecycle, clears its recorded conflict resolution. It fails when effects is
// nil or either step fails; the result names the failed step.
func Abort(ctx context.Context, request AbortRequest, effects port.BaseSyncAbortEffects) (AbortResult, error) {
	if effects == nil {
		return AbortResult{}, fmt.Errorf("base sync abort effects are required")
	}
	if code, out := effects.AbortMerge(ctx, request.Root); code != 0 {
		return AbortResult{FailedStep: "merge_abort"}, fmt.Errorf("git merge --abort: %s", strings.TrimSpace(out))
	}
	if request.Released {
		if err := effects.ClearResolution(ctx, request.ID); err != nil {
			return AbortResult{FailedStep: "clear_resolution"}, err
		}
	}
	return AbortResult{Aborted: true}, nil
}
