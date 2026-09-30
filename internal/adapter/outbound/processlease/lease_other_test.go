//go:build !unix

package processlease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedLeasePlatformDoesNotCreateState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "leases")
	if lease, err := Acquire(context.Background(), dir, "cycle"); err == nil || lease != nil {
		t.Fatalf("unsupported acquisition succeeded: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("unsupported acquisition mutated state: %v", err)
	}
}
