package state

import (
	"context"
)

func WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	return NewService().WithKeyLock(ctx, dir, key, fn)
}
