package issueopscleanup

import "context"

// CleanupLifetime stays held by the owner and every inherited command. Drain
// transfers it to a fresh exclusive handle only after those commands have ended.
type CleanupLifetime interface {
	Context(context.Context) context.Context
	Close() error
	Drain(context.Context) (CleanupLifetime, error)
}
