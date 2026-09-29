package issueopsapp

import (
	failurecauseadapter "issueops/internal/adapter/failurecause"
	toolconformancetaildeps "issueops/internal/adapter/toolconformance"
)

// configureTailCapabilities는 tool conformance의 실패 원인 분류기를 연결한다.
func configureTailCapabilities() {
	toolconformancetaildeps.ClassifyFailureCause = failurecauseadapter.Classify
}
