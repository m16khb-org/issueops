package projectdocs

import (
	"io/fs"
	"path"
	"strings"
	"testing"
	"time"
)

type routeEffects struct {
	existing map[string]bool
	files    map[string]string
}

func (*routeEffects) Path(root, rel string) string { return path.Join(root, rel) }
func (fake *routeEffects) Exists(path string) bool { return fake.existing[path] }
func (*routeEffects) Now() time.Time               { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }
func (fake *routeEffects) ReadDir(dir string) ([]string, error) {
	var names []string
	for name := range fake.files {
		if path.Dir(name) == dir {
			names = append(names, path.Base(name))
		}
	}
	if names == nil {
		return nil, fs.ErrNotExist
	}
	return names, nil
}
func (fake *routeEffects) ReadFile(path string) (string, error) { return fake.files[path], nil }

func TestRouteAddsExistingFamilyDetailAndMissingDocsWarning(t *testing.T) {
	fake := &routeEffects{existing: map[string]bool{"/repo/.issueops/architecture/overview.md": true}}
	result := Route("/repo", "  REFACTOR  ", fake)
	if result.Task != "refactor" || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "project docs are missing") {
		t.Fatalf("route=%+v", result)
	}
	found := false
	for _, doc := range result.Docs {
		if doc.RelPath == ".issueops/architecture/overview.md" && doc.Exists {
			found = true
		}
	}
	if !found {
		t.Fatalf("family overview missing: %+v", result.Docs)
	}
}

func TestRouteReturnsRecordsSharingTermsWithKoreanTask(t *testing.T) {
	fake := &routeEffects{
		existing: map[string]bool{"/repo/.issueops": true},
		files: map[string]string{
			"/repo/.issueops/cautions/2026-09-07-disk.md":  "# Reserve host disk space before Docker image extraction\n\n- Summary: Docker image extraction exhausted the host disk.\n",
			"/repo/.issueops/cautions/2026-09-08-other.md": "# Korean copy breaks mid-word\n\n- Summary: Dialog text wraps inside a word.\n",
			"/repo/.issueops/cautions/overview.md":         "# Docker image docker image\n",
			"/repo/.issueops/adr/2026-09-01-auth.md":       "# Auth tokens\n\n- Summary: Tokens rotate.\n",
		},
	}
	result := Route("/repo", "운영 서버에 docker compose로 재배포하고 이미지를 빌드한다", fake)
	var records []string
	for _, doc := range result.Docs {
		if strings.HasPrefix(doc.RelPath, ".issueops/cautions/") || strings.HasPrefix(doc.RelPath, ".issueops/adr/") {
			records = append(records, doc.RelPath)
		}
	}
	if len(records) != 1 || records[0] != ".issueops/cautions/2026-09-07-disk.md" || len(result.Warnings) != 0 {
		t.Fatalf("records=%v warnings=%v", records, result.Warnings)
	}
}
