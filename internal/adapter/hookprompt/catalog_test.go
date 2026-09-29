package hookprompt

import (
	"issueops/internal/domain/projectdoc"
	"strings"
	"testing"
)

func TestRenderProjectDocCatalogUserViewFallsBackToTitle(t *testing.T) {
	view := RenderProjectDocCatalogUserView([]projectdoc.ProjectDocCatalogEntry{
		{RelPath: ".issueops/ADR.md", Title: "Decisions", Description: "Structural decisions."},
		{RelPath: ".issueops/NOTES.md", Title: "Notes only"},
	})
	if !strings.Contains(view, "• ADR.md — Structural decisions.") || !strings.Contains(view, "• NOTES.md — Notes only") {
		t.Fatalf("user view = %q", view)
	}
	if RenderProjectDocCatalogUserView(nil) != "" {
		t.Fatal("empty catalog must render nothing")
	}
}
