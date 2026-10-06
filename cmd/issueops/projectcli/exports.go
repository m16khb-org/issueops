package projectcli

import (
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	docsapp "issueops/internal/application/projectdocs"
)

type Dependencies struct {
	Commit    commitapp.Service
	Lint      lintapp.Service
	Docs      docsapp.Service
	Bootstrap bootstrapapp.Service
}

func Run(deps Dependencies, args []string) error {
	return runProject(deps, args)
}
