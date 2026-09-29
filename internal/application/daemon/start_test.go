package daemon

import (
	"context"
	"errors"
	contract "issueops/internal/contract/daemon"
	"testing"
	"time"
)

func TestWaiterCancellationPreservesLastObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checks, sleeps := 0, 0
	observed := contract.Status{OK: true, Code: contract.StatusStopped, Message: "observed before cancellation"}
	waiter := Waiter{
		Now:         func() time.Time { return time.Unix(1, 0) },
		CheckStatus: func() contract.Status { checks++; return observed },
		SleepContext: func(got context.Context, d time.Duration) error {
			sleeps++
			if got != ctx || d != 50*time.Millisecond {
				t.Fatalf("sleep context/duration: %v %v", got, d)
			}
			cancel()
			return ctx.Err()
		},
	}
	got, err := waiter.Run(ctx, contract.Paths{Dir: "fallback"}, time.Second)
	if !errors.Is(err, context.Canceled) || got != observed || checks != 1 || sleeps != 1 {
		t.Fatalf("wait: %+v %v checks=%d sleeps=%d", got, err, checks, sleeps)
	}
	got, err = waiter.Run(ctx, contract.Paths{Dir: "fallback"}, time.Second)
	if !errors.Is(err, context.Canceled) || got != (contract.Status{}) || checks != 1 || sleeps != 1 {
		t.Fatalf("already canceled wait performed work: %+v %v checks=%d sleeps=%d", got, err, checks, sleeps)
	}
}
