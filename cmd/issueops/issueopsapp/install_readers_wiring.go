package issueopsapp

import (
	augmentplaninstalldeps "issueops/cmd/issueops/selfworkflow/augmentplan"
	installadapter "issueops/internal/adapter/install"
	nativeintegrationinstalldeps "issueops/internal/adapter/verification/probe/nativeintegration"
	qagateinstalldeps "issueops/internal/adapter/verification/probe/qagate"
)

// configureInstallReaders는 native runtime 진단과 skill 목록 조회를 설치한다.
func configureInstallReaders() {
	augmentplaninstalldeps.ListSkillNames = installadapter.ListSkillNames
	nativeintegrationinstalldeps.ListSkillNames = installadapter.ListSkillNames
	qagateinstalldeps.ListSkillNames = installadapter.ListSkillNames
}
