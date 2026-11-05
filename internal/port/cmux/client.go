// Package cmux declares the port the application layer uses to drive the cmux
// terminal multiplexer for handoffs: the client, the pinned-directory abstraction
// used to fence the worktree root, and the typed mutation error.
package cmux

import (
	"context"
	"fmt"

	contract "issueops/internal/contract/cmux"
	model "issueops/internal/contract/issueops"
)

// Client is the cmux operation surface the handoff service depends on: check the
// host is usable, create a workspace, and send input to it.
type Client interface {
	Preflight(context.Context, contract.PreflightRequest) (contract.PreflightResult, error)
	CreateWorkspace(context.Context, contract.CreateRequest) (contract.CreatedWorkspace, error)
	Send(context.Context, contract.SendRequest) (contract.SendReceipt, error)
}

// Directory is a resolved filesystem directory identity. Path is its canonical
// path, SamePath compares it with another path by filesystem identity, and Pin
// opens a handle so later checks can detect a swapped directory.
type Directory interface {
	Path() string
	SamePath(string) bool
	Pin() (DirectoryPin, error)
}

// DirectoryPin is an open handle on a Directory. Matches reports whether a
// currently resolved Directory is still the same filesystem object, and Close
// releases the handle.
type DirectoryPin interface {
	Matches(Directory) bool
	Close()
}

// RootEnvironment bundles the effects needed to validate and fence a handoff
// root: reading the lifecycle record, resolving a Directory, the working
// directory, and the git top-level lookup.
type RootEnvironment struct {
	Read      func(string, string) (model.Record, error)
	Directory func(string) (Directory, error)
	Getwd     func() (string, error)
	GitTop    func(string) (string, error)
}

// MutationError reports a failed cmux mutation. Phase names the step that failed.
// Ambiguous is true when the mutation may already have taken effect, so callers
// must not blindly retry.
type MutationError struct {
	Phase     string
	Ambiguous bool
	Cause     error
}

func (err *MutationError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("cmux %s failed: %v", err.Phase, err.Cause)
}

func (err *MutationError) Unwrap() error { return err.Cause }
