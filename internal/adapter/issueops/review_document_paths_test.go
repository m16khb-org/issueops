package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

// Cycle material under .issueops/issues and .issueops/gates is not a project
// doc; AGENTS.md or any other .issueops markdown is.
func TestReviewDocumentPathsProjectDocsPresent(t *testing.T) {
	root := t.TempDir()
	write := func(rel string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := ReviewDocumentPaths{}
	if paths.ProjectDocsPresent(root) {
		t.Fatal("empty root has project docs")
	}
	write(".issueops/issues/41/plan.md")
	write(".issueops/gates/lint.md")
	if paths.ProjectDocsPresent(root) {
		t.Fatal("cycle material counted as a project doc")
	}
	write(".issueops/cautions/2026-01-01-x.md")
	if !paths.ProjectDocsPresent(root) {
		t.Fatal("nested .issueops doc not found")
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "AGENTS.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !paths.ProjectDocsPresent(other) || !paths.ProjectDocsPresent("") {
		t.Fatal("AGENTS.md or unknown root must count as present")
	}
}
