//go:build unix

package processlease

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSharedLeasePreservesConcurrentWritersAndExcludesCleanup(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "leases")
	first, err := AcquireShared(ctx, dir, "records")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := AcquireShared(ctx, dir, "records")
	if err != nil {
		t.Fatal(err)
	}
	if blocked, err := Acquire(ctx, dir, "records"); !errors.Is(err, ErrBusy) {
		if blocked != nil {
			blocked.Close()
		}
		t.Fatalf("exclusive admitted during writers: %v", err)
	}
	if _, ok := any(first).(interface {
		Drain(context.Context) (*Lease, error)
	}); ok {
		t.Fatal("shared writer lease exposes an unsupported drainage guarantee")
	}
	if _, ok := any(first).(interface {
		Context(context.Context) context.Context
	}); ok {
		t.Fatal("shared writer lease exposes execution inheritance")
	}
	first.Close()
	second.Close()
	exclusive, err := Acquire(ctx, dir, "records")
	if err != nil {
		t.Fatal(err)
	}
	if writer, err := AcquireShared(ctx, dir, "records"); !errors.Is(err, ErrBusy) {
		if writer != nil {
			writer.Close()
		}
		t.Fatalf("writer admitted during exclusive cleanup: %v", err)
	}
	exclusive.Close()
	writer, err := AcquireShared(ctx, dir, "records")
	if err != nil {
		t.Fatalf("writer did not recover: %v", err)
	}
	writer.Close()
}

func TestSharedLeaseRefusesCancelledAcquisition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lease, err := AcquireShared(ctx, filepath.Join(t.TempDir(), "leases"), "records")
	if lease != nil {
		lease.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled shared acquisition: %v", err)
	}
}
