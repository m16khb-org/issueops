package projectcli

import (
	docsfiles "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	docsapp "issueops/internal/application/projectdocs"
)

func testProjectDocsService() docsapp.Service {
	return docsapp.Service{NormalizeRoot: repopath.NormalizeRoot, Revision: docsfiles.RevisionFiles{}, Routing: docsfiles.RouteFiles{}, Appending: docsfiles.AppendFiles{}}
}
