package omp

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOmpHTTPInstallMergesOnlyIssueOpsEntryOwnerOnly(t *testing.T) {
	req := ompTestRequest(t)
	userPath := filepath.Join(req.Home, ".omp", "agent", "mcp.json")
	writeOmpTestJSON(t, userPath, map[string]any{"mcpServers": map[string]any{
		"other":    map[string]any{"command": "other"},
		"issueops": map[string]any{"command": "/old/issueops", "args": []any{"mcp"}, "env": map[string]any{"ISSUEOPS_ROOT": "/old"}},
	}})
	if err := os.Chmod(userPath, 0o644); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(req.Root, ".omp", "mcp.json")
	writeOmpTestJSON(t, projectPath, map[string]any{"mcpServers": map[string]any{
		"keep":             map[string]any{"command": "keep"},
		"issueops_project": map[string]any{"command": "./bin/issueops", "args": []any{"mcp"}},
	}})
	req.ProjectLocal = true
	req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"

	if result, err := testInstaller().Install(req); err != nil || !result.OK {
		t.Fatalf("install err=%v result=%+v", err, result)
	}
	catalog, err := testDependencies().MCPCatalogSHA256()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"mcpServers": map[string]any{
		"other": map[string]any{"command": "other"},
		"issueops": map[string]any{"type": "http", "url": "http://127.0.0.1:47831/mcp", "headers": map[string]any{
			"Authorization": "Bearer secret-abc", ompMCPCatalogHeader: catalog,
		}},
	}}
	if got := readOmpTestJSON(t, userPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("user config = %#v", got)
	}
	if info, err := os.Stat(userPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("user config mode = %v, %v", info.Mode().Perm(), err)
	}
	if got := readOmpTestJSON(t, projectPath); !reflect.DeepEqual(got, map[string]any{"mcpServers": map[string]any{"keep": map[string]any{"command": "keep"}}}) {
		t.Fatalf("project config must drop only the duplicate issueops entry: %#v", got)
	}
	if _, err := testInstaller().VerifyActivation(req); err != nil {
		t.Fatalf("http activation readback: %v", err)
	}
}

// omp keys its tool-catalog cache on the whole server config, so the HTTP
// entry must change when the advertised catalog does; otherwise new sessions
// reuse a stale tools/list for up to the cache TTL.
func TestOmpHTTPEntryChangesWithTheAdvertisedCatalog(t *testing.T) {
	entry := func(digest string) map[string]any {
		deps := testDependencies()
		deps.MCPCatalogSHA256 = func() (string, error) { return digest, nil }
		req := ompTestRequest(t)
		req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"
		server, err := NewInstaller(deps, nil).ompUserMCPServer(req)
		if err != nil {
			t.Fatal(err)
		}
		return server
	}
	first, second := entry("aaaa"), entry("bbbb")
	if reflect.DeepEqual(first, second) {
		t.Fatalf("catalog change left the omp HTTP entry unchanged: %#v", first)
	}
	headers, _ := second["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer secret-abc" || headers[ompMCPCatalogHeader] != "bbbb" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestOmpHTTPDryRunWritesNothing(t *testing.T) {
	req := ompTestRequest(t)
	req.MCPTransport, req.MCPURL, req.DryRun = "http", "http://127.0.0.1:47831/mcp", true
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(req.Home, ".omp", "agent", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote the omp user config: %v", err)
	}
}
