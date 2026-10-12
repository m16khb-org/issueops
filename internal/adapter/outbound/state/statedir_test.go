package state

import (
	"path/filepath"
	"testing"
)

func TestStateDirResolvesRelativeEnvToAbsolutePath(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("ISSUEOPS_STATE_DIR", "rel-state")
	got := StateDir()
	if !filepath.IsAbs(got) || filepath.Base(got) != "rel-state" {
		t.Fatalf("relative ISSUEOPS_STATE_DIR must become absolute, got %q", got)
	}
}
