package issueopsapp

import (
	issueopsclitaildeps "issueops/cmd/issueops/issueopscli"
	remotecmdtaildeps "issueops/cmd/issueops/issueopscli/remotecmd"
	mcpclitaildeps "issueops/cmd/issueops/mcpcli"
	webfetchclitaildeps "issueops/cmd/issueops/webfetchcli"
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	provideradapter "issueops/internal/adapter/provider"
	toolconformancetaildeps "issueops/internal/adapter/toolconformance"
	webfetchtaildeps "issueops/internal/adapter/verification/probe/webfetch"
)

// configureTailCapabilities는 실패 원인 분류, 정책 감사, 웹 조회, provider 해석을
// 설치한다. 모두 파일·네트워크·프로세스에 닿는 연산이다.
func configureTailCapabilities() {
	issueopsclitaildeps.Resolve = provideradapter.Resolve
	mcpclitaildeps.Fetch = webfetchadapter.Fetch
	remotecmdtaildeps.Resolve = provideradapter.Resolve
	toolconformancetaildeps.ClassifyFailureCause = failurecauseadapter.Classify
	webfetchclitaildeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchclitaildeps.Fetch = webfetchadapter.Fetch
	webfetchclitaildeps.RunBenchmark = webfetchadapter.RunBenchmark
	webfetchtaildeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtaildeps.RunBenchmark = webfetchadapter.RunBenchmark
}
