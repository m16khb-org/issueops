package webfetch

import (
	"context"
	app "issueops/internal/application/webfetch"
	model "issueops/internal/contract/webfetch"
	port "issueops/internal/port/webfetch"
	"net/http"
	"time"
)

type Options struct {
	HTTPClient port.HTTPClient
	Resolver   port.Resolver
	Now        func() time.Time
}

func Fetch(ctx context.Context, req model.Request) (model.Result, error) {
	return FetchWithOptions(ctx, req, Options{})
}
func FetchWithOptions(ctx context.Context, req model.Request, options Options) (model.Result, error) {
	client := options.HTTPClient
	if client == nil {
		client = NewHTTPClient(http.DefaultClient)
	}
	resolver := options.Resolver
	if resolver == nil {
		resolver = NetResolver{}
	}
	return app.Fetch(ctx, req, app.Dependencies{HTTPClient: client, URLPolicy: app.URLValidator{Resolver: resolver}, Now: options.Now, RetryAfter: ParseRetryAfter})
}
func RunBenchmark(ctx context.Context, req model.BenchmarkRequest) (model.BenchmarkResult, error) {
	service := app.Benchmark{Fetch: Fetch, Fixtures: DeterministicFixtures, RunFixture: (FixtureRunner{Fetch: Fetch}).Run, CheckComparator: CheckComparator, RunComparator: RunComparator, Now: time.Now}
	return service.Run(ctx, req)
}
