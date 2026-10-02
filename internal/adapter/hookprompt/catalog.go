package hookprompt

import (
	"issueops/internal/domain/projectdoc"
	"strings"
)

// RenderProjectDocCatalogOmissions returns a trailing user-view line naming the
// omitted-document counts, or "" when nothing was omitted.
func RenderProjectDocCatalogOmissions(omissions projectdoc.CatalogOmissions) string {
	if !omissions.Any() {
		return ""
	}
	return "\n⚠ 일부 문서가 목록에서 생략됨: " + omissions.Summary()
}

func RenderProjectDocCatalogUserView(docs []projectdoc.ProjectDocCatalogEntry) string {
	if len(docs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("📚 issueops · 이 레포 project docs (관련된 것을 읽고 작업하세요)")
	for _, doc := range docs {
		name := strings.TrimPrefix(doc.RelPath, ".issueops/")
		desc := doc.Description
		if desc == "" {
			desc = doc.Title
		}
		b.WriteString("\n• " + name)
		if desc != "" {
			b.WriteString(" — " + desc)
		}
	}
	return b.String()
}
