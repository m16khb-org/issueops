package augmentplan

import (
	installadapter "issueops/internal/adapter/install"
	qagateinsdeps "issueops/internal/adapter/verification/probe/qagate"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
