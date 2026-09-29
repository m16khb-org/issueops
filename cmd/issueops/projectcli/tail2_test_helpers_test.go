package projectcli

import (
	commitadapter "issueops/internal/adapter/commitsuggest"
	lintadapter "issueops/internal/adapter/lintdiagnose"
	"issueops/internal/adapter/repopath"
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
)

func testCommitService() commitapp.Service {
	return commitapp.Service{Effects: commitadapter.Effects{Normalize: repopath.NormalizeRoot}}
}
func testLintService() lintapp.Service {
	return lintapp.Service{Effects: lintadapter.Effects{Normalize: repopath.NormalizeRoot}}
}
