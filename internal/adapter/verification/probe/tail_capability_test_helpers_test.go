package probe

import (
	summarytdeps "issueops/cmd/issueops/selfworkflow/summary"
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	webfetchtdeps "issueops/internal/adapter/verification/probe/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	summarytdeps.Classify = failurecauseadapter.Classify
	webfetchtdeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtdeps.RunBenchmark = webfetchadapter.RunBenchmark
}
