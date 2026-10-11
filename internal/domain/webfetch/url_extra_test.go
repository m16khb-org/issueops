package webfetch

import (
	"strings"
	"testing"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
)

func TestSanitizeURLRedactsUserinfoAndTrims(t *testing.T) {
	got := SanitizeURL("  https://user:secret@example.test/path?q=1  ")
	if strings.Contains(got, "secret") || strings.Contains(got, "user:") || !strings.Contains(got, "example.test/path?q=1") {
		t.Fatalf("SanitizeURL leaked credentials or lost path: %q", got)
	}
	if got := SanitizeURL("https://example.test/a"); got != "https://example.test/a" {
		t.Fatalf("credential-free URL changed: %q", got)
	}
	if got := SanitizeURL("  http://[::1  "); got != "http://[::1" {
		t.Fatalf("unparseable URL must be returned trimmed, got %q", got)
	}
}

func TestResolveRedirectResolvesRelativeAndRejectsInvalid(t *testing.T) {
	tests := []struct{ base, location, want string }{
		{"https://example.test/a/b", "/root", "https://example.test/root"},
		{"https://example.test/a/b", "c", "https://example.test/a/c"},
		{"https://example.test/a/b", "https://other.test/x", "https://other.test/x"},
	}
	for _, tt := range tests {
		got, err := ResolveRedirect(tt.base, tt.location)
		if err != nil || got != tt.want {
			t.Errorf("ResolveRedirect(%q,%q) = %q, %v; want %q", tt.base, tt.location, got, err, tt.want)
		}
	}
	if _, err := ResolveRedirect("http://[::1", "/x"); err == nil || !strings.Contains(err.Error(), "redirect base") {
		t.Errorf("invalid base error = %v", err)
	}
	if _, err := ResolveRedirect("https://example.test", "http://[::1"); err == nil || !strings.Contains(err.Error(), "redirect location") {
		t.Errorf("invalid location error = %v", err)
	}
}

func TestIsRedirectOnlyFollowsRedirectStatuses(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		if !IsRedirect(status) {
			t.Errorf("%d must be a redirect", status)
		}
	}
	for _, status := range []int{200, 300, 304, 305, 306, 309, 404} {
		if IsRedirect(status) {
			t.Errorf("%d must not be a redirect", status)
		}
	}
}

func TestParseFetchURLRejectsMalformedAndLocalhostSubdomains(t *testing.T) {
	if _, err := ParseFetchURL("http://[::1", true); err == nil || !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("malformed URL error = %v", err)
	}
	if _, err := ParseFetchURL("http://app.localhost/", false); err == nil {
		t.Error("*.localhost must be rejected without allowPrivate")
	}
	if _, err := ParseFetchURL("http://APP.Localhost/", true); err != nil {
		t.Errorf("*.localhost must be allowed with allowPrivate: %v", err)
	}
	if _, err := ParseFetchURL("http://[fe80::1%25eth0]/", false); err == nil || !strings.Contains(err.Error(), "unsafe host") {
		t.Errorf("zoned link-local literal must be rejected without allowPrivate: %v", err)
	}
	u, err := ParseFetchURL("  https://example.test/p  ", false)
	if err != nil || u.Host != "example.test" {
		t.Errorf("public URL = %v, %v", u, err)
	}
}

func TestIsUnsafeHostCoversAddressClasses(t *testing.T) {
	tests := []struct {
		host         string
		allowPrivate bool
		want         bool
	}{
		{"0.0.0.0", true, true},
		{"169.254.1.1", false, true},
		{"169.254.1.1", true, false},
		{"224.0.0.1", false, true},
		{"224.0.0.1", true, false},
		{"::", false, true},
		{"192.168.1.1", false, true},
		{"192.168.1.1", true, false},
		{"[fe80::1]", false, true},
		{"::ffff:10.0.0.1", false, true},
		{"8.8.8.8", false, false},
		{"2606:4700::1111", false, false},
		{"example.test", false, false},
		{"fe80::1%eth0", false, true},
		{"fe80::1%eth0", true, false},
		{"fe80::1%25eth0", false, true},
	}
	for _, tt := range tests {
		if got := IsUnsafeHost(tt.host, tt.allowPrivate); got != tt.want {
			t.Errorf("IsUnsafeHost(%q,%v) = %v, want %v", tt.host, tt.allowPrivate, got, tt.want)
		}
	}
}

func TestIsIPAddressRecognisesLiteralsOnly(t *testing.T) {
	for _, host := range []string{"1.2.3.4", "::1", "::ffff:1.2.3.4"} {
		if !IsIPAddress(host) {
			t.Errorf("%q must be an IP", host)
		}
	}
	for _, host := range []string{"example.test", "", "300.1.1.1", "fe80::1%eth0", "[::1]"} {
		if IsIPAddress(host) {
			t.Errorf("%q must not be an IP", host)
		}
	}
}

func TestValidateLiveBenchmarkRequiresOptInThenFixtures(t *testing.T) {
	fixtures := []webfetchcontract.BenchmarkFixture{{ID: "a"}}
	msg, err := ValidateLiveBenchmark(webfetchcontract.BenchmarkRequest{Fixtures: fixtures})
	if err == nil || !strings.Contains(msg, "ISSUEOPS_WEBFETCH_LIVE=1") {
		t.Errorf("missing opt-in: %q, %v", msg, err)
	}
	msg, err = ValidateLiveBenchmark(webfetchcontract.BenchmarkRequest{LiveOptIn: true})
	if err == nil || !strings.Contains(msg, "URL fixtures") {
		t.Errorf("missing fixtures: %q, %v", msg, err)
	}
	if msg, err = ValidateLiveBenchmark(webfetchcontract.BenchmarkRequest{LiveOptIn: true, Fixtures: fixtures}); err != nil || msg != "" {
		t.Errorf("valid request rejected: %q, %v", msg, err)
	}
}

func TestBenchmarkTimeoutDefaultsByMode(t *testing.T) {
	if got := BenchmarkTimeout(3*time.Second, true); got != 3*time.Second {
		t.Errorf("explicit timeout ignored: %s", got)
	}
	if got := BenchmarkTimeout(0, true); got != 10*time.Second {
		t.Errorf("live default = %s", got)
	}
	if got := BenchmarkTimeout(-1, false); got != 2*time.Second {
		t.Errorf("offline default = %s", got)
	}
}

func TestPercentHandlesZeroDenominator(t *testing.T) {
	if got := Percent(1, 4); got != 25 {
		t.Errorf("Percent(1,4) = %v", got)
	}
	if Percent(3, 0) != 0 || Percent(3, -2) != 0 {
		t.Error("non-positive denominator must yield 0")
	}
}

func TestPercentileLatencyUsesNearestRankWithoutMutatingInput(t *testing.T) {
	values := []int64{50, 10, 40, 20, 30}
	tests := []struct {
		p    int
		want int64
	}{{0, 10}, {-5, 10}, {50, 30}, {95, 50}, {100, 50}, {1000, 50}}
	for _, tt := range tests {
		if got := PercentileLatency(values, tt.p); got != tt.want {
			t.Errorf("PercentileLatency(p=%d) = %d, want %d", tt.p, got, tt.want)
		}
	}
	if values[0] != 50 || values[1] != 10 {
		t.Errorf("input mutated: %v", values)
	}
	if PercentileLatency(nil, 50) != 0 {
		t.Error("empty input must yield 0")
	}
}

func TestHasLiveBenchmarkURLIgnoresBlank(t *testing.T) {
	if HasLiveBenchmarkURL(webfetchcontract.BenchmarkFixture{URL: " \t"}) {
		t.Error("blank URL counted")
	}
	if !HasLiveBenchmarkURL(webfetchcontract.BenchmarkFixture{URL: " https://example.test "}) {
		t.Error("real URL not counted")
	}
}

func TestCompleteBenchmarkLiveWithoutComparator(t *testing.T) {
	result := CompleteBenchmark(NewBenchmarkResult(1), true, "")
	if !result.OK || !result.LiveParityEvaluated || result.LiveParityStatus != "evaluated without baseline comparator" {
		t.Fatalf("result = %+v", result)
	}
	result = NewBenchmarkResult(1)
	result.LiveParityReport.FalseStrongOK = 1
	if got := CompleteBenchmark(result, true, ""); got.OK {
		t.Fatalf("live false strong_ok must fail: %+v", got)
	}
}

func TestValidateResponseExtractsOGTitleMetadata(t *testing.T) {
	long := strings.Repeat("body text ", 30)
	tests := []struct {
		name, html, want string
	}{
		{"double quotes", `<meta property="og:title" content="Hello">` + long, "Hello"},
		{"single quotes", `<meta property="og:title" content='Hi there'>` + long, "Hi there"},
		{"case insensitive", `<META PROPERTY="OG:TITLE" CONTENT="Up">` + long, "Up"},
		{"unquoted content", `<meta property="og:title" content=bare>` + long, ""},
		{"unterminated quote", `<meta property="og:title" content="open` + long, ""},
		{"no content attribute", `<meta property="og:title">` + long, ""},
		{"content at end of text", `<meta property="og:title" content=`, ""},
		{"no marker", "<p>" + long + "</p>", ""},
	}
	for _, tt := range tests {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Header: map[string][]string{"Content-Type": {"text/html"}}, Body: []byte(tt.html)})
		title, _ := got.Metadata["og_title"].(string)
		if title != tt.want {
			t.Errorf("%s: og_title = %q, want %q", tt.name, title, tt.want)
		}
		if got.Metadata["content_type"] != "text/html" {
			t.Errorf("%s: content_type metadata = %v", tt.name, got.Metadata["content_type"])
		}
	}
}
