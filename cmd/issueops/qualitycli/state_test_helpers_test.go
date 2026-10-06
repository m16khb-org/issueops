package qualitycli

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"

	"testing"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {

}

// configureTestStateStore는 SNR baseline 테스트가 쓰는 주입 헬퍼다.
func configureTestStateStore(t *testing.T) {
	t.Helper()
	deps := hostDeps
	deps.StateRead = statestore.NewService().Read
	deps.StateWrite = func(key, content string) (statecontract.StateResult, error) {
		return statestore.NewService().Write(context.Background(), key, content)
	}
	Configure(deps)
	t.Cleanup(Reset)
}
