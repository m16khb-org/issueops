package mcpcli

import (
	"context"
	statecontract "issueops/internal/contract/state"
	"time"
)

// StateDependencies는 서버별 state application 연산이다.
type StateDependencies struct {
	Doctor   func() (statecontract.DoctorResult, error)
	List     func() (statecontract.ListResult, error)
	Maintain func() (statecontract.MaintainResult, error)
	Prune    func(ctx context.Context, maxAge time.Duration, confirm bool) (statecontract.PruneResult, error)
	Read     func(key string) (statecontract.Result, error)
	Write    func(ctx context.Context, key, content string) (statecontract.Result, error)
}
