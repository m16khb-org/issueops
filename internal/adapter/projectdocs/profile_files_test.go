package projectdocs

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestAnalyzeProjectSignalsIncludesFilesAtMaxDepthOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "b", "c", "go.mod"), "")
	writeFile(t, filepath.Join(root, "a", "b", "c", "d", "go.mod"), "")
	got := AnalyzeProjectSignals(root).Files
	if !slices.Contains(got, "a/b/c/go.mod") || slices.Contains(got, "a/b/c/d/go.mod") {
		t.Fatalf("files=%v: four path segments are in, five are out", got)
	}
}

func TestInferredVCSProfileDependsOnGitDirectory(t *testing.T) {
	plain := t.TempDir()
	if vcs := AnalyzeProjectSignals(plain).Profile.VCS; vcs.Provider != "none" || vcs.Hosting != "local" {
		t.Fatalf("no .git: vcs=%+v", vcs)
	}
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if vcs := AnalyzeProjectSignals(repo).Profile.VCS; vcs.Provider != "git" || vcs.Hosting != "local" || vcs.RemoteName != "origin" {
		t.Fatalf(".git without remote: vcs=%+v", vcs)
	}
}
