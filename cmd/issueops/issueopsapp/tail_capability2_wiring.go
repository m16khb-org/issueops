package issueopsapp

import (
	mcpclit2deps "issueops/cmd/issueops/mcpcli"
	projectclit2deps "issueops/cmd/issueops/projectcli"
	commitsuggestadapter "issueops/internal/adapter/commitsuggest"
	lintdiagnoseadapter "issueops/internal/adapter/lintdiagnose"
)

// configureTailCapabilities2는 commit 제안과 lint 진단을
// 설치한다. 모두 저장소를 읽거나 외부 명령을 부른다.
func configureTailCapabilities2() {
	mcpclit2deps.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	mcpclit2deps.SuggestCommit = commitsuggestadapter.SuggestCommit
	projectclit2deps.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	projectclit2deps.SuggestCommit = commitsuggestadapter.SuggestCommit
}
