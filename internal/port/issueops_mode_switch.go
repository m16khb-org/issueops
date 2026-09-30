package port

import "context"

// ModeSwitchEffects owns the Git and record mutations required after the
// switch-mode eligibility and preview fingerprint have been verified.
type ModeSwitchEffects interface {
	RemoveWorktree(context.Context, string, string) error
	RemoveBranch(context.Context, string, string) error
	ResetExecution(context.Context, string, string) error
}
