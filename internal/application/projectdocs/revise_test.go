package projectdocs

import (
	"testing"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

type revisionEffects struct {
	current string
	exists  bool
	writes  []string
}

func (fake *revisionEffects) Read(string) (string, bool, error) {
	return fake.current, fake.exists, nil
}
func (fake *revisionEffects) Write(_ string, content string) error {
	fake.writes = append(fake.writes, content)
	return nil
}
func (*revisionEffects) Now() time.Time { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestReviseReadsBeforeDecisionAndWritesOnlyConfirmedChanges(t *testing.T) {
	fake := &revisionEffects{current: "# Existing\n", exists: true}
	request := projectdocscontract.ProjectDocsReviseRequest{Content: "# New", Summary: "change", ExpectedSHA256: projectdocdomain.SHA256Hex(fake.current)}
	result, err := Revise(request, "/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", fake)
	if err != nil || !result.DryRun || result.Action != "update" || len(fake.writes) != 0 {
		t.Fatalf("dry-run result=%+v writes=%v err=%v", result, fake.writes, err)
	}
	request.Confirm = true
	result, err = Revise(request, "/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", fake)
	if err != nil || result.DryRun || len(fake.writes) != 1 || fake.writes[0] != "# New\n" {
		t.Fatalf("write result=%+v writes=%v err=%v", result, fake.writes, err)
	}
	request.ExpectedSHA256 = "stale"
	if _, err = Revise(request, "/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", fake); err == nil || len(fake.writes) != 1 {
		t.Fatalf("stale SHA wrote: writes=%v err=%v", fake.writes, err)
	}
}
