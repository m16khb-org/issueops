package webfetch

import (
	"context"

	webfetchcontract "issueops/internal/contract/webfetch"
)

// Validator 타입은 web fetch 벤치마크를 실행하는 연산을 주입받는다.
// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
type Validator struct {
	DeterministicFixtures func() []webfetchcontract.BenchmarkFixture
	RunBenchmark          func(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error)
}
