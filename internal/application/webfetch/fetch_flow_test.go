package webfetch

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
	webfetchport "issueops/internal/port/webfetch"
)

type flowClient struct {
	do    func(call int, req webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error)
	urls  []string
	heads []map[string]string
}

func (c *flowClient) Do(_ context.Context, req webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
	c.urls = append(c.urls, req.URL)
	c.heads = append(c.heads, req.Header)
	return c.do(len(c.urls)-1, req)
}

func textResponse(status int, header map[string][]string, body string) webfetchport.HTTPResponse {
	return webfetchport.HTTPResponse{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}

func redirectTo(location string) webfetchport.HTTPResponse {
	return textResponse(302, map[string][]string{"Location": {location}}, "")
}

type recordingPolicy struct {
	seen   []string
	reject func(raw string) error
}

func (p *recordingPolicy) Validate(_ context.Context, raw string, _ bool) (string, error) {
	p.seen = append(p.seen, raw)
	if p.reject != nil {
		if err := p.reject(raw); err != nil {
			return "", err
		}
	}
	return raw, nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom read") }

const longText = "plain readable article body that is comfortably long enough. "

func fetchWith(t *testing.T, client webfetchport.HTTPClient, policy webfetchport.URLPolicy, deps func(*Dependencies)) webfetchcontract.Result {
	t.Helper()
	d := Dependencies{HTTPClient: client, URLPolicy: policy}
	if deps != nil {
		deps(&d)
	}
	result, err := Fetch(t.Context(), webfetchcontract.Request{URL: "https://start.test/"}, d)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	return result
}

func TestFetchRequiresDependencies(t *testing.T) {
	req := webfetchcontract.Request{URL: "https://start.test/"}
	if _, err := Fetch(t.Context(), req, Dependencies{HTTPClient: &flowClient{}}); err == nil {
		t.Error("missing URLPolicy must error")
	}
	if _, err := Fetch(t.Context(), req, Dependencies{URLPolicy: &recordingPolicy{}}); err == nil {
		t.Error("missing HTTPClient must error")
	}
}

func TestFetchSafetyRejectionNeverCallsClient(t *testing.T) {
	client := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		t.Fatal("client must not be called for a rejected URL")
		return webfetchport.HTTPResponse{}, nil
	}}
	policy := &recordingPolicy{reject: func(string) error { return errors.New("unsafe host") }}
	result := fetchWith(t, client, policy, nil)
	if result.OK || result.Category != webfetchcontract.CategoryBlocked || result.StopReason != webfetchcontract.StopReasonSafetyRejected || !result.GridExhausted {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "unsafe host" || len(result.AttemptedRoutes) != 0 {
		t.Fatalf("warnings=%v attempted=%v", result.Warnings, result.AttemptedRoutes)
	}
	if len(result.UntriedRoutes) != 3 || result.UntriedRoutes[0].Reason != "safety_rejected" {
		t.Fatalf("untried = %+v", result.UntriedRoutes)
	}
}

func TestFetchFollowsRedirectAndReportsFinalURL(t *testing.T) {
	client := &flowClient{do: func(call int, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		if call == 0 {
			return redirectTo("/final"), nil
		}
		return textResponse(200, map[string][]string{"Content-Type": {"text/plain"}}, strings.Repeat(longText, 3)), nil
	}}
	policy := &recordingPolicy{}
	result := fetchWith(t, client, policy, nil)
	if !result.OK || result.StopReason != webfetchcontract.StopReasonAccepted || result.GridExhausted {
		t.Fatalf("result = %+v", result)
	}
	if result.FinalURL != "https://start.test/final" || result.URL != "https://start.test/" {
		t.Fatalf("URL=%q FinalURL=%q", result.URL, result.FinalURL)
	}
	if len(client.urls) != 2 || client.urls[1] != "https://start.test/final" {
		t.Fatalf("client urls = %v", client.urls)
	}
	if len(policy.seen) != 2 || policy.seen[1] != "https://start.test/final" {
		t.Fatalf("redirect target must be re-validated: %v", policy.seen)
	}
	if client.heads[0]["User-Agent"] != "issueops-web-fetch/1.0" {
		t.Fatalf("headers = %v", client.heads[0])
	}
	if len(result.AttemptedRoutes) != 1 || result.AttemptedRoutes[0].ID != "direct_http" || result.UntriedRoutes[0].Reason != "not_needed_after_success" {
		t.Fatalf("routes attempted=%v untried=%v", result.AttemptedRoutes, result.UntriedRoutes)
	}
}

func TestFetchStopsOnRedirectProblems(t *testing.T) {
	tests := []struct {
		name         string
		location     string
		reject       func(string) error
		wantCategory string
		wantReason   string
		wantSkipped  string
		wantWarning  string
	}{
		{"empty location", "  ", nil, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonGridExhausted, "empty_redirect_location", ""},
		{"unresolvable location", "http://[::1", nil, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonUnsafeRedirect, "unsafe_redirect", "invalid redirect location"},
		{"policy rejects target", "http://evil.test/", func(raw string) error {
			if strings.Contains(raw, "evil") {
				return errors.New("blocked evil")
			}
			return nil
		}, webfetchcontract.CategoryBlocked, webfetchcontract.StopReasonUnsafeRedirect, "unsafe_redirect", "blocked evil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
				return redirectTo(tt.location), nil
			}}
			result := fetchWith(t, client, &recordingPolicy{reject: tt.reject}, nil)
			if result.OK || !result.GridExhausted || result.Category != tt.wantCategory || result.StopReason != tt.wantReason {
				t.Fatalf("result = %+v", result)
			}
			if len(client.urls) != 1 {
				t.Fatalf("rejected redirect target was fetched: %v", client.urls)
			}
			if result.UntriedRoutes[0].Reason != tt.wantSkipped {
				t.Fatalf("skipped reason = %q", result.UntriedRoutes[0].Reason)
			}
			if tt.wantWarning != "" && !strings.Contains(strings.Join(result.Warnings, "|"), tt.wantWarning) {
				t.Fatalf("warnings = %v", result.Warnings)
			}
			if tt.wantWarning == "" && len(result.Warnings) != 0 {
				t.Fatalf("unexpected warnings = %v", result.Warnings)
			}
		})
	}
}

func TestFetchDetectsRedirectLoopAndHopLimit(t *testing.T) {
	loop := &flowClient{do: func(call int, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		return redirectTo([]string{"/b", "/a"}[call%2]), nil
	}}
	result := fetchWith(t, loop, &recordingPolicy{}, nil)
	if result.StopReason != webfetchcontract.StopReasonRedirectLoop || result.Category != webfetchcontract.CategoryUnknown || !result.GridExhausted {
		t.Fatalf("loop result = %+v", result)
	}
	if len(loop.urls) != 3 {
		t.Fatalf("loop must stop on first revisit, urls=%v", loop.urls)
	}

	chain := &flowClient{do: func(call int, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		return redirectTo("/hop" + string(rune('a'+call))), nil
	}}
	result = fetchWith(t, chain, &recordingPolicy{}, nil)
	if result.StopReason != webfetchcontract.StopReasonRedirectLoop || result.OK {
		t.Fatalf("chain result = %+v", result)
	}
	if len(chain.urls) != maxRedirects+1 {
		t.Fatalf("fetched %d urls, want %d", len(chain.urls), maxRedirects+1)
	}
}

func TestFetchReportsTransportAndReadErrors(t *testing.T) {
	failing := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		return webfetchport.HTTPResponse{}, errors.New("dial refused")
	}}
	result := fetchWith(t, failing, &recordingPolicy{}, nil)
	if result.StopReason != webfetchcontract.StopReasonError || result.Category != webfetchcontract.CategoryUnknown || !result.GridExhausted ||
		result.UntriedRoutes[0].Reason != "request_error" || len(result.Warnings) != 1 || result.Warnings[0] != "dial refused" {
		t.Fatalf("request error result = %+v", result)
	}

	broken := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		return webfetchport.HTTPResponse{StatusCode: 200, Body: io.NopCloser(errReader{})}, nil
	}}
	result = fetchWith(t, broken, &recordingPolicy{}, nil)
	if result.OK || result.UntriedRoutes[0].Reason != "read_error" || len(result.Warnings) != 1 || result.Warnings[0] != "boom read" {
		t.Fatalf("read error result = %+v", result)
	}
}

func TestFetchRetriesRateLimitOnlyOnceAndOnlyWithHeader(t *testing.T) {
	limited := func(header map[string][]string) *flowClient {
		return &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
			return textResponse(429, header, "slow"), nil
		}}
	}
	withHeader := limited(map[string][]string{"Retry-After": {"1"}})
	result := fetchWith(t, withHeader, &recordingPolicy{}, nil)
	if len(withHeader.urls) != 2 {
		t.Fatalf("calls = %d, want exactly one retry", len(withHeader.urls))
	}
	if result.Category != webfetchcontract.CategoryRateLimited || result.StopReason != webfetchcontract.StopReasonRateLimited || result.OK ||
		result.Metadata["retry_after_retried"] != true || result.UntriedRoutes[0].Reason != webfetchcontract.CategoryRateLimited {
		t.Fatalf("result = %+v", result)
	}

	noHeader := limited(nil)
	result = fetchWith(t, noHeader, &recordingPolicy{}, nil)
	if len(noHeader.urls) != 1 || result.Category != webfetchcontract.CategoryRateLimited {
		t.Fatalf("calls=%d result=%+v", len(noHeader.urls), result)
	}
	if _, ok := result.Metadata["retry_after_retried"]; ok {
		t.Fatalf("metadata marks retry that did not happen: %v", result.Metadata)
	}
}

func TestFetchRetryDelayIsBoundedAndOptional(t *testing.T) {
	newClient := func() *flowClient {
		return &flowClient{do: func(call int, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
			if call == 0 {
				return textResponse(429, map[string][]string{"Retry-After": {"120"}}, ""), nil
			}
			return textResponse(200, nil, strings.Repeat(longText, 3)), nil
		}}
	}
	var gotValue string
	var slept []time.Duration
	result := fetchWith(t, newClient(), &recordingPolicy{}, func(d *Dependencies) {
		d.RetryAfter = func(v string) time.Duration { gotValue = v; return 10 * time.Second }
		d.Sleep = func(_ context.Context, delay time.Duration) { slept = append(slept, delay) }
	})
	if !result.OK || gotValue != "120" || len(slept) != 1 || slept[0] != 100*time.Millisecond {
		t.Fatalf("ok=%v value=%q slept=%v", result.OK, gotValue, slept)
	}

	for name, retryAfter := range map[string]func(string) time.Duration{
		"nil parser": nil,
		"zero delay": func(string) time.Duration { return 0 },
		"negative":   func(string) time.Duration { return -time.Second },
	} {
		slept = nil
		result = fetchWith(t, newClient(), &recordingPolicy{}, func(d *Dependencies) {
			d.RetryAfter = retryAfter
			d.Sleep = func(_ context.Context, delay time.Duration) { slept = append(slept, delay) }
		})
		if !result.OK || len(slept) != 0 {
			t.Errorf("%s: ok=%v slept=%v, want retry without sleeping", name, result.OK, slept)
		}
	}

	started := time.Now()
	result = fetchWith(t, newClient(), &recordingPolicy{}, func(d *Dependencies) {
		d.RetryAfter = func(string) time.Duration { return 20 * time.Millisecond }
	})
	if !result.OK || result.Metadata["retry_after_retried"] != true || time.Since(started) < 20*time.Millisecond {
		t.Fatalf("timer-based retry must wait the parsed delay before retrying: result=%+v elapsed=%s", result, time.Since(started))
	}
}

func TestFetchRetryWaitEndsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := &flowClient{do: func(call int, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		if call == 0 {
			return textResponse(429, map[string][]string{"Retry-After": {"9"}}, ""), nil
		}
		return textResponse(200, nil, strings.Repeat(longText, 3)), nil
	}}
	started := time.Now()
	result, err := Fetch(ctx, webfetchcontract.Request{URL: "https://start.test/"}, Dependencies{
		HTTPClient: client, URLPolicy: &recordingPolicy{},
		RetryAfter: func(string) time.Duration { return time.Hour },
	})
	if err != nil || !result.OK || len(client.urls) != 2 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, len(client.urls))
	}
	// The retry delay is capped at 100ms, so finishing well under the cap proves ctx.Done ended the wait.
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("cancelled context did not interrupt the retry wait: %s", elapsed)
	}
}

func TestFetchClassifiesFinalResponses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantOK     bool
		wantCat    string
		wantReason string
		wantSkip   string
	}{
		{"not found", 404, "gone", false, webfetchcontract.CategoryNotFound, webfetchcontract.StopReasonNotFound, webfetchcontract.CategoryNotFound},
		{"paywalled counts as ok", 200, "Subscribe to read the rest", true, webfetchcontract.CategoryPaywalled, webfetchcontract.StopReasonGridExhausted, webfetchcontract.CategoryPaywalled},
		{"challenge", 200, "Just a moment...", false, webfetchcontract.CategoryChallenge, webfetchcontract.StopReasonGridExhausted, webfetchcontract.CategoryChallenge},
		{"strong", 200, strings.Repeat(longText, 20), true, webfetchcontract.CategoryStrongOK, webfetchcontract.StopReasonAccepted, "not_needed_after_success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
				return textResponse(tt.status, map[string][]string{"Content-Type": {"text/html"}}, tt.body), nil
			}}
			result := fetchWith(t, client, &recordingPolicy{}, nil)
			if result.OK != tt.wantOK || result.Category != tt.wantCat || result.StopReason != tt.wantReason || result.UntriedRoutes[0].Reason != tt.wantSkip {
				t.Fatalf("result = %+v", result)
			}
			if result.GridExhausted != (tt.wantReason != webfetchcontract.StopReasonAccepted) {
				t.Fatalf("GridExhausted = %v", result.GridExhausted)
			}
		})
	}
}

func TestFetchTruncatesContentToMaxChars(t *testing.T) {
	client := &flowClient{do: func(int, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
		return textResponse(200, nil, strings.Repeat(longText, 20)), nil
	}}
	result, err := Fetch(t.Context(), webfetchcontract.Request{URL: "https://start.test/", MaxChars: 10}, Dependencies{HTTPClient: client, URLPolicy: &recordingPolicy{}})
	if err != nil || len([]rune(result.Content)) != 10 {
		t.Fatalf("content=%q err=%v", result.Content, err)
	}
}

func TestRequestTimeoutDefaultsWhenUnset(t *testing.T) {
	if got := requestTimeout(webfetchcontract.Request{}); got != 30*time.Second {
		t.Errorf("default = %s", got)
	}
	if got := requestTimeout(webfetchcontract.Request{Timeout: -1}); got != 30*time.Second {
		t.Errorf("negative = %s", got)
	}
	if got := requestTimeout(webfetchcontract.Request{Timeout: time.Second}); got != time.Second {
		t.Errorf("explicit = %s", got)
	}
}

type resolverFake struct {
	addrs []string
	err   error
	hosts []string
}

func (r *resolverFake) LookupIPAddr(_ context.Context, host string) ([]string, error) {
	r.hosts = append(r.hosts, host)
	return r.addrs, r.err
}

func TestURLValidatorResolvesHostnamesAndBlocksUnsafeAnswers(t *testing.T) {
	ctx := t.Context()
	t.Run("public answer accepted", func(t *testing.T) {
		r := &resolverFake{addrs: []string{"93.184.216.34"}}
		got, err := URLValidator{Resolver: r}.Validate(ctx, "https://example.test/x?y=1", false)
		if err != nil || got != "https://example.test/x?y=1" || len(r.hosts) != 1 || r.hosts[0] != "example.test" {
			t.Fatalf("got=%q err=%v hosts=%v", got, err, r.hosts)
		}
	})
	t.Run("any unsafe answer rejects", func(t *testing.T) {
		r := &resolverFake{addrs: []string{"93.184.216.34", "10.0.0.5"}}
		_, err := URLValidator{Resolver: r}.Validate(ctx, "https://rebind.test/", false)
		if err == nil || !strings.Contains(err.Error(), "unsafe resolved host") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("private answer allowed when permitted", func(t *testing.T) {
		r := &resolverFake{addrs: []string{"10.0.0.5"}}
		if _, err := (URLValidator{Resolver: r}).Validate(ctx, "https://intranet.test/", true); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("metadata answer rejected even when private allowed", func(t *testing.T) {
		r := &resolverFake{addrs: []string{"169.254.169.254"}}
		if _, err := (URLValidator{Resolver: r}).Validate(ctx, "https://meta.test/", true); err == nil {
			t.Fatal("metadata IP accepted")
		}
	})
	t.Run("resolution failure wraps cause", func(t *testing.T) {
		cause := errors.New("nxdomain")
		_, err := URLValidator{Resolver: &resolverFake{err: cause}}.Validate(ctx, "https://nope.test/", false)
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "host resolution failed") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("IP literals skip DNS", func(t *testing.T) {
		r := &resolverFake{err: errors.New("must not be called")}
		for _, raw := range []string{"http://93.184.216.34/", "http://[2606:4700::1111]/"} {
			if _, err := (URLValidator{Resolver: r}).Validate(ctx, raw, false); err != nil {
				t.Errorf("%s: %v", raw, err)
			}
		}
		if len(r.hosts) != 0 {
			t.Errorf("resolver consulted for IP literals: %v", r.hosts)
		}
	})
	t.Run("syntactic rejection precedes DNS", func(t *testing.T) {
		r := &resolverFake{}
		for _, raw := range []string{"ftp://example.test/", "http://127.0.0.1/", "http://localhost/"} {
			if _, err := (URLValidator{Resolver: r}).Validate(ctx, raw, false); err == nil {
				t.Errorf("%s accepted", raw)
			}
		}
		if len(r.hosts) != 0 {
			t.Errorf("resolver consulted: %v", r.hosts)
		}
	})
}
