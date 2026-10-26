package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/processlease"
)

// A short-lived holder (a concurrent preview) is waited out; a holder that
// stays refuses with ErrBusy and names what holds the lock.
func TestCleanupLifetimeLockWaitsOutShortHoldersOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lock := CleanupLifetimeLock{StateRoot: root}
	const id = "io-0123456789ab"

	first, err := lock.Acquire(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = first.Close()
	}()
	second, err := lock.Acquire(context.Background(), id)
	if err != nil {
		t.Fatalf("a holder released within the wait must not refuse: %v", err)
	}
	defer func() { _ = second.Close() }()

	_, err = lock.Acquire(context.Background(), id)
	if !errors.Is(err, processlease.ErrBusy) || !strings.Contains(err.Error(), "another cleanup command for "+id) {
		t.Fatalf("a holder that stays must refuse with the holder named: %v", err)
	}
}
