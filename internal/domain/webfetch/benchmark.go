package webfetch

import (
	"fmt"
	"sort"
	"strings"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
)

func NewBenchmarkResult(count int) webfetchcontract.BenchmarkResult {
	return webfetchcontract.BenchmarkResult{FixtureCount: count, LiveParityStatus: "not evaluated", LiveParityReport: webfetchcontract.LiveParityReport{Warnings: []string{}}, DimensionScores: map[string]float64{}, SafetyPassRate: 100, LiveParityEvaluated: false}
}
func ValidateLiveBenchmark(req webfetchcontract.BenchmarkRequest) (string, error) {
	if !req.LiveOptIn {
		return "live mode requires ISSUEOPS_WEBFETCH_LIVE=1", fmt.Errorf("live benchmark requires ISSUEOPS_WEBFETCH_LIVE=1")
	}
	if len(req.Fixtures) == 0 {
		return "live mode requires URL fixtures", fmt.Errorf("live benchmark requires URL fixtures")
	}
	return "", nil
}
func BenchmarkTimeout(timeout time.Duration, live bool) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if live {
		return 10 * time.Second
	}
	return 2 * time.Second
}
func EvaluateBenchmarkFixture(fixture webfetchcontract.BenchmarkFixture, result webfetchcontract.Result, live bool, latency int64) (webfetchcontract.BenchmarkFixtureRun, bool) {
	run := webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, Category: result.Category, StopReason: result.StopReason, LatencyMS: latency}
	if live && len(fixture.Expected) == 0 {
		run.OK = result.OK || result.GridExhausted
	} else if CategoryAllowed(result.Category, fixture.Expected) {
		run.OK = true
	} else {
		run.Failure = fmt.Sprintf("category %s not in expected %v", result.Category, fixture.Expected)
	}
	if fixture.MinBodyChars > 0 && len(result.Content) < fixture.MinBodyChars {
		run.OK = false
		run.Failure = fmt.Sprintf("content length %d < %d", len(result.Content), fixture.MinBodyChars)
	}
	falseStrong := IsFalseStrongOKFixture(fixture.ID) && result.Category == webfetchcontract.CategoryStrongOK
	if falseStrong {
		run.OK = false
		run.Failure = "false strong_ok"
	}
	return run, falseStrong
}
func CompleteOfflineBenchmark(result webfetchcontract.BenchmarkResult, unsafeFetches, falseStrong int) webfetchcontract.BenchmarkResult {
	if unsafeFetches > 0 {
		result.HardFailures = append(result.HardFailures, fmt.Sprintf("unsafe redirect final target fetch count = %d", unsafeFetches))
	}
	result.FalseStrongOK = falseStrong
	if falseStrong > 0 {
		result.HardFailures = append(result.HardFailures, fmt.Sprintf("false strong_ok count = %d", falseStrong))
	}
	return CompleteBenchmark(result, false, "")
}
func CompleteBenchmark(result webfetchcontract.BenchmarkResult, live bool, comparator string) webfetchcontract.BenchmarkResult {
	result.Score = ScoreBenchmark(result)
	result.DimensionScores = map[string]float64{"safety": 25, "classification_correctness": 30, "exhaustion_semantics": 15, "citation_ready_output": 10, "efficiency": 10, "live_parity_readiness": 10}
	result.OK = result.Score >= 95 && len(result.HardFailures) == 0
	if live {
		result.LiveParityEvaluated = true
		if comparator == "" {
			result.LiveParityStatus = "evaluated without baseline comparator"
		} else if result.LiveParityReport.BaselineAvailable {
			result.LiveParityStatus = "evaluated with baseline comparator"
		} else {
			result.LiveParityStatus = "evaluated; baseline comparator unavailable"
		}
		result.OK = result.OK && result.LiveParityReport.SafetyFailures == 0 && result.LiveParityReport.FalseStrongOK == 0
	}
	return result
}
func CategoryAllowed(category string, expected []string) bool {
	for _, value := range expected {
		if category == value {
			return true
		}
	}
	return false
}

func IsFalseStrongOKFixture(id string) bool {
	switch id {
	case "auth_required", "login_wall", "paywall_shell", "waf_challenge", "empty_spa":
		return true
	default:
		return false
	}
}

func ScoreBenchmark(result webfetchcontract.BenchmarkResult) float64 {
	score := 100.0
	score -= float64(len(result.HardFailures)) * 10
	if score < 0 {
		return 0
	}
	return score
}

func Percent(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func PercentileLatency(values []int64, Percentile int) int64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]int64(nil), values...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	if Percentile <= 0 {
		return copied[0]
	}
	index := (len(copied)*Percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(copied) {
		index = len(copied)
	}
	return copied[index-1]
}

func HasLiveBenchmarkURL(fixture webfetchcontract.BenchmarkFixture) bool {
	return strings.TrimSpace(fixture.URL) != ""
}
