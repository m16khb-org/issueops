package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

type ApplyRequest struct {
	Push        PushRequest
	WorkOID     string
	MergeNeeded bool
	Released    bool
}

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
