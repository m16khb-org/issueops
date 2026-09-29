package issueopscli

import (
	remotecmdtdeps "issueops/cmd/issueops/issueopscli/remotecmd"
	provideradapter "issueops/internal/adapter/provider"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	Resolve = provideradapter.Resolve
	remotecmdtdeps.Resolve = provideradapter.Resolve
}
