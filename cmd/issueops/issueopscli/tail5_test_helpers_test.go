package issueopscli

import (
	pathutiladapter "issueops/internal/adapter/issueops/pathutil"

	operationalhealtht5d "issueops/internal/adapter/operationalhealth"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	operationalhealtht5d.CleanAbsPath = pathutiladapter.CleanAbsPath
}
