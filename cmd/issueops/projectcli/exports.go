package projectcli

import docsapp "issueops/internal/application/projectdocs"

func Run(docs docsapp.Service, args []string) error {
	return runProject(docs, args)
}

func RunBootstrap(args []string) error {
	return runProjectBootstrap(args)
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
