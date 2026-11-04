// Package state defines the DTOs, errors and timestamp parsing shared by the agent state checkpoint store and its callers.
package state

import "errors"

// ErrInvalidState is the sentinel returned (wrapped or directly) when a state record or request fails validation.
var ErrInvalidState = errors.New("invalid state")

// Invalid returns ErrInvalidState and discards the detail argument, so callers can match with errors.Is without leaking record contents.
func Invalid(_ string) error { return ErrInvalidState }
