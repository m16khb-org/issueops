package mcpcli

import (
	"context"
	outbound "issueops/internal/adapter/outbound/webfetch"
	app "issueops/internal/application/webfetch"
	model "issueops/internal/contract/webfetch"
	"net/http"
	"time"
)

func testWebFetch(ctx context.Context, req model.Request) (model.Result, error) {
	return app.Fetch(ctx, req, app.Dependencies{HTTPClient: outbound.NewHTTPClient(http.DefaultClient), URLPolicy: app.URLValidator{Resolver: outbound.NetResolver{}}, Now: time.Now, RetryAfter: outbound.ParseRetryAfter})
}
