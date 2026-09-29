package issueopsapp

import (
	"issueops/cmd/issueops/basiccli"
	guardadapter "issueops/internal/adapter/guard"
	guardcontract "issueops/internal/contract/guard"
)

// configureTail8은 guard 차단 오류 생성을 설치한다.
func configureTail8() {
	basiccli.NewGuardBlockedError = func(findings []guardcontract.GuardFinding) error {
		return guardadapter.GuardBlockedError{Findings: findings}
	}
}
