package mcpcli

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPUnauthorizedLogNamesReasonWithoutLeakingCredentials(t *testing.T) {
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	for _, tc := range []struct {
		name      string
		header    map[string]string
		presented string
		reason    string
	}{
		{name: "missing", header: map[string]string{"Authorization": ""}, reason: "missing"},
		{name: "malformed scheme", header: map[string]string{"Authorization": "Basic presented-secret-credential"}, presented: "presented-secret-credential", reason: "malformed"},
		{name: "malformed no space", header: map[string]string{"Authorization": "Bearerpresented-secret-credential"}, presented: "presented-secret-credential", reason: "malformed"},
		{name: "malformed empty token", header: map[string]string{"Authorization": "Bearer "}, reason: "malformed"},
		{name: "mismatch", header: map[string]string{"Authorization": "Bearer presented-secret-credential"}, presented: "presented-secret-credential", reason: "mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := startHTTPTestServer(t, testTransportServices(), nil)
			tc.header["User-Agent"] = "probe-client/1.2"
			response, err := server.post(t.Context(), revisionInitialize, list, tc.header)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", response.StatusCode)
			}
			logged := server.access.String()
			if !strings.Contains(logged, "status=401") || !strings.Contains(logged, "auth="+tc.reason) {
				t.Fatalf("log lacks status=401 auth=%s: %q", tc.reason, logged)
			}
			if !strings.Contains(logged, "user_agent=probe-client/1.2") {
				t.Fatalf("log lacks user_agent hint: %q", logged)
			}
			if tc.presented != "" && strings.Contains(logged, tc.presented) {
				t.Fatalf("log leaked the presented credential: %q", logged)
			}
			if strings.Contains(logged, server.bearer) || strings.Contains(logged, server.bearer[:8]) {
				t.Fatalf("log leaked the installed bearer or a prefix: %q", logged)
			}
		})
	}
}

func TestHTTPAuthorizedAndForbiddenLogsCarryNoAuthDiagnostics(t *testing.T) {
	server := startHTTPTestServer(t, testTransportServices(), nil)
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	for _, header := range []map[string]string{nil, {"Origin": "http://evil.example", "Authorization": ""}} {
		response, err := server.post(t.Context(), revisionInitialize, list, header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
	}
	logged := server.access.String()
	if !strings.Contains(logged, "status=200") || !strings.Contains(logged, "status=403") {
		t.Fatalf("expected 200 and 403 log lines: %q", logged)
	}
	if strings.Contains(logged, "auth=") || strings.Contains(logged, "user_agent=") {
		t.Fatalf("non-401 log line carries 401 diagnostics: %q", logged)
	}
}

func TestHTTPUnauthorizedUserAgentIsBounded(t *testing.T) {
	server := startHTTPTestServer(t, testTransportServices(), nil)
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	agent := strings.Repeat("a", 200)
	response, err := server.post(t.Context(), revisionInitialize, list, map[string]string{"Authorization": "", "User-Agent": agent})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	logged := server.access.String()
	if !strings.Contains(logged, "user_agent="+strings.Repeat("a", 80)+" ") && !strings.HasSuffix(strings.TrimSpace(logged), "user_agent="+strings.Repeat("a", 80)) {
		t.Fatalf("user_agent not cut to 80 chars: %q", logged)
	}
	if strings.Contains(logged, strings.Repeat("a", 81)) {
		t.Fatalf("user_agent longer than 80 chars: %q", logged)
	}
}

func TestSanitizeClientHint(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		max            int
	}{
		{name: "plain", in: "curl/8.0", max: 80, want: "curl/8.0"},
		{name: "control characters stripped", in: "a\r\nb\x00c\x1b[31md\te", max: 80, want: "abc[31mde"},
		{name: "invalid utf8 dropped", in: "ok\xff\xfeend", max: 80, want: "okend"},
		{name: "truncated by runes", in: "가나다라마", max: 3, want: "가나다"},
		{name: "empty", in: "", max: 80, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeClientHint(tc.in, tc.max); got != tc.want {
				t.Fatalf("sanitizeClientHint(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}
