package hookprompt

import (
	"issueops/internal/domain/projectdoc"
	"strings"
	"testing"
)

func TestRenderProjectDocCatalogOmissions(t *testing.T) {
	if got := RenderProjectDocCatalogOmissions(projectdoc.CatalogOmissions{}); got != "" {
		t.Fatalf("no omissions must render nothing, got %q", got)
	}
	got := RenderProjectDocCatalogOmissions(projectdoc.CatalogOmissions{Oversize: 1, OverCap: 2, ScanTruncated: true})
	if !strings.HasPrefix(got, "\n") || !strings.Contains(got, "oversize=1 over_cap=2 scan_truncated") {
		t.Fatalf("omissions line = %q", got)
	}
}

func TestRenderProjectDocCatalogUserViewIsOneLineNamingTheRepository(t *testing.T) {
	docs := []projectdoc.ProjectDocCatalogEntry{
		{RelPath: ".issueops/ADR.md", Title: "Decisions", Description: "Structural decisions."},
		{RelPath: ".issueops/NOTES.md", Title: "Notes only"},
	}
	if got := RenderProjectDocCatalogUserView("issueops", docs); got != "📚 issueops · project docs 2개 (.issueops/)" {
		t.Fatalf("user view = %q", got)
	}
	if got := RenderProjectDocCatalogUserView("", docs); got != "📚 project docs 2개 (.issueops/)" {
		t.Fatalf("user view without a repository name = %q", got)
	}
	if RenderProjectDocCatalogUserView("issueops", nil) != "" {
		t.Fatal("empty catalog must render nothing")
	}
}
