package probe

import (
	augmentplaninsdeps "issueops/cmd/issueops/selfworkflow/augmentplan"
	installadapter "issueops/internal/adapter/install"
	nativeintegrationinsdeps "issueops/internal/adapter/verification/probe/nativeintegration"
	qagateinsdeps "issueops/internal/adapter/verification/probe/qagate"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	augmentplaninsdeps.ListSkillNames = installadapter.ListSkillNames
	nativeintegrationinsdeps.ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
