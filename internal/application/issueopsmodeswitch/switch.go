package issueopsmodeswitch

import (
	"context"
	"fmt"

	"issueops/internal/port"
)

type ApplyRequest struct {
	ID                string
	Repo              string
	WorktreeRoot      string
	WorktreePresent   bool
	Branch            string
	BranchPresent     bool
	ExpectedRecordSHA string
}

// Apply preserves the external-effect order before the fenced record reset.
func Apply(ctx context.Context, request ApplyRequest, effects port.ModeSwitchEffects) error {
	if effects == nil {
		return fmt.Errorf("mode switch effects are required")
	}
	if request.WorktreePresent {
		if err := effects.RemoveWorktree(ctx, request.Repo, request.WorktreeRoot); err != nil {
			return err
		}
	}
	if request.BranchPresent {
		if err := effects.RemoveBranch(ctx, request.Repo, request.Branch); err != nil {
			return err
		}
	}
	return effects.ResetExecution(ctx, request.ID, request.ExpectedRecordSHA)
}
