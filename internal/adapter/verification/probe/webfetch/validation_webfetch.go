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

type StepResult = verifycontract.StepResult

func Validate(binary, root string, seed int64) StepResult {
	started := time.Now()
	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Fixtures: DeterministicFixtures(),
		Timeout:  time.Second,
	})
	if err != nil {
		return verifydomain.FailedStep("web fetch battery", err)
	}
	out, _ := json.Marshal(result)
	if !result.OK {
		return verifydomain.AssertionStepWithOutput("web fetch battery", time.Since(started).Milliseconds(), result.HardFailures, []string{string(out)}, []string{"internal/adapter/outbound/webfetch deterministic benchmark"}, 8*1024)
	}
	return StepResult{
		Label:      "web fetch battery",
		Command:    fmt.Sprintf("%s web-fetch benchmark --fixtures builtin --json", binary),
		OK:         true,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     strings.TrimSpace(string(out)),
	}
}
