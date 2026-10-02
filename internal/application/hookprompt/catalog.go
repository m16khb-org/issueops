package hookprompt

import (
	hookcontract "issueops/internal/contract/hookprompt"
	"issueops/internal/domain/projectdoc"
)

// CatalogService reads the project documents and prepares both host views.
// Renderers own presentation; discovering documents is the only I/O here.
//
// DiscoverReport, when set, replaces Discover and also reports what discovery
// omitted; the Format*Omission renderers append that to each view only when
// something was omitted, so a catalog without omissions renders unchanged.
type CatalogService struct {
	Discover              func(string) []projectdoc.ProjectDocCatalogEntry
	DiscoverReport        func(string) ([]projectdoc.ProjectDocCatalogEntry, projectdoc.CatalogOmissions, projectdoc.CatalogStats)
	FormatCompact         func([]projectdoc.ProjectDocCatalogEntry) string
	FormatUserView        func([]projectdoc.ProjectDocCatalogEntry) string
	FormatCompactOmission func(projectdoc.CatalogOmissions) string
	FormatUserOmission    func(projectdoc.CatalogOmissions) string
}

func (s CatalogService) Build(repo string) hookcontract.ProjectDocCatalogContext {
	var docs []projectdoc.ProjectDocCatalogEntry
	var omissions projectdoc.CatalogOmissions
	if s.DiscoverReport != nil {
		docs, omissions, _ = s.DiscoverReport(repo)
	} else {
		docs = s.Discover(repo)
	}
	var omitted *projectdoc.CatalogOmissions
	if omissions.Any() {
		omitted = &omissions
	}
	if len(docs) == 0 {
		return hookcontract.ProjectDocCatalogContext{Omitted: omitted}
	}
	compact, userView := s.FormatCompact(docs), s.FormatUserView(docs)
	if omitted != nil {
		if s.FormatCompactOmission != nil {
			compact += s.FormatCompactOmission(omissions)
		}
		if s.FormatUserOmission != nil {
			userView += s.FormatUserOmission(omissions)
		}
	}
	return hookcontract.ProjectDocCatalogContext{
		ShouldInject: true, ProjectDocs: docs,
		Compact: compact, UserView: userView, Omitted: omitted,
	}
}
