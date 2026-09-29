package webfetch

import (
	"strings"
	"testing"

	model "issueops/internal/contract/webfetch"
)

func TestBenchmarkFixtureDecisionPreservesFailurePriority(t *testing.T) {
	fixture := model.BenchmarkFixture{ID: "login_wall", Expected: []string{model.CategoryStrongOK}, MinBodyChars: 50}
	result := model.Result{OK: true, Category: model.CategoryStrongOK, Content: "short"}
	run, falseStrong := EvaluateBenchmarkFixture(fixture, result, true, 12)
	if run.OK || !falseStrong || run.Failure != "false strong_ok" || run.LatencyMS != 12 {
		t.Fatalf("unsafe accepted classification: %+v falseStrong=%v", run, falseStrong)
	}
	fixture.ID = "article"
	run, falseStrong = EvaluateBenchmarkFixture(fixture, result, false, 0)
	if run.OK || falseStrong || run.Failure != "content length 5 < 50" {
		t.Fatalf("minimum body ignored: %+v", run)
	}
	fixture.MinBodyChars = 0
	fixture.Expected = nil
	result.OK = false
	result.GridExhausted = true
	result.Category = model.CategoryBlocked
	offline, _ := EvaluateBenchmarkFixture(fixture, result, false, 0)
	live, _ := EvaluateBenchmarkFixture(fixture, result, true, 0)
	if offline.OK || !live.OK {
		t.Fatalf("offline expected category and live exhaustion contracts changed: offline=%+v live=%+v", offline, live)
	}
}

func TestBenchmarkCompletionEnforcesHardFailuresAndSafety(t *testing.T) {
	result := CompleteOfflineBenchmark(NewBenchmarkResult(1), 1, 1)
	if result.OK || result.Score != 80 || result.FalseStrongOK != 1 || len(result.HardFailures) != 2 || !strings.Contains(result.HardFailures[0], "unsafe redirect") {
		t.Fatalf("hard gates escaped: %+v", result)
	}
	result = NewBenchmarkResult(1)
	result.LiveParityReport.SafetyFailures = 1
	result = CompleteBenchmark(result, true, "comparator")
	if result.OK || !result.LiveParityEvaluated || result.LiveParityStatus != "evaluated; baseline comparator unavailable" {
		t.Fatalf("live safety gate escaped: %+v", result)
	}
	result = NewBenchmarkResult(1)
	result.LiveParityReport.BaselineAvailable = true
	result = CompleteBenchmark(result, true, "comparator")
	if !result.OK || result.LiveParityStatus != "evaluated with baseline comparator" {
		t.Fatalf("available baseline lost: %+v", result)
	}
	result.HardFailures = make([]string, 11)
	if ScoreBenchmark(result) != 0 {
		t.Fatal("score went below zero")
	}
}
