package mcpcli

import (
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	tracetdeps "issueops/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	Fetch = webfetchadapter.Fetch
	tracetdeps.Classify = failurecauseadapter.Classify
}
