package issueopsapp

import (
	"context"
	"net/http"
	"os"
	"time"

	"issueops/cmd/issueops/webfetchcli"
	outbound "issueops/internal/adapter/outbound/webfetch"
	probe "issueops/internal/adapter/verification/probe/webfetch"
	app "issueops/internal/application/webfetch"
	model "issueops/internal/contract/webfetch"
)

func newWebFetch() func(context.Context, model.Request) (model.Result, error) {
	deps := app.Dependencies{
		HTTPClient: outbound.NewHTTPClient(http.DefaultClient),
		URLPolicy:  app.URLValidator{Resolver: outbound.NetResolver{}},
		Now:        time.Now,
		RetryAfter: outbound.ParseRetryAfter,
	}
	return func(ctx context.Context, req model.Request) (model.Result, error) { return app.Fetch(ctx, req, deps) }
}
func newWebFetchBenchmark() app.Benchmark {
	fetch := newWebFetch()
	return app.Benchmark{
		Fetch:           fetch,
		Fixtures:        outbound.DeterministicFixtures,
		RunFixture:      (outbound.FixtureRunner{Fetch: fetch}).Run,
		CheckComparator: outbound.CheckComparator,
		RunComparator:   outbound.RunComparator,
		Now:             time.Now,
	}
}
func webFetchDependencies() webfetchcli.Deps {
	return webfetchcli.Deps{Stdout: os.Stdout, Fetch: newWebFetch(), RunBenchmark: newWebFetchBenchmark().Run, DeterministicFixtures: outbound.DeterministicFixtures}
}
func newWebFetchProbe() probe.Validator {
	return probe.Validator{DeterministicFixtures: outbound.DeterministicFixtures, RunBenchmark: newWebFetchBenchmark().Run}
}
