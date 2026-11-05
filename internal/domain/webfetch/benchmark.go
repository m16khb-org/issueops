package webfetch

import (
	"fmt"
	"slices"
	"strings"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
)

// NewBenchmarkResult returns the starting result for a benchmark over count
// fixtures: no live parity evaluated, a full safety pass rate, and empty
// score and warning collections so JSON output never carries null.
func NewBenchmarkResult(count int) webfetchcontract.BenchmarkResult {
	return webfetchcontract.BenchmarkResult{FixtureCount: count, LiveParityStatus: "not evaluated", LiveParityReport: webfetchcontract.LiveParityReport{Warnings: []string{}}, DimensionScores: map[string]float64{}, SafetyPassRate: 100, LiveParityEvaluated: false}
}

// ValidateLiveBenchmark rejects a live benchmark request that lacks the
// ISSUEOPS_WEBFETCH_LIVE opt-in or URL fixtures. On failure it returns both a
// short skip reason and the error.
func ValidateLiveBenchmark(req webfetchcontract.BenchmarkRequest) (string, error) {
	if !req.LiveOptIn {
		return "live mode requires ISSUEOPS_WEBFETCH_LIVE=1", fmt.Errorf("live benchmark requires ISSUEOPS_WEBFETCH_LIVE=1")
	}
	if len(req.Fixtures) == 0 {
		return "live mode requires URL fixtures", fmt.Errorf("live benchmark requires URL fixtures")
	}
	return "", nil
}

// BenchmarkTimeout returns timeout when positive, otherwise the default per
// fixture: 10s for live runs and 2s for offline runs.
func BenchmarkTimeout(timeout time.Duration, live bool) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if live {
		return 10 * time.Second
	}
	return 2 * time.Second
}

// EvaluateBenchmarkFixture judges one fixture result against its expectations
// and returns the run record plus whether it was a false strong_ok, that is, a
// strong_ok classification for a fixture that must never classify as one. A
// live fixture without expected categories passes when the fetch succeeded or
// the grid was exhausted; latency is in milliseconds.
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
	falseStrong := isFalseStrongOKFixture(fixture.ID) && result.Category == webfetchcontract.CategoryStrongOK
	if falseStrong {
		run.OK = false
		run.Failure = "false strong_ok"
	}
	return run, falseStrong
}

// CompleteOfflineBenchmark records unsafe-redirect fetches and false
// strong_ok counts as hard failures on result and finalizes it as an offline
// (non-live) benchmark.
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

// CompleteBenchmark computes the score and dimension weights of result and
// sets OK when the score is at least 95 with no hard failures. For a live run
// it also records the live parity status, which depends on whether a
// comparator and baseline were available, and requires zero parity safety
// failures and false strong_ok results.
func CompleteBenchmark(result webfetchcontract.BenchmarkResult, live bool, comparator string) webfetchcontract.BenchmarkResult {
	result.Score = scoreBenchmark(result)
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

// CategoryAllowed reports whether category appears in expected.
func CategoryAllowed(category string, expected []string) bool {
	return slices.Contains(expected, category)
}

func isFalseStrongOKFixture(id string) bool {
	switch id {
	case "auth_required", "login_wall", "paywall_shell", "waf_challenge", "empty_spa":
		return true
	default:
		return false
	}
}

func scoreBenchmark(result webfetchcontract.BenchmarkResult) float64 {
	score := 100.0
	score -= float64(len(result.HardFailures)) * 10
	if score < 0 {
		return 0
	}
	return score
}

// Percent returns numerator as a percentage of denominator, or 0 when the
// denominator is not positive.
func Percent(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

// PercentileLatency returns the nearest-rank Percentile (0-100) of values
// without modifying the slice; it returns 0 for no values and the minimum
// when Percentile is not positive.
func PercentileLatency(values []int64, Percentile int) int64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]int64(nil), values...)
	slices.Sort(copied)
	if Percentile <= 0 {
		return copied[0]
	}
	index := min(max((len(copied)*Percentile+99)/100, 1), len(copied))
	return copied[index-1]
}

// HasLiveBenchmarkURL reports whether fixture names a non-blank URL to fetch
// live.
func HasLiveBenchmarkURL(fixture webfetchcontract.BenchmarkFixture) bool {
	return strings.TrimSpace(fixture.URL) != ""
}
