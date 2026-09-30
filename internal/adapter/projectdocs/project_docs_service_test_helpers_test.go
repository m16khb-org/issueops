package projectdocs

import (
	"issueops/internal/adapter/repopath"
	docsapp "issueops/internal/application/projectdocs"
)

func testProjectDocsService() docsapp.Service {
	return docsapp.Service{NormalizeRoot: repopath.NormalizeRoot, Revision: RevisionFiles{}, Routing: RouteFiles{}, Appending: AppendFiles{}}
}
