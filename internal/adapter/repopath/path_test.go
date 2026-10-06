package repopath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRoot(t *testing.T) {
	t.Run("current directory", func(t *testing.T) {
		root, err := NormalizeRoot("")
		if err != nil {
			t.Fatalf("NormalizeRoot empty: %v", err)
		}
		if root == "" {
			t.Error("expected non-empty root")
		}
	})

	t.Run("temp dir", func(t *testing.T) {
		dir := t.TempDir()
		root, err := NormalizeRoot(dir)
		if err != nil {
			t.Fatalf("NormalizeRoot: %v", err)
		}
		if root != dir {
			t.Errorf("got %q, want %q", root, dir)
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		_, err := NormalizeRoot("/nonexistent/path")
		if err == nil {
			t.Error("expected error for non-existent dir")
		}
	})

	t.Run("file not dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := NormalizeRoot(f)
		if err == nil {
			t.Error("expected error for file path")
		}
	})
}
