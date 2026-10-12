package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func TestGuardReadFileStagedPrefersIndexContentOverWorkingTree(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initGuardRepo(t)
	writeGuardFile(t, repo, "pkg/foo.go", "staged\n")
	if code, _, stderr := gitCmd(repo, "add", "pkg/foo.go"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	writeGuardFile(t, repo, "pkg/foo.go", "working\n")

	if got, ok := guardReadFile(repo, "pkg/foo.go", true); !ok || strings.TrimSpace(got) != "staged" {
		t.Fatalf("staged read = %q, %v; want index content", got, ok)
	}
	if got, ok := guardReadFile(repo, "pkg/foo.go", false); !ok || got != "working\n" {
		t.Fatalf("unstaged read = %q, %v; want working tree content", got, ok)
	}
}

func TestSourceResolveRootJoinsRelativePathOntoBaseDir(t *testing.T) {
	base := t.TempDir()
	if got, want := (Source{BaseDir: base}).ResolveRoot("sub"), filepath.Join(base, "sub"); got != want {
		t.Fatalf("ResolveRoot relative = %q, want %q", got, want)
	}
	abs := t.TempDir()
	if got := (Source{BaseDir: base}).ResolveRoot(abs); got != abs {
		t.Fatalf("ResolveRoot absolute = %q, want %q", got, abs)
	}
}
