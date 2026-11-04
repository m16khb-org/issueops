package projectcli

import (
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	docsapp "issueops/internal/application/projectdocs"
)

// Dependencies holds the application services the project CLI delegates to;
// the composition root builds them.
type Dependencies struct {
	Commit    commitapp.Service
	Lint      lintapp.Service
	Docs      docsapp.Service
	Bootstrap bootstrapapp.Service
}
