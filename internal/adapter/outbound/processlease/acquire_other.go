//go:build !unix

package processlease

import (
	"context"
	"errors"
)

func Acquire(ctx context.Context, directory, key string) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("execution lifetime locks are unsupported on this platform")
}

func AcquireShared(ctx context.Context, directory, key string) (*SharedLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("execution lifetime locks are unsupported on this platform")
}
