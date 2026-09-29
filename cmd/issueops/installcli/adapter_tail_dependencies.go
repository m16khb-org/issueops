package installcli

import (
	installcontract "issueops/internal/contract/install"
)

// 관리 대상 명령 파일 트랜잭션이다. installcli는 채택 구현을 모르고 이 세 연산만
// 호출한다 — 소비자가 필요한 만큼만 인터페이스로 선언한다.
type ManagedCommandPathTransaction interface {
	Apply() (installcontract.ManagedCommandPathPlan, error)
	Rollback() (installcontract.ManagedCommandPathPlan, error)
	Finalize() (installcontract.ManagedCommandPathPlan, error)
}
