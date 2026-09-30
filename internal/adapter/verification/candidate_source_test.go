package verification

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCandidateSourceReportsStatObservationWithoutReadingContent(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	if path, exists := CandidateSource(root); path != want || exists {
		t.Fatalf("missing source=%q,%v", path, exists)
	}
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("source observation only"), 0644); err != nil {
		t.Fatal(err)
	}
	if path, exists := CandidateSource(root); path != want || !exists {
		t.Fatalf("file source=%q,%v", path, exists)
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(want, 0755); err != nil {
		t.Fatal(err)
	}
	if _, exists := CandidateSource(root); !exists {
		t.Fatal("existing directory must preserve previous os.Stat semantics")
	}
}
