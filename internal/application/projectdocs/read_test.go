package projectdocs

import (
	"testing"
	"time"

	projectdocdomain "issueops/internal/domain/projectdoc"
)

type readEffects struct {
	content string
	exists  bool
}

func (fake readEffects) Read(string) (string, bool, error) { return fake.content, fake.exists, nil }
func (readEffects) Now() time.Time                         { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestReadDistinguishesMissingAndEmptyDocuments(t *testing.T) {
	missing, err := Read("/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", readEffects{})
	if err != nil || missing.Exists || missing.SHA256 != "" || len(missing.Warnings) != 1 {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
	empty, err := Read("/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", readEffects{exists: true})
	if err != nil || !empty.Exists || empty.SHA256 != projectdocdomain.SHA256Hex("") || len(empty.Warnings) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}
