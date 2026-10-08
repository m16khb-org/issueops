package hookprompt

import (
	hookcontract "issueops/internal/contract/hookprompt"
	"issueops/internal/domain/projectdoc"
)

// CatalogService reads the project documents and prepares both host views.
// Renderers own presentation; the only I/O is discovering documents and the
// injected RepoName, which names the repository in the user view.
//
// DiscoverReport also reports what discovery omitted; the Format*Omission
// renderers append that to each view only when something was omitted, so a
// catalog without omissions renders unchanged.
type CatalogService struct {
	DiscoverReport        func(string) ([]projectdoc.ProjectDocCatalogEntry, projectdoc.CatalogOmissions, projectdoc.CatalogStats)
	FormatCompact         func([]projectdoc.ProjectDocCatalogEntry) string
	RepoName              func(string) string
	FormatUserView        func(repoName string, docs []projectdoc.ProjectDocCatalogEntry) string
	FormatCompactOmission func(projectdoc.CatalogOmissions) string
	FormatUserOmission    func(projectdoc.CatalogOmissions) string
}

func (s CatalogService) Build(repo string) hookcontract.ProjectDocCatalogContext {
	docs, omissions, _ := s.DiscoverReport(repo)
	var omitted *projectdoc.CatalogOmissions
	if omissions.Any() {
		omitted = &omissions
	}
	if len(docs) == 0 {
		return hookcontract.ProjectDocCatalogContext{Omitted: omitted}
	}
	repoName := ""
	if s.RepoName != nil {
		repoName = s.RepoName(repo)
	}
	compact, userView := s.FormatCompact(docs), s.FormatUserView(repoName, docs)
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
