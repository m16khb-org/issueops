package projectdocs

import (
	"path"
	"strings"
	"testing"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
)

type appendEffects struct {
	existing map[string]bool
	events   []string
	lines    map[string][]string
}

func (*appendEffects) Path(root, rel string) string { return path.Join(root, rel) }
func (fake *appendEffects) Exists(path string) bool {
	fake.events = append(fake.events, "exists")
	return fake.existing[path]
}
func (fake *appendEffects) EnsureDir(string) error {
	fake.events = append(fake.events, "mkdir")
	return nil
}
func (fake *appendEffects) Write(string, string) error {
	fake.events = append(fake.events, "write")
	return nil
}
func (fake *appendEffects) AppendLine(path, line string) error {
	fake.events = append(fake.events, "appendline")
	if fake.lines == nil {
		fake.lines = map[string][]string{}
	}
	fake.lines[path] = append(fake.lines[path], line)
	return nil
}
func (*appendEffects) Render(_ string, name, _ string, _ projectdocscontract.ProjectDocsAppendRequest, _ time.Time) string {
	return "record:" + name
}
func (*appendEffects) Now() time.Time { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestAppendSelectsUniqueRecordBeforeWrite(t *testing.T) {
	fake := &appendEffects{existing: map[string]bool{"/repo/.issueops/adr/2000-01-01-choice.md": true}}
	result, err := Append("/repo", projectdocscontract.ProjectDocsAppendRequest{Kind: "decision", Title: "Choice", Summary: "why"}, fake)
	if err != nil || result.RecordKind != "adr" || result.RelPath != ".issueops/adr/2000-01-01-choice-2.md" || result.BytesAppended == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if strings.Join(fake.events[:4], ",") != "mkdir,exists,exists,write" {
		t.Fatalf("events=%v", fake.events)
	}
}

func TestAppendLinksRecordFromExistingOverviewOnly(t *testing.T) {
	fake := &appendEffects{existing: map[string]bool{"/repo/.issueops/cautions/overview.md": true}}
	result, err := Append("/repo", projectdocscontract.ProjectDocsAppendRequest{Kind: "caution", Title: "Build [images] serially", Summary: "why"}, fake)
	if err != nil || len(result.Warnings) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := `- [Build \[images\] serially](2000-01-01-build-images-serially.md)`
	if got := fake.lines["/repo/.issueops/cautions/overview.md"]; len(got) != 1 || got[0] != want {
		t.Fatalf("overview lines=%q, want %q", got, want)
	}

	missing := &appendEffects{existing: map[string]bool{}}
	result, err = Append("/repo", projectdocscontract.ProjectDocsAppendRequest{Kind: "adr", Title: "Choice", Summary: "why"}, missing)
	if err != nil || len(missing.lines) != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], ".issueops/adr/overview.md is missing") {
		t.Fatalf("result=%+v lines=%v err=%v", result, missing.lines, err)
	}
}
