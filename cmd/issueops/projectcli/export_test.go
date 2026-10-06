package projectcli

import (
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
	docsapp "issueops/internal/application/projectdocs"
)

func RunRouteDocs(docs docsapp.Service, args []string) error {
	return runProjectRouteDocs(docs, args)
}

func RunRecord(docs docsapp.Service, args []string) error {
	return runProjectAppend(docs, args)
}

func RunCommitSuggest(service commitapp.Service, args []string) error {
	return runProjectCommitSuggest(service, args)
}

func RunLintDiagnose(service lintapp.Service, args []string) error {
	return runProjectLintDiagnose(service, args)
}
