package operationalhealth

import (
	"path/filepath"
	"testing"
)

func TestCanonicalInventoryPathWorksWithoutGlobalInitialization(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("path resolution depends on global initialization: %v", recovered)
		}
	}()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	got := canonicalInventoryPath("  " + filepath.Join(root, "missing", "child") + "  ")
	if got != filepath.Join(resolved, "missing", "child") {
		t.Fatalf("missing descendant=%q", got)
	}
	if got := canonicalInventoryPath("  "); got != "" {
		t.Fatalf("empty path=%q", got)
	}
}
