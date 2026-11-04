package issueops

import (
	"context"
	"time"
)

// SleepWithContext waits for d and returns nil, or returns ctx.Err() as soon as ctx ends.
func SleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
