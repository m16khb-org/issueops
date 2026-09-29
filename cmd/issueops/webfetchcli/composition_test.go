package webfetchcli

import (
	"context"
	"io"
	outbound "issueops/internal/adapter/outbound/webfetch"
	app "issueops/internal/application/webfetch"
	model "issueops/internal/contract/webfetch"
	"net/http"
	"time"
)

func testFetch(ctx context.Context, req model.Request) (model.Result, error) {
	return app.Fetch(ctx, req, app.Dependencies{HTTPClient: outbound.NewHTTPClient(http.DefaultClient), URLPolicy: app.URLValidator{Resolver: outbound.NetResolver{}}, Now: time.Now, RetryAfter: outbound.ParseRetryAfter})
}
func testBenchmark() app.Benchmark {
	return app.Benchmark{Fetch: testFetch, Fixtures: outbound.DeterministicFixtures, RunFixture: (outbound.FixtureRunner{Fetch: testFetch}).Run, CheckComparator: outbound.CheckComparator, RunComparator: outbound.RunComparator, Now: time.Now}
}

func testDependencies(stdout io.Writer) Deps {
	return Deps{Stdout: stdout, Fetch: testFetch, RunBenchmark: testBenchmark().Run, DeterministicFixtures: outbound.DeterministicFixtures}
}
