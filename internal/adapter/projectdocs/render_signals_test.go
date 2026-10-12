package projectdocs

import (
	"strings"
	"testing"

	projectdoccontract "issueops/internal/contract/projectdoc"
	projectdoc "issueops/internal/domain/projectdoc"
)

func anyRenderedDocContains(docs map[string]string, want string) bool {
	for _, content := range docs {
		if strings.Contains(content, want) {
			return true
		}
	}
	return false
}

func TestRenderProjectDocsReflectsOptionalSignals(t *testing.T) {
	const (
		ciCaution    = "CI workflows exist; compare local verification with CI behavior."
		noConvention = "Few conventions were auto-detected."
		buildHint    = "Candidate build commands are listed in TESTING.md"
	)
	root := t.TempDir()

	empty := RenderProjectDocs(root, projectdoc.ProjectSignals{})
	for _, unwanted := range []string{ciCaution, buildHint} {
		if anyRenderedDocContains(empty, unwanted) {
			t.Fatalf("empty signals must not render %q", unwanted)
		}
	}
	if !anyRenderedDocContains(empty, noConvention) {
		t.Fatalf("empty signals must render the no-conventions fallback")
	}

	full := RenderProjectDocs(root, projectdoc.ProjectSignals{
		GitHubWorkflows:     []string{".github/workflows/ci.yml"},
		DetectedConventions: []string{"Makefile exists: use it"},
		BuildCommands:       []projectdoccontract.EvidenceCommand{{Command: "make build"}},
	})
	for _, want := range []string{ciCaution, buildHint, "- Makefile exists: use it\n"} {
		if !anyRenderedDocContains(full, want) {
			t.Fatalf("populated signals must render %q", want)
		}
	}
	if anyRenderedDocContains(full, noConvention) {
		t.Fatalf("detected conventions must replace the fallback")
	}
}

func TestRenderDesignDocListsFrameworksOrFallsBackToProjectTypes(t *testing.T) {
	designRel := projectdoc.ProjectDocsDir + "/DESIGN.md"
	root := t.TempDir()

	withFrameworks := RenderProjectDocs(root, projectdoc.ProjectSignals{Profile: projectdoccontract.ProjectProfile{
		ProjectTypes: []string{"frontend"},
		Frameworks:   []string{"React", "Vite"},
	}})[designRel]
	if !strings.Contains(withFrameworks, "## Detected client surface\n\n- React\n- Vite\n") || strings.Contains(withFrameworks, "inferred from project types") {
		t.Fatalf("frameworks must be listed:\n%s", withFrameworks)
	}

	byType := RenderProjectDocs(root, projectdoc.ProjectSignals{Profile: projectdoccontract.ProjectProfile{
		ProjectTypes: []string{"desktop-client", "backend"},
	}})[designRel]
	if !strings.Contains(byType, "- Client surface inferred from project types: desktop-client, backend\n") || strings.Contains(byType, "No client frameworks confirmed") {
		t.Fatalf("project types must be the fallback:\n%s", byType)
	}
}
