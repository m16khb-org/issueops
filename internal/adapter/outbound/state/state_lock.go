package state

import (
	"context"
)

// WithKeyLock runs fn while holding the per-key lock for key in the state store at
// dir, so concurrent writers of the same key are serialized.
func WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	return NewService().WithKeyLock(ctx, dir, key, fn)
}
