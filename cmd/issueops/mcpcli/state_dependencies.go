package mcpcli

import (
	statecontract "issueops/internal/contract/state"
	"time"
)

// StateDependencies는 서버별 state application 연산이다.
type StateDependencies struct {
	Doctor   func() (statecontract.StateDoctorResult, error)
	List     func() (statecontract.StateListResult, error)
	Maintain func() (statecontract.StateMaintainResult, error)
	Prune    func(maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error)
	Read     func(key string) (statecontract.StateResult, error)
	Write    func(key, content string) (statecontract.StateResult, error)
}
