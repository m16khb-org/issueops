package hookprompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	renderer "issueops/internal/adapter/hookprompt"
	reader "issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
	"issueops/internal/domain/projectdoc"
)

func writeProjectDoc(t *testing.T, repo, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".issueops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildProjectDocCatalogContext(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	service := app.CatalogService{Discover: reader.DiscoverProjectDocs, FormatCompact: reader.FormatProjectDocCatalog, FormatUserView: renderer.RenderProjectDocCatalogUserView}
	cat := service.Build(repo)
	if !cat.ShouldInject || len(cat.ProjectDocs) != 1 {
		t.Fatalf("expected catalog context with one doc: %+v", cat)
	}
	canonical, _ := projectdoc.DocMetaDescription("ARCHITECTURE.md")
	if !strings.Contains(cat.Compact, "project docs (read what's relevant):") || !strings.Contains(cat.Compact, "ARCHITECTURE.md="+canonical) {
		t.Fatalf("compact catalog missing canonical meta: %q", cat.Compact)
	}
	if !strings.Contains(cat.UserView, "📚") || !strings.Contains(cat.UserView, "ARCHITECTURE.md") {
		t.Fatalf("user view missing catalog: %q", cat.UserView)
	}
	if got := service.Build(t.TempDir()); got.ShouldInject {
		t.Fatalf("expected no injection without docs: %+v", got)
	}
}

func TestCatalogWithoutDocsSkipsRendering(t *testing.T) {
	for _, docs := range [][]projectdoc.ProjectDocCatalogEntry{nil, {}} {
		calls := 0
		service := app.CatalogService{
			Discover: func(repo string) []projectdoc.ProjectDocCatalogEntry {
				if repo != "repo" {
					t.Fatalf("repo=%q", repo)
				}
				calls++
				return docs
			},
			FormatCompact:  func([]projectdoc.ProjectDocCatalogEntry) string { t.Fatal("empty catalog rendered"); return "" },
			FormatUserView: func([]projectdoc.ProjectDocCatalogEntry) string { t.Fatal("empty catalog rendered"); return "" },
		}
		got := service.Build("repo")
		if calls != 1 || got.ShouldInject || got.ProjectDocs != nil || got.Compact != "" || got.UserView != "" {
			t.Fatalf("empty catalog = %+v, calls=%d", got, calls)
		}
	}
}
