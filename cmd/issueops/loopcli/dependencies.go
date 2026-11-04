// Package loopcli 패키지는 `issueops loop` 명령의 flag 해석과 출력을 맡는 inbound CLI
// adapter다. loop 실행 연산은 Dependencies로 주입받는다.
package loopcli

import (
	"context"
	loopruncontract "issueops/internal/contract/looprun"
)

// Dependencies 타입은 loop CLI가 필요로 하는 loop run 연산을 함수로 받는다.
//
// loop 상태는 harness state에 저장된다. CLI는 flag 해석과 출력만 소유하고,
// 저장소 조립은 composition root가 한다.
type Dependencies struct {
	Start         func(context.Context, loopruncontract.StartLoopRequest) (loopruncontract.LoopRun, error)
	RecordAttempt func(ctx context.Context, loopID string, req loopruncontract.RecordAttemptRequest) (loopruncontract.LoopRun, error)
	Stop          func(ctx context.Context, loopID string, success bool, reason string) (loopruncontract.LoopRun, error)
	Status        func(loopID string) (loopruncontract.StatusResult, error)
	ResolveID     func(repo, name string) (string, error)
}
