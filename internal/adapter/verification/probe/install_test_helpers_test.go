package probe

import (
	installadapter "issueops/internal/adapter/install"
	nativeintegrationinsdeps "issueops/internal/adapter/verification/probe/nativeintegration"
	qagateinsdeps "issueops/internal/adapter/verification/probe/qagate"
)

// production wiring과 같은 install reader를 설치한다.
func init() {

	nativeintegrationinsdeps.ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
