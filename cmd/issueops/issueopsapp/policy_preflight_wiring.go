package issueopsapp

import (
	issueopsdeps "issueops/internal/adapter/issueops"
	preflightadapter "issueops/internal/adapter/preflight"
)

// configurePolicyAndGitObservers는 명령 정책 평가·실행과 git 관측을 설치한다.
//
// 두 기능 모두 프로세스를 띄운다. 어떤 실행기를 쓸지는 composition root의
// 결정이고, 소비자는 요청과 결과 형식만 안다.
func configurePolicyAndGitObservers() {
	issueopsdeps.GitCmd = preflightadapter.GitCmd
	issueopsdeps.GitCmdRaw = preflightadapter.GitCmdRaw
}
