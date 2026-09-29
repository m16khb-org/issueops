package issueopscli

import (
	loopgatet5d "issueops/internal/adapter/issueops/loopgate"
	pathutiladapter "issueops/internal/adapter/issueops/pathutil"

	operationalhealtht5d "issueops/internal/adapter/operationalhealth"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	loopgatet5d.RepoGateMissing = testLoopRepoGateMissing
	operationalhealtht5d.CleanAbsPath = pathutiladapter.CleanAbsPath
}
