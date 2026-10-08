package extensionhost

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHTTPInstallMergesOnlyIssueOpsEntryOwnerOnly(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		req := testRequest(t)
		userPath := tc.userMCPPath(req)
		writeTestJSON(t, userPath, map[string]any{"mcpServers": map[string]any{
			"other":    map[string]any{"command": "other"},
			"issueops": map[string]any{"command": "/old/issueops", "args": []any{"mcp"}, "env": map[string]any{"ISSUEOPS_ROOT": "/old"}},
		}})
		if err := os.Chmod(userPath, 0o644); err != nil {
			t.Fatal(err)
		}
		projectPath := filepath.Join(req.Root, "."+tc.spec.Host, "mcp.json")
		writeTestJSON(t, projectPath, map[string]any{"mcpServers": map[string]any{
			"keep":             map[string]any{"command": "keep"},
			"issueops_project": map[string]any{"command": "./bin/issueops", "args": []any{"mcp"}},
		}})
		req.ProjectLocal = true
		req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"

		if result, err := tc.installer().Install(req); err != nil || !result.OK {
			t.Fatalf("install err=%v result=%+v", err, result)
		}
		catalog, err := testDependencies().MCPCatalogSHA256()
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"mcpServers": map[string]any{
			"other": map[string]any{"command": "other"},
			"issueops": map[string]any{"type": "http", "url": "http://127.0.0.1:47831/mcp", "headers": map[string]any{
				"Authorization": "Bearer secret-abc", mcpCatalogHeader: catalog,
			}},
		}}
		if got := readTestJSON(t, userPath); !reflect.DeepEqual(got, want) {
			t.Fatalf("user config = %#v", got)
		}
		if info, err := os.Stat(userPath); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("user config mode = %v, %v", info.Mode().Perm(), err)
		}
		if got := readTestJSON(t, projectPath); !reflect.DeepEqual(got, map[string]any{"mcpServers": map[string]any{"keep": map[string]any{"command": "keep"}}}) {
			t.Fatalf("project config must drop only the duplicate issueops entry: %#v", got)
		}
		if _, err := tc.installer().VerifyActivation(req); err != nil {
			t.Fatalf("http activation readback: %v", err)
		}
	})
}

// The host keys its tool-catalog cache on the whole server config, so the HTTP
// entry must change when the advertised catalog does; otherwise new sessions
// reuse a stale tools/list for up to the cache TTL.
func TestHTTPEntryChangesWithTheAdvertisedCatalog(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		entry := func(digest string) map[string]any {
			deps := testDependencies()
			deps.MCPCatalogSHA256 = func() (string, error) { return digest, nil }
			req := testRequest(t)
			req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"
			server, err := NewInstaller(tc.spec, deps, nil).userMCPServer(req)
			if err != nil {
				t.Fatal(err)
			}
			return server
		}
		first, second := entry("aaaa"), entry("bbbb")
		if reflect.DeepEqual(first, second) {
			t.Fatalf("catalog change left the %s HTTP entry unchanged: %#v", tc.spec.DisplayName, first)
		}
		headers, _ := second["headers"].(map[string]any)
		if headers["Authorization"] != "Bearer secret-abc" || headers[mcpCatalogHeader] != "bbbb" {
			t.Fatalf("headers = %#v", headers)
		}
	})
}

func TestHTTPDryRunWritesNothing(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		req := testRequest(t)
		req.MCPTransport, req.MCPURL, req.DryRun = "http", "http://127.0.0.1:47831/mcp", true
		if _, err := tc.installer().Install(req); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(tc.userMCPPath(req)); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote the %s user config: %v", tc.spec.DisplayName, err)
		}
	})
}
