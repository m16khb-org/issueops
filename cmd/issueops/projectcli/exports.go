package projectcli

import (
	bootstrapapp "issueops/internal/application/projectbootstrap"
	docsapp "issueops/internal/application/projectdocs"
)

type Dependencies struct {
	Docs      docsapp.Service
	Bootstrap bootstrapapp.Service
}

func Run(deps Dependencies, args []string) error {
	return runProject(deps, args)
}

func RunBootstrap(bootstrap bootstrapapp.Service, args []string) error {
	return runProjectBootstrap(bootstrap, args)
}

func RunDocs(docs docsapp.Service, args []string) error {
	return runProjectDocs(docs, args)
}

func RunRouteDocs(docs docsapp.Service, args []string) error {
	return runProjectRouteDocs(docs, args)
}

func RunRecord(docs docsapp.Service, args []string) error {
	return runProjectAppend(docs, args)
}

func RunCommitSuggest(args []string) error {
	return runProjectCommitSuggest(args)
}

func RunLintDiagnose(args []string) error {
	return runProjectLintDiagnose(args)
}
