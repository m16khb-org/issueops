package webfetch

import (
	"context"
	"encoding/json"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
	domain "issueops/internal/domain/webfetch"
)

type Benchmark struct {
	Fetch           func(context.Context, webfetchcontract.Request) (webfetchcontract.Result, error)
	Fixtures        func() []webfetchcontract.BenchmarkFixture
	RunFixture      func(context.Context, webfetchcontract.BenchmarkFixture, time.Duration) (webfetchcontract.Result, int, error)
	CheckComparator func(string) error
	RunComparator   func(context.Context, string, string) ([]byte, error)
	Now             func() time.Time
}

func (service Benchmark) Run(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error) {
	if req.Live {
		return service.runLiveBenchmark(ctx, req)
	}
	fixtures := req.Fixtures
	if len(fixtures) == 0 {
		fixtures = service.Fixtures()
	}
	timeout := domain.BenchmarkTimeout(req.Timeout, false)
	result := domain.NewBenchmarkResult(len(fixtures))

	unsafeRedirectFetches := 0
	falseStrongOK := 0
	for _, fixture := range fixtures {
		fetchResult, finalFetches, err := service.RunFixture(ctx, fixture, timeout)
		if err != nil {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+err.Error())
			result.FixtureResults = append(result.FixtureResults, webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, OK: false, Failure: err.Error()})
			continue
		}
		unsafeRedirectFetches += finalFetches
		run, falseStrong := domain.EvaluateBenchmarkFixture(fixture, fetchResult, false, 0)
		if falseStrong {
			falseStrongOK++
		}
		if !run.OK {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+run.Failure)
		}
		result.FixtureResults = append(result.FixtureResults, run)
	}
	return domain.CompleteOfflineBenchmark(result, unsafeRedirectFetches, falseStrongOK), nil
}

func (service Benchmark) runLiveBenchmark(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error) {
	result := domain.NewBenchmarkResult(len(req.Fixtures))
	if status, err := domain.ValidateLiveBenchmark(req); err != nil {
		result.LiveParityStatus = status
		return result, err
	}
	timeout := domain.BenchmarkTimeout(req.Timeout, true)

	successes := 0
	comparable := 0
	agreements := 0
	var latencies []int64
	candidateCategories := map[string]string{}
	for _, fixture := range req.Fixtures {
		if !domain.HasLiveBenchmarkURL(fixture) {
			msg := "live fixture missing url"
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+msg)
			result.FixtureResults = append(result.FixtureResults, webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, OK: false, Failure: msg})
			continue
		}
		started := service.Now()
		fetchResult, err := service.Fetch(ctx, webfetchcontract.Request{
			URL:                 fixture.URL,
			Timeout:             timeout,
			AllowPrivateNetwork: req.AllowPrivateNetwork,
		})
		latencyMS := service.Now().Sub(started).Milliseconds()
		latencies = append(latencies, latencyMS)
		result.LiveParityReport.RouteCount += len(fetchResult.AttemptedRoutes)
		run := webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, Category: fetchResult.Category, StopReason: fetchResult.StopReason, LatencyMS: latencyMS}
		if err != nil {
			run.Failure = err.Error()
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+err.Error())
			result.FixtureResults = append(result.FixtureResults, run)
			continue
		}
		if fetchResult.OK {
			successes++
		}
		candidateCategories[fixture.ID] = fetchResult.Category
		if len(fixture.Expected) > 0 {
			comparable++
			if domain.CategoryAllowed(fetchResult.Category, fixture.Expected) {
				agreements++
			}
		}
		run, falseStrong := domain.EvaluateBenchmarkFixture(fixture, fetchResult, true, latencyMS)
		if falseStrong {
			result.LiveParityReport.FalseStrongOK++
			result.FalseStrongOK++
		}
		if !run.OK {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+run.Failure)
		}
		result.FixtureResults = append(result.FixtureResults, run)
	}

	if len(req.Fixtures) > 0 {
		result.LiveParityReport.SuccessRate = domain.Percent(successes, len(req.Fixtures))
	}
	if comparable > 0 {
		result.LiveParityReport.CategoryAgreement = domain.Percent(agreements, comparable)
	}
	result.LiveParityReport.LatencyP50MS = domain.PercentileLatency(latencies, 50)
	result.LiveParityReport.LatencyP95MS = domain.PercentileLatency(latencies, 95)
	if req.CompareCommand != "" {
		service.compareBaseline(ctx, req.CompareCommand, req.Fixtures, candidateCategories, timeout, &result)
	}
	return domain.CompleteBenchmark(result, true, req.CompareCommand), nil
}

func (service Benchmark) compareBaseline(ctx context.Context, command string, fixtures []webfetchcontract.BenchmarkFixture, candidateCategories map[string]string, timeout time.Duration, result *webfetchcontract.BenchmarkResult) {
	if err := service.CheckComparator(command); err != nil {
		result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator unavailable: "+err.Error())
		return
	}
	baselineTimeout := timeout
	if baselineTimeout < 5*time.Second {
		baselineTimeout = 5 * time.Second
	}
	successes := 0
	comparable := 0
	agreements := 0
	var latencies []int64
	for _, fixture := range fixtures {
		if !domain.HasLiveBenchmarkURL(fixture) {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, baselineTimeout)
		started := service.Now()
		output, err := service.RunComparator(runCtx, command, fixture.URL)
		cancel()
		latencies = append(latencies, service.Now().Sub(started).Milliseconds())
		if err != nil {
			result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator failed for "+fixture.ID+": "+err.Error())
			continue
		}
		var payload struct {
			OK       bool   `json:"ok"`
			Category string `json:"category"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator returned invalid JSON for "+fixture.ID+": "+err.Error())
			continue
		}
		result.LiveParityReport.BaselineAvailable = true
		if payload.OK {
			successes++
		}
		candidateCategory := candidateCategories[fixture.ID]
		if payload.Category != "" && candidateCategory != "" {
			comparable++
			if payload.Category == candidateCategory {
				agreements++
			}
		}
	}
	if result.LiveParityReport.BaselineAvailable {
		result.LiveParityReport.BaselineSuccessRate = domain.Percent(successes, len(fixtures))
		result.LiveParityReport.BaselineLatencyP50MS = domain.PercentileLatency(latencies, 50)
		result.LiveParityReport.BaselineLatencyP95MS = domain.PercentileLatency(latencies, 95)
		if comparable > 0 {
			result.LiveParityReport.CategoryAgreement = domain.Percent(agreements, comparable)
		}
	}
}
