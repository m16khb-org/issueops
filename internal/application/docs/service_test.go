package docs

import (
	docscontract "issueops/internal/contract/docs"
	"reflect"
	"testing"
	"time"
)

type observerFake struct {
	events       []string
	gitAvailable bool
}

func (f *observerFake) Candidates(root string, roots []string) []docscontract.Candidate {
	f.events = append(f.events, "candidates")
	if root != "/root" || !reflect.DeepEqual(roots, []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", ".issueops", "skills/self-verify", "skills/self-augment"}) {
		panic("wrong discovery roots")
	}
	return []docscontract.Candidate{{Path: "/root/CLAUDE.md", RelPath: "CLAUDE.md"}, {Path: "/root/AGENTS.md", RelPath: "AGENTS.md"}}
}
func (f *observerFake) TrackedPaths(string) (map[string]bool, bool) {
	f.events = append(f.events, "git")
	return map[string]bool{"AGENTS.md": true, "CLAUDE.md": true}, f.gitAvailable
}
func (f *observerFake) ModuleDirectories(string) []string {
	f.events = append(f.events, "manifest")
	return nil
}
func (f *observerFake) ReadDocument(root, path string) (docscontract.DocIndexInfo, bool) {
	f.events = append(f.events, "read:"+path)
	if path == "/root/CLAUDE.md" {
		return docscontract.DocIndexInfo{}, false
	}
	return docscontract.DocIndexInfo{Path: path, RelPath: "AGENTS.md", Title: "Agents", Bytes: 9, Headings: []string{"Agents"}}, true
}
func TestIndexObservesInOrderAndSkipsDisappearedFiles(t *testing.T) {
	for _, available := range []bool{true, false} {
		f := &observerFake{gitAvailable: available}
		s := Service{Observer: f, Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}
		got := s.Index("/root", "v")
		wantEvents := []string{"candidates", "git"}
		if available {
			wantEvents = append(wantEvents, "manifest")
		}
		wantEvents = append(wantEvents, "read:/root/AGENTS.md", "read:/root/CLAUDE.md")
		if !reflect.DeepEqual(f.events, wantEvents) {
			t.Fatalf("events %v want %v", f.events, wantEvents)
		}
		if !got.OK || got.Version != "v" || got.IssueOpsRoot != "/root" || got.GeneratedAt != "2026-01-02T03:04:05Z" || len(got.Docs) != 1 || got.Docs[0].Title != "Agents" {
			t.Fatalf("index %+v", got)
		}
	}
}
