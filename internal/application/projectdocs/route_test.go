package projectdocs

import (
	"path"
	"strings"
	"testing"
	"time"
)

type routeEffects struct{ existing map[string]bool }

func (*routeEffects) Path(root, rel string) string { return path.Join(root, rel) }
func (fake *routeEffects) Exists(path string) bool { return fake.existing[path] }
func (*routeEffects) Now() time.Time               { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

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
