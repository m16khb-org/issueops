// Package webfetch 패키지는 공개 웹 가져오기 도구의 outbound adapter다. net/http 기반
// 클라이언트, 호스트 이름 해석기, Retry-After 해석, 그리고 로컬 서버로 재생하는
// 벤치마크 fixture 실행기를 제공한다.
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

// FixtureRunner 타입은 벤치마크 fixture 하나를 로컬 httptest 서버로 재생하며 주입된
// Fetch로 가져온다.
type FixtureRunner struct {
	Fetch func(context.Context, webfetchcontract.Request) (webfetchcontract.Result, error)
}

// CheckComparator 함수는 비교 대상 명령 파일이 존재하는지 확인한다.
func CheckComparator(command string) error { _, err := os.Stat(command); return err }

// RunComparator 함수는 비교 대상 명령을 "--url url --json"으로 실행하고 표준 출력을
// 돌려준다.
func RunComparator(ctx context.Context, command, url string) ([]byte, error) {
	return exec.CommandContext(ctx, command, "--url", url, "--json").Output()
}

// DeterministicFixtures 함수는 네트워크 없이 재생할 수 있는 고정 벤치마크 fixture 목록을
// 돌려준다. 각 fixture는 기대하는 분류 범주를 함께 선언한다.
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

// Run 메서드는 fixture를 재생하는 로컬 서버를 띄워 그 URL을 가져오고 결과를 돌려준다.
// 사설망 접근을 허용하며, 상태 코드가 지정되지 않은 fixture는 200으로 재생한다. 두
// 번째 반환값은 최종 대상 fetch 횟수이며 이 구현은 세지 않고 0을 돌려준다.
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
