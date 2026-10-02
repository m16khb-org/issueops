package sqlstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkOpenCachedRoots(b *testing.B) {
	for _, roots := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("roots=%d", roots), func(b *testing.B) {
			parent := b.TempDir()
			var requested string
			for i := range roots {
				dir := filepath.Join(parent, fmt.Sprint(i))
				if _, err := Open(dir); err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() {
					if err := CloseRoot(dir); err != nil {
						b.Error(err)
					}
				})
				requested = dir
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := Open(requested); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestOpenPrunesRemovedRootsOnCacheHit(t *testing.T) {
	for _, requestedRemoved := range []bool{true, false} {
		t.Run(fmt.Sprintf("requested_removed=%t", requestedRemoved), func(t *testing.T) {
			parent := t.TempDir()
			removed := filepath.Join(parent, "removed")
			live := filepath.Join(parent, "live")
			old, err := Open(removed)
			if err != nil {
				t.Fatal(err)
			}
			liveDB, err := Open(live)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = CloseRoot(removed)
				_ = CloseRoot(live)
			})
			if err := old.Put("resource", "old", []byte("old")); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(removed); err != nil {
				t.Fatal(err)
			}
			requested := live
			if requestedRemoved {
				requested = removed
			}
			again, err := Open(requested)
			if err != nil {
				t.Fatal(err)
			}
			if old.data.Ping() == nil || old.span.Ping() == nil {
				t.Fatal("Open did not close both removed-root connections")
			}
			if requestedRemoved {
				if again == old {
					t.Fatal("Open reused the removed requested-root handle")
				}
				if _, found, err := again.Get("resource", "old"); err != nil || found {
					t.Fatalf("recreated store retained old row: found=%t err=%v", found, err)
				}
				assertMode(t, removed, 0o700)
			} else {
				if again != liveDB {
					t.Fatal("Open replaced the live cached handle")
				}
				handlesMu.Lock()
				_, cached := handles[removed]
				handlesMu.Unlock()
				if cached {
					t.Fatal("cache hit retained another removed-root handle")
				}
			}
		})
	}
}

func TestRepeatedOpenKeepsHandleAndConnectionCountsStable(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WithSpan(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.Get("resource", "missing"); err != nil {
		t.Fatal(err)
	}
	handlesMu.Lock()
	handlesBefore := len(handles)
	handlesMu.Unlock()
	dataBefore := d.data.Stats().OpenConnections
	spanBefore := d.span.Stats().OpenConnections
	fdBefore, fdOK := readableFDCount()

	for range 200 {
		again, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if again != d {
			t.Fatal("Open returned a different cached handle")
		}
		if _, _, err := again.Get("resource", "missing"); err != nil {
			t.Fatal(err)
		}
	}

	handlesMu.Lock()
	handlesAfter := len(handles)
	handlesMu.Unlock()
	if handlesAfter != handlesBefore {
		t.Fatalf("handles: %d -> %d", handlesBefore, handlesAfter)
	}
	if got := d.data.Stats().OpenConnections; got != dataBefore {
		t.Fatalf("data connections: %d -> %d", dataBefore, got)
	}
	if got := d.span.Stats().OpenConnections; got != spanBefore {
		t.Fatalf("span connections: %d -> %d", spanBefore, got)
	}
	if fdAfter, ok := readableFDCount(); fdOK && ok {
		t.Logf("/dev/fd: before=%d after=%d delta=%d", fdBefore, fdAfter, fdAfter-fdBefore)
	}
}

func TestConcurrentCachedOpenAcrossRoots(t *testing.T) {
	start := make(chan struct{})
	done := make(chan error, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 2 {
		dir := t.TempDir()
		d, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = CloseRoot(dir) })
		go func() {
			<-start
			for range 20 {
				again, err := Open(dir)
				if err != nil {
					done <- err
					return
				}
				if again != d {
					done <- fmt.Errorf("cached handle changed for %s", dir)
					return
				}
				if err := again.WithSpan(ctx, func(context.Context) error {
					return again.Put("resource", "row", []byte("value"))
				}); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
	}
	close(start)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}

func TestOpenPrunesCachedHandlesForRemovedRoots(t *testing.T) {
	parent := t.TempDir()
	removed := filepath.Join(parent, "removed")
	live := filepath.Join(parent, "live")
	if _, err := Open(removed); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(live); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = CloseRoot(live)
		_ = CloseRoot(removed)
	})

	removedAbs, err := filepath.Abs(removed)
	if err != nil {
		t.Fatal(err)
	}
	handlesMu.Lock()
	_, cached := handles[removedAbs]
	handlesMu.Unlock()
	if cached {
		t.Fatal("Open retained a cached handle after its state root was removed")
	}
}

func readableFDCount() (int, bool) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0, false
	}
	return len(entries), true
}
