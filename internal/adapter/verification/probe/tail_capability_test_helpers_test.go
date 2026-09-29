package probe

import (
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	webfetchtdeps "issueops/internal/adapter/verification/probe/webfetch"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	webfetchtdeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtdeps.RunBenchmark = webfetchadapter.RunBenchmark
}
