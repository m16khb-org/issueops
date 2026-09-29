package issueopsapp

import (
	installadapter "issueops/internal/adapter/install"
	nativeintegrationinstalldeps "issueops/internal/adapter/verification/probe/nativeintegration"
)

// configureInstallReaders는 native runtime 진단과 skill 목록 조회를 설치한다.
func configureInstallReaders() {

	nativeintegrationinstalldeps.ListSkillNames = installadapter.ListSkillNames
}
