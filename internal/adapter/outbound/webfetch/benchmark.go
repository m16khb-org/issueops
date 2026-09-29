package webfetch

import (
	"bytes"
	"compress/gzip"
	"context"
	webfetchcontract "issueops/internal/contract/webfetch"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"time"
)

type FixtureRunner struct {
	Fetch func(context.Context, webfetchcontract.Request) (webfetchcontract.Result, error)
}

func CheckComparator(command string) error { _, err := os.Stat(command); return err }
func RunComparator(ctx context.Context, command, url string) ([]byte, error) {
	return exec.CommandContext(ctx, command, "--url", url, "--json").Output()
}
func DeterministicFixtures() []webfetchcontract.BenchmarkFixture {
	return []webfetchcontract.BenchmarkFixture{
		{ID: "article_basic", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: "<main>" + strings.Repeat("article body ", 60) + "</main>", Expected: []string{webfetchcontract.CategoryStrongOK}, MinBodyChars: 500},
		{ID: "json_api", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"ok":true,"items":[{"id":1}]}`, Expected: []string{webfetchcontract.CategoryStrongOK}},
		{ID: "rss_feed", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/rss+xml"}, Body: `<rss><channel><item><title>one</title></item></channel></rss>`, Expected: []string{webfetchcontract.CategoryStrongOK, webfetchcontract.CategoryWeakOK}},
		{ID: "empty_spa", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `<html><body><div id="root"></div><script src="/app.js"></script></body></html>`, Expected: []string{webfetchcontract.CategorySuspectOK}},
		{ID: "waf_challenge", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `Just a moment... captcha check your browser`, Expected: []string{webfetchcontract.CategoryChallenge, webfetchcontract.CategoryBlocked}},
		{ID: "login_wall", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `Sign in to continue. Log in to view this page.`, Expected: []string{webfetchcontract.CategoryAuthRequired}},
		{ID: "paywall_shell", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `<meta property="og:title" content="Headline"><body>Subscribe to read this member-only article.</body>`, Expected: []string{webfetchcontract.CategoryPaywalled}},
		{ID: "rate_limit", StatusCode: http.StatusTooManyRequests, Headers: map[string]string{"Content-Type": "text/plain", "Retry-After": "1"}, Body: `slow down`, Expected: []string{webfetchcontract.CategoryRateLimited}},
		{ID: "redirect_loop", RedirectLoop: true, Expected: []string{webfetchcontract.CategoryUnknown}},
		{ID: "unsafe_redirect", UnsafeRedirect: true, Expected: []string{webfetchcontract.CategoryBlocked}},
		{ID: "gzip_charset", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html; charset=shift_jis", "Content-Encoding": "gzip"}, Body: gzipString("<main>" + strings.Repeat("decoded body ", 60) + "</main>"), Expected: []string{webfetchcontract.CategoryStrongOK}},
		{ID: "malformed_content", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"broken":`, Expected: []string{webfetchcontract.CategoryUnknown, webfetchcontract.CategorySuspectOK}},
	}
}

func (runner FixtureRunner) Run(ctx context.Context, fixture webfetchcontract.BenchmarkFixture, timeout time.Duration) (webfetchcontract.Result, int, error) {
	finalTargetFetches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fixture.RedirectLoop {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		if fixture.UnsafeRedirect {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		for k, v := range fixture.Headers {
			w.Header().Set(k, v)
		}
		// live 전용 fixture(URL만 있고 StatusCode 미지정 = 0)를 오프라인
		// 서버로 재생할 때 0은 http 허용 코드가 아니어서 panic한다. 실측
		// 2026-08-22: public-fixtures.json을 오프라인으로 돌리면 즉시
		// 패닉했다. 미지정 상태는 200으로 재생한다.
		status := fixture.StatusCode
		if status < 100 || status > 599 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(fixture.Body))
	}))
	defer server.Close()

	fetchResult, err := runner.Fetch(ctx, webfetchcontract.Request{URL: server.URL, Timeout: timeout, AllowPrivateNetwork: true})
	return fetchResult, finalTargetFetches, err
}

func gzipString(s string) string {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte(s))
	_ = writer.Close()
	return buf.String()
}
