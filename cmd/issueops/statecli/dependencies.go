// Package statecli 패키지는 `issueops state` 명령(write, read, list, prune, doctor,
// maintain)의 flag 해석과 출력을 맡는 inbound CLI adapter다.
package statecli

import (
	"context"
	"time"

	"issueops/cmd/issueops/jsonout"
	statecontract "issueops/internal/contract/state"
)

// Dependencies 타입은 state CLI가 필요로 하는 state 연산을 함수로 받는다.
//
// CLI는 inbound adapter이므로 flag 해석과 출력만 소유하고, 저장소 구현은 모른다.
// concrete outbound adapter의 조립은 composition root(`cmd/issueops/issueopsapp`)에만 둔다.
type Dependencies struct {
	Write    func(ctx context.Context, key, content string) (statecontract.Result, error)
	Read     func(key string) (statecontract.Result, error)
	List     func() (statecontract.ListResult, error)
	Prune    func(ctx context.Context, maxAge time.Duration, confirm bool) (statecontract.PruneResult, error)
	Doctor   func() (statecontract.DoctorResult, error)
	Maintain func() (statecontract.MaintainResult, error)
}

var printJSON = jsonout.Print
