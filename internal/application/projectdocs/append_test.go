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
	if len(fake.events) != 4 || strings.Join(fake.events, ",") != "mkdir,exists,exists,write" {
		t.Fatalf("events=%v", fake.events)
	}
}
