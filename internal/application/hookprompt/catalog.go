package hookprompt

import (
	hookcontract "issueops/internal/contract/hookprompt"
	"issueops/internal/domain/projectdoc"
)

// CatalogService reads the project documents and prepares both host views.
// Renderers own presentation; discovering documents is the only I/O here.
type CatalogService struct {
	Discover       func(string) []projectdoc.ProjectDocCatalogEntry
	FormatCompact  func([]projectdoc.ProjectDocCatalogEntry) string
	FormatUserView func([]projectdoc.ProjectDocCatalogEntry) string
}

func (s CatalogService) Build(repo string) hookcontract.ProjectDocCatalogContext {
	docs := s.Discover(repo)
	if len(docs) == 0 {
		return hookcontract.ProjectDocCatalogContext{}
	}
	return hookcontract.ProjectDocCatalogContext{
		ShouldInject: true, ProjectDocs: docs,
		Compact: s.FormatCompact(docs), UserView: s.FormatUserView(docs),
	}
}
