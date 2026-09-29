package issueopscli

import (
	remotecmdtdeps "issueops/cmd/issueops/issueopscli/remotecmd"
	provideradapter "issueops/internal/adapter/provider"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	remotecmdtdeps.Resolve = provideradapter.Resolve
}
