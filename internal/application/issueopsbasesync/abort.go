package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

type AbortRequest struct {
	ID       string
	Root     string
	Released bool
}

type AbortResult struct {
	Aborted    bool
	FailedStep string
}

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
