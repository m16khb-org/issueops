package issueopscli

import (
	remotecmdtdeps "issueops/cmd/issueops/issueopscli/remotecmd"
	mcpclitdeps "issueops/cmd/issueops/mcpcli"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	provideradapter "issueops/internal/adapter/provider"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	Resolve = provideradapter.Resolve
	mcpclitdeps.Fetch = webfetchadapter.Fetch
	remotecmdtdeps.Resolve = provideradapter.Resolve
}
