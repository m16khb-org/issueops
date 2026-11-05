// Package issueopsprovenance defines the port for observing the provenance
// (executable path and digest) of the running IssueOps binary.
package issueopsprovenance

import (
	"context"
)

// Receipt identifies the running executable by absolute path and SHA-256 digest.
type Receipt struct {
	ExecutablePath   string
	ExecutableSHA256 string
}

// Observer observes the provenance Receipt of the executable that is running
// the command.
type Observer interface {
	Observe(context.Context) (Receipt, error)
}
