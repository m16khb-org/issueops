package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

// ApplyRequest describes one sync-base run: the push parameters, the
// work-branch OID used for conflict prediction, whether the base must be
// merged at all, and whether the lifecycle is released (so conflicts are
// recorded as a pending resolution).
type ApplyRequest struct {
	Push        PushRequest
	WorkOID     string
	MergeNeeded bool
	Released    bool
}

// ApplyResult reports the outcome of Apply: conflicting files and whether the
// run paused on them, whether a merge happened or is still in progress, the
// resulting merge commit, push state, and the step that failed.
type ApplyResult struct {
	ConflictFiles     []string
	ConflictPaused    bool
	Merged            bool
	MergeInProgress   bool
	MergeCommit       string
	Pushed            bool
	PushRetryRequired bool
	FailedStep        string
}

// Apply merges the base branch when needed, then pushes and records the sync
// event via PushAndRecord. On merge conflicts it leaves the merge in progress,
// records the conflicting files as a resolution for released lifecycles, and
// returns with ConflictPaused set and no error. A failure to record the
// resolution aborts the merge again.
func Apply(ctx context.Context, request ApplyRequest, effects port.BaseSyncApplyEffects) (ApplyResult, error) {
	if effects == nil {
		return ApplyResult{}, fmt.Errorf("base sync apply effects are required")
	}
	result := ApplyResult{}
	if request.MergeNeeded {
		conflicts, err := effects.PredictConflicts(ctx, request.Push.Root, request.WorkOID, request.Push.BaseOID)
		if err != nil {
			return ApplyResult{FailedStep: "merge_tree"}, err
		}
		if code, out := effects.Merge(ctx, request.Push.Root, request.Push.BaseOID); code != 0 {
			result.MergeInProgress = true
			result.ConflictFiles = conflicts
			if len(result.ConflictFiles) == 0 {
				result.ConflictFiles = effects.UnmergedPaths(ctx, request.Push.Root)
			}
			if len(result.ConflictFiles) == 0 {
				result.FailedStep = "merge"
				return result, fmt.Errorf("git merge: %s", strings.TrimSpace(out))
			}
			if request.Released {
				if err := effects.StartResolution(ctx, request.Push.ID, result.ConflictFiles); err != nil {
					_, _ = effects.AbortMerge(ctx, request.Push.Root)
					result.MergeInProgress = false
					result.FailedStep = "record_resolution"
					return result, err
				}
			}
			result.ConflictPaused = true
			return result, nil
		}
		result.Merged = true
	}
	code, head := effects.Head(ctx, request.Push.Root)
	if code != 0 || strings.TrimSpace(head) == "" {
		result.FailedStep = "head"
		return result, fmt.Errorf("git rev-parse HEAD: %s", strings.TrimSpace(head))
	}
	result.MergeCommit = strings.TrimSpace(head)
	request.Push.MergeCommit = result.MergeCommit
	request.Push.At = effects.Now()
	push, err := PushAndRecord(ctx, request.Push, effects)
	result.Pushed, result.PushRetryRequired, result.FailedStep = push.Pushed, push.PushRetryRequired, push.FailedStep
	return result, err
}
