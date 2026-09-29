package issueopsapp

import (
	commitadapter "issueops/internal/adapter/commitsuggest"
	lintadapter "issueops/internal/adapter/lintdiagnose"
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
)

func newCommitService(defaultRoot string) commitapp.Service {
	return commitapp.Service{Effects: commitadapter.Effects{Normalize: newRepoRootResolver(defaultRoot)}}
}
func newLintService(defaultRoot string) lintapp.Service {
	return lintapp.Service{Effects: lintadapter.Effects{Normalize: newRepoRootResolver(defaultRoot)}}
}
