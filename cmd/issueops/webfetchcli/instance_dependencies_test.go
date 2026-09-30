package webfetchcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/webfetch"
)

func TestPreparedWebFetchCommandsKeepTheirDependencies(t *testing.T) {
	prepare := func(name string) func([]string) error {
		deps := Deps{}
		deps.Fetch = func(context.Context, model.Request) (model.Result, error) {
			return model.Result{}, errors.New(name + " fetch")
		}
		deps.DeterministicFixtures = func() []model.BenchmarkFixture { return []model.BenchmarkFixture{{ID: name}} }
		deps.RunBenchmark = func(_ context.Context, req model.BenchmarkRequest) (model.BenchmarkResult, error) {
			return model.BenchmarkResult{}, errors.New(name + " benchmark " + req.Fixtures[0].ID)
		}
		return func(args []string) error { return RunWithDeps(args, deps) }
	}
	first, second := prepare("first"), prepare("second")
	for _, tc := range []struct {
		run  func([]string) error
		name string
	}{{first, "first"}, {second, "second"}, {first, "first"}} {
		for _, kind := range []string{"fetch", "benchmark"} {
			args := []string{"fetch", "--url", "https://example.invalid"}
			want := tc.name + " fetch"
			if kind == "benchmark" {
				args = []string{"benchmark", "--fixtures", "builtin"}
				want = tc.name + " benchmark " + tc.name
			}
			if err := tc.run(args); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("prepared %s lost dependencies: got %v, want %q", kind, err, want)
			}
		}
	}
}
