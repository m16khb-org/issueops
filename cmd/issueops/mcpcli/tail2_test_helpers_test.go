package mcpcli

import (
	commitsuggestadapter "issueops/internal/adapter/commitsuggest"
	lintdiagnoseadapter "issueops/internal/adapter/lintdiagnose"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	SuggestCommit = commitsuggestadapter.SuggestCommit
}
