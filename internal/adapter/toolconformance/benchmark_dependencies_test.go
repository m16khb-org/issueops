package toolconformance_test

import (
	"context"
	failurecause "issueops/internal/adapter/failurecause"
	adapter "issueops/internal/adapter/toolconformance"
	app "issueops/internal/application/toolconformance"
	failurecontract "issueops/internal/contract/failurecause"
	contract "issueops/internal/contract/toolconformance"
	"issueops/internal/port"
	"testing"
	"time"
)

// Integration fixtures compose the same concrete capabilities as the CLI root.
func runLiveBenchmark(ctx context.Context, request app.LiveBenchmarkRequest, descriptors []contract.ToolDescriptor, deps app.LiveBenchmarkDependencies) (contract.BenchmarkReport, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Token == nil {
		deps.Token = adapter.RandomToken
	}
	deps.LoadManifest = (app.FixtureService{Files: adapter.FixtureFiles{}}).LoadManifest
	deps.Classify = failurecause.Classify
	return app.RunLiveBenchmark(ctx, request, descriptors, deps)
}

func TestLiveBenchmarksKeepTheirFailureClassifier(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	makeRun := func(reason string) func() (contract.BenchmarkReport, error) {
		deps := app.LiveBenchmarkDependencies{
			Runners: map[string]port.HostProbeRunner{"codex": &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "probe_result_missing"}},
			Now:     func() time.Time { return time.Unix(1, 0) }, Token: func() string { return "token" },
			LoadManifest: (app.FixtureService{Files: adapter.FixtureFiles{}}).LoadManifest,
			Classify: func(failed bool, evidence []failurecontract.Evidence) failurecontract.Result {
				result := failurecause.Classify(failed, evidence)
				result.Reason = reason
				return result
			},
		}
		return func() (contract.BenchmarkReport, error) {
			return app.RunLiveBenchmark(context.Background(), app.LiveBenchmarkRequest{
				Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "instance",
			}, catalogDescriptors(), deps)
		}
	}
	first, second := makeRun("first-classifier"), makeRun("second-classifier")
	for _, tc := range []struct {
		run  func() (contract.BenchmarkReport, error)
		want string
	}{{first, "first-classifier"}, {second, "second-classifier"}, {first, "first-classifier"}} {
		report, err := tc.run()
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Hosts) != 1 || len(report.Hosts[0].Cases) != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		episode := report.Hosts[0].Cases[0]
		if episode.FailureCauseReason != tc.want || episode.FailureCause != failurecontract.Transport || report.OK {
			t.Fatalf("expected %s transport refusal, got %+v", tc.want, report)
		}
	}
}
