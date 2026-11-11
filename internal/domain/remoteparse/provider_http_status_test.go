package remoteparse

import (
	"fmt"
	"net/http"
	"testing"
)

func TestProviderHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		diagnostic string
		want       int
	}{
		{"http prefix with url", "HTTP 404: Not Found (https://api.github.com/x)", 404},
		{"http prefix bad credentials", "HTTP 401: Bad credentials", 401},
		{"http prefix validation failed", "HTTP 422: Validation Failed", 422},
		{"error with reason phrase", "ERROR 404 Not Found", 404},
		{"bare code with reason phrase", "403 Forbidden", 403},
		{"bad gateway", "HTTP 502: Bad Gateway", 502},
		{"phrase after newline", "gh: create failed\n422 Unprocessable Entity", 422},
		{"same status twice", "HTTP 404: Not Found HTTP 404: Not Found", 404},

		{"empty", "", 0},
		{"exit status", "exit status 1", 0},
		{"digit before", "issue 1404 Not Found", 0},
		{"digit after", "HTTP 4040 Not Found", 0},
		{"inside url path", "https://h/x/404/y", 0},
		{"duration", "took 500ms", 0},
		{"out of range", "200 OK", 0},
		{"bare code", "404", 0},
		{"wrong phrase", "404 Forbidden", 0},
		{"two different statuses", "HTTP 404: Not Found then 500 Internal Server Error", 0},
		{"prefix below range", "HTTP 302: Found", 0},
		{"prefix above range", "HTTP 600: x", 0},
		{"phrase runs into a word", "404 Not Founder", 0},
		{"prefix without colon", "HTTP 404 Bad credentials", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderHTTPStatus(tt.diagnostic); got != tt.want {
				t.Fatalf("ProviderHTTPStatus(%q) = %d, want %d", tt.diagnostic, got, tt.want)
			}
		})
	}
	t.Run("reason_phrases_match_net_http", func(t *testing.T) {
		for code, phrase := range httpReasonPhrases {
			if want := http.StatusText(code); phrase != want {
				t.Errorf("httpReasonPhrases[%d] = %q, net/http has %q", code, phrase, want)
			}
		}
		for code := 400; code <= 599; code++ {
			phrase := http.StatusText(code)
			if phrase == "" {
				continue
			}
			diagnostic := fmt.Sprintf("%d %s", code, phrase)
			if got := ProviderHTTPStatus(diagnostic); got != code {
				t.Errorf("ProviderHTTPStatus(%q) = %d, want %d", diagnostic, got, code)
			}
		}
	})
}
