// Package hookprompt renders the user-facing views of the project document
// catalog that host lifecycle hooks inject.
package hookprompt

import (
	"issueops/internal/domain/projectdoc"
	"strconv"
)

// RenderProjectDocCatalogOmissions returns a trailing user-view line naming the
// omitted-document counts, or "" when nothing was omitted.
func RenderProjectDocCatalogOmissions(omissions projectdoc.CatalogOmissions) string {
	if !omissions.Any() {
		return ""
	}
	return "\n⚠ 일부 문서가 목록에서 생략됨: " + omissions.Summary()
}

// RenderProjectDocCatalogUserView returns the one-line notice the user sees
// when the catalog is injected. The document list itself is model-facing only.
func RenderProjectDocCatalogUserView(repoName string, docs []projectdoc.CatalogEntry) string {
	if len(docs) == 0 {
		return ""
	}
	prefix := "📚 "
	if repoName != "" {
		prefix += repoName + " · "
	}
	return prefix + "project docs " + strconv.Itoa(len(docs)) + "개 (.issueops/)"
}
