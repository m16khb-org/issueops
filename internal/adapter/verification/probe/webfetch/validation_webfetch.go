// Package webfetch 패키지는 self-verify의 web fetch 검증 probe다.
// 결정적 fixture 벤치마크를 실행해 web fetch 동작을 검사한다.
package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	webfetchcontract "issueops/internal/contract/webfetch"
	verifydomain "issueops/internal/domain/selfverify"
)

// Validate 메서드는 내장 결정적 fixture로 web fetch 벤치마크를 실행해 StepResult로 반환한다.
// 벤치마크가 실패하거나 하드 실패가 있으면 단계도 실패한다. seed는 쓰지 않는다.
func (v Validator) Validate(binary, root string, seed int64) verifycontract.StepResult {
	started := time.Now()
	result, err := v.RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Fixtures: v.DeterministicFixtures(),
		Timeout:  time.Second,
	})
	if err != nil {
		return verifydomain.FailedStep("web fetch battery", err)
	}
	out, _ := json.Marshal(result)
	if !result.OK {
		return verifydomain.AssertionStepWithOutput("web fetch battery", time.Since(started).Milliseconds(), result.HardFailures, []string{string(out)}, []string{"internal/adapter/outbound/webfetch deterministic benchmark"}, 8*1024)
	}
	return verifycontract.StepResult{
		Label:      "web fetch battery",
		Command:    fmt.Sprintf("%s web-fetch benchmark --fixtures builtin --json", binary),
		OK:         true,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     strings.TrimSpace(string(out)),
	}
}
