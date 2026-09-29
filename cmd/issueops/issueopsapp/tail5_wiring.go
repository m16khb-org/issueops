package issueopsapp

import (
	pathutiladapter "issueops/internal/adapter/issueops/pathutil"
	operationalhealthadapter "issueops/internal/adapter/operationalhealth"
)

// configureTail5는 경로 정리 구현을 설치한다.
func configureTail5() {
	operationalhealthadapter.CleanAbsPath = pathutiladapter.CleanAbsPath
}
