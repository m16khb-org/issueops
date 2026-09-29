package webfetch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/webfetch"
)

func TestBenchmarkRefusesLiveBeforeAnyEffects(t *testing.T) {
	service := Benchmark{}
	for _, req := range []model.BenchmarkRequest{{Live: true}, {Live: true, LiveOptIn: true}} {
		result, err := service.Run(context.Background(), req)
		if err == nil || result.OK || result.LiveParityEvaluated {
			t.Fatalf("live precondition escaped: %+v %v", result, err)
		}
	}
}

func TestBenchmarkContinuesAfterReplayFailureAndAppliesDomainRules(t *testing.T) {
	var called []string
	fixtures := []model.BenchmarkFixture{{ID: "broken"}, {ID: "login_wall", Expected: []string{model.CategoryStrongOK}}, {ID: "article", Expected: []string{model.CategoryStrongOK}}}
	service := Benchmark{Fixtures: func() []model.BenchmarkFixture { return fixtures }, RunFixture: func(_ context.Context, f model.BenchmarkFixture, timeout time.Duration) (model.Result, int, error) {
		called = append(called, f.ID)
		if timeout != 2*time.Second {
			t.Fatalf("default timeout=%v", timeout)
		}
		if f.ID == "broken" {
			return model.Result{}, 0, errors.New("replay failed")
		}
		return model.Result{OK: true, Category: model.CategoryStrongOK}, 0, nil
	}}
	result, err := service.Run(context.Background(), model.BenchmarkRequest{})
	if err != nil || result.OK || result.Score != 70 || result.FalseStrongOK != 1 || len(result.FixtureResults) != 3 || !result.FixtureResults[2].OK || !reflect.DeepEqual(called, []string{"broken", "login_wall", "article"}) {
		t.Fatalf("replay flow changed: %+v %v calls=%v", result, err, called)
	}
	if result.HardFailures[0] != "broken: replay failed" || result.HardFailures[1] != "login_wall: false strong_ok" {
		t.Fatalf("failure order changed: %v", result.HardFailures)
	}
}

func TestBenchmarkBaselinePreservesPartialFailureAndMetricDenominators(t *testing.T) {
	var effects []string
	now := time.Unix(100, 0)
	service := Benchmark{
		Now: func() time.Time { now = now.Add(time.Millisecond); return now },
		Fetch: func(_ context.Context, req model.Request) (model.Result, error) {
			effects = append(effects, "fetch:"+req.URL)
			return model.Result{OK: true, Category: model.CategoryStrongOK}, nil
		},
		CheckComparator: func(string) error { effects = append(effects, "check"); return nil },
		RunComparator: func(ctx context.Context, _ string, url string) ([]byte, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < 4*time.Second {
				t.Fatal("comparator minimum timeout lost")
			}
			effects = append(effects, "compare:"+url)
			switch url {
			case "https://first.invalid":
				return []byte(`{"ok":true,"category":"strong_ok"}`), nil
			case "https://second.invalid":
				return []byte(`{`), nil
			default:
				return nil, errors.New("runner refused")
			}
		},
	}
	req := model.BenchmarkRequest{Live: true, LiveOptIn: true, CompareCommand: "compare", Timeout: time.Second, Fixtures: []model.BenchmarkFixture{{ID: "first", URL: "https://first.invalid"}, {ID: "missing"}, {ID: "second", URL: "https://second.invalid"}, {ID: "third", URL: "https://third.invalid"}}}
	result, err := service.Run(context.Background(), req)
	if err != nil || result.OK || !result.LiveParityReport.BaselineAvailable || result.LiveParityReport.BaselineSuccessRate != 25 || result.LiveParityReport.SuccessRate != 75 || result.LiveParityReport.CategoryAgreement != 100 {
		t.Fatalf("partial baseline flow changed: %+v %v", result, err)
	}
	if len(result.LiveParityReport.Warnings) != 2 || !strings.Contains(result.LiveParityReport.Warnings[0], "invalid JSON for second") || !strings.Contains(result.LiveParityReport.Warnings[1], "failed for third") {
		t.Fatalf("warning ordering changed: %+v", result)
	}
	want := []string{"fetch:https://first.invalid", "fetch:https://second.invalid", "fetch:https://third.invalid", "check", "compare:https://first.invalid", "compare:https://second.invalid", "compare:https://third.invalid"}
	if !reflect.DeepEqual(effects, want) {
		t.Fatalf("effect order=%v", effects)
	}
}
