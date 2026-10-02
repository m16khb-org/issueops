package hookprompt_test

import (
	"encoding/json"
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

func omissionService(docs []projectdoc.ProjectDocCatalogEntry, omissions projectdoc.CatalogOmissions) app.CatalogService {
	return app.CatalogService{
		DiscoverReport: func(string) ([]projectdoc.ProjectDocCatalogEntry, projectdoc.CatalogOmissions, projectdoc.CatalogStats) {
			return docs, omissions, projectdoc.CatalogStats{}
		},
		FormatCompact:         reader.FormatProjectDocCatalog,
		FormatUserView:        renderer.RenderProjectDocCatalogUserView,
		FormatCompactOmission: reader.FormatProjectDocCatalogOmissions,
		FormatUserOmission:    renderer.RenderProjectDocCatalogOmissions,
	}
}

func TestBuildWithoutOmissionsIsByteIdenticalToDiscoverPath(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n")
	base := app.CatalogService{Discover: reader.DiscoverProjectDocs, FormatCompact: reader.FormatProjectDocCatalog, FormatUserView: renderer.RenderProjectDocCatalogUserView}
	report := omissionService(nil, projectdoc.CatalogOmissions{})
	report.DiscoverReport = reader.DiscoverProjectDocsReport

	want, got := base.Build(repo), report.Build(repo)
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) || got.Omitted != nil {
		t.Fatalf("no-omission output changed:\nwant %s\n got %s", wantJSON, gotJSON)
	}
}

func TestBuildReportsExactOmissionCounts(t *testing.T) {
	docs := []projectdoc.ProjectDocCatalogEntry{{RelPath: ".issueops/ADR.md", Description: "Decisions."}}
	omissions := projectdoc.CatalogOmissions{Oversize: 2, Unreadable: 1, OverCap: 3, HeaderTruncated: 4, ScanTruncated: true}

	got := omissionService(docs, omissions).Build("repo")
	if got.Omitted == nil || *got.Omitted != omissions {
		t.Fatalf("omitted = %+v, want %+v", got.Omitted, omissions)
	}
	counts := "oversize=2 unreadable=1 over_cap=3 header_truncated=4 scan_truncated"
	if !strings.HasSuffix(got.Compact, "; omitted: "+counts) || !strings.Contains(got.UserView, counts) {
		t.Fatalf("omission counts not visible:\ncompact=%q\nuser=%q", got.Compact, got.UserView)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Omitted map[string]any `json:"omitted"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"oversize": 2.0, "unreadable": 1.0, "over_cap": 3.0, "header_truncated": 4.0, "scan_truncated": true}
	if len(decoded.Omitted) != len(want) {
		t.Fatalf("omitted json = %v", decoded.Omitted)
	}
	for key, value := range want {
		if decoded.Omitted[key] != value {
			t.Fatalf("omitted json %s = %v, want %v", key, decoded.Omitted[key], value)
		}
	}
}

func TestBuildWithOnlyOmissionsStaysNonInjectingButVisible(t *testing.T) {
	got := omissionService(nil, projectdoc.CatalogOmissions{Oversize: 1}).Build("repo")
	if got.ShouldInject || got.ProjectDocs != nil || got.Compact != "" || got.UserView != "" {
		t.Fatalf("nothing to inject: %+v", got)
	}
	if got.Omitted == nil || got.Omitted.Oversize != 1 {
		t.Fatalf("omission must stay visible in --json: %+v", got.Omitted)
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
