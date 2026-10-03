package nativeintegration

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalOmoMCPAcceptsHTTPAndRejectsUnsafeConfigurations(t *testing.T) {
	bearer := base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	for _, test := range []struct {
		name   string
		url    string
		auth   string
		hybrid bool
		want   bool
	}{
		{"shared-http", "http://127.0.0.1:47831/mcp", "Bearer " + bearer, false, true},
		{"ipv6-custom-port", "http://[::1]:47832/mcp", "Bearer " + bearer, false, true},
		{"remote", "http://192.0.2.1:47831/mcp", "Bearer " + bearer, false, false},
		{"hostname", "http://localhost:47831/mcp", "Bearer " + bearer, false, false},
		{"wrong-path", "http://127.0.0.1:47831/other", "Bearer " + bearer, false, false},
		{"wrong-scheme", "https://127.0.0.1:47831/mcp", "Bearer " + bearer, false, false},
		{"missing-port", "http://127.0.0.1/mcp", "Bearer " + bearer, false, false},
		{"invalid-port", "http://127.0.0.1:99999/mcp", "Bearer " + bearer, false, false},
		{"query", "http://127.0.0.1:47831/mcp?key=x", "Bearer " + bearer, false, false},
		{"userinfo", "http://user@127.0.0.1:47831/mcp", "Bearer " + bearer, false, false},
		{"fragment", "http://127.0.0.1:47831/mcp#x", "Bearer " + bearer, false, false},
		{"missing-auth", "http://127.0.0.1:47831/mcp", "", false, false},
		{"short-auth", "http://127.0.0.1:47831/mcp", "Bearer short", false, false},
		{"invalid-header", "http://127.0.0.1:47831/mcp", "Bearer " + bearer + "\n", false, false},
		{"wrong-auth-scheme", "http://127.0.0.1:47831/mcp", "Basic " + bearer, false, false},
		{"hybrid-command", "http://127.0.0.1:47831/mcp", "Bearer " + bearer, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := map[string]any{"type": "http", "url": test.url, "headers": map[string]any{"Authorization": test.auth}}
			if test.hybrid {
				server["command"] = "/harness/bin/issueops"
			}
			body, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"issueops": server}})
			if err != nil {
				t.Fatal(err)
			}
			if got := hasCanonicalOmoMCP(body, "/harness/bin/issueops", "/harness"); got != test.want {
				t.Fatalf("canonical=%v, want %v", got, test.want)
			}
		})
	}
}
