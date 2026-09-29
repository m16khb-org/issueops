package issueopsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/projectcli"
	docsapp "issueops/internal/application/projectdocs"
	docscontract "issueops/internal/contract/projectdocs"
)

func TestProjectDocMCPInstancesKeepDefaultRootsSeparate(t *testing.T) {
	var deps [2]mcpcli.MCPDependencies
	var sessions [2]*mcp.ClientSession
	var roots [2]string
	for i := range deps {
		roots[i] = t.TempDir()
		if err := os.MkdirAll(filepath.Join(roots[i], ".issueops"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(roots[i], ".issueops", "TESTING.md"), []byte(fmt.Sprintf("# owner %d\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISSUEOPS_ROOT", t.TempDir())
		t.Setenv("CLAUDE_PROJECT_DIR", roots[i])
		t.Setenv("PWD", t.TempDir())
		deps[i] = issueOpsMCPDependencies()
		sessions[i] = startHistoryMCPTestSession(t, deps[i])
	}
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	t.Setenv("PWD", t.TempDir())
	var wg sync.WaitGroup
	for i := range deps {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 3; n++ {
				read, err := projectDocsDirectRead(deps[i])
				if err != nil {
					t.Error(err)
					return
				}
				if read.RepoRoot != roots[i] || !read.Exists {
					t.Errorf("instance %d read=%+v", i, read)
					return
				}
				want := fmt.Sprintf("# owner %d revision %d\n", i, n)
				result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_revise", Arguments: map[string]any{"rel_path": ".issueops/TESTING.md", "content": want, "expected_sha256": read.SHA256, "summary": "verified revision", "confirm": true}})
				if err != nil || result.IsError {
					t.Errorf("revise %d: %v %+v", i, err, result)
					return
				}
				after, err := projectDocsDirectRead(deps[i])
				if err != nil || after.Content != want || after.RepoRoot != roots[i] {
					t.Errorf("after %d: %+v %v", i, after, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	for i, root := range roots {
		raw, err := os.ReadFile(filepath.Join(root, ".issueops", "TESTING.md"))
		if err != nil || string(raw) != fmt.Sprintf("# owner %d revision 2\n", i) {
			t.Errorf("file %d: %s %v", i, raw, err)
		}
	}
}

func TestProjectDocMCPRefusalsPreserveFileBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".issueops"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".issueops", "TESTING.md")
	const original = "# original\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	deps := issueOpsMCPDependencies()
	session := startHistoryMCPTestSession(t, deps)
	for _, args := range []map[string]any{
		{"repo": root, "rel_path": ".issueops/TESTING.md", "content": "changed", "expected_sha256": strings.Repeat("0", 64), "summary": "stale", "confirm": true},
		{"repo": root, "rel_path": "../TESTING.md", "content": "changed", "summary": "escape", "confirm": true},
		{"repo": root, "rel_path": ".issueops/TESTING.md", "content": "changed", "summary": "missing digest", "confirm": true},
	} {
		params, _ := json.Marshal(map[string]any{"name": "project_docs_revise", "arguments": args})
		if _, err := mcpcli.HandleToolCallWithDependencies(params, deps); err == nil {
			t.Errorf("direct accepted %+v", args)
		}
		if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_revise", Arguments: args}); err == nil {
			t.Errorf("SDK accepted %+v", args)
		}
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != original {
			t.Fatalf("refusal changed bytes: %s %v", raw, err)
		}
	}
}

func projectDocsDirectRead(deps mcpcli.MCPDependencies) (docscontract.ProjectDocsReadResult, error) {
	result, rpcErr := mcpcli.HandleToolCallWithDependencies(json.RawMessage(`{"name":"project_docs_read","arguments":{"rel_path":".issueops/TESTING.md"}}`), deps)
	if rpcErr != nil {
		return docscontract.ProjectDocsReadResult{}, rpcErr
	}
	text := result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
	var read docscontract.ProjectDocsReadResult
	err := json.Unmarshal([]byte(text), &read)
	return read, err
}

func TestProjectDocCLIInstancesPinRelativeRootsAndRejectInvalidAppend(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	t.Chdir(roots[0])
	first := newProjectDocsService(".")
	t.Chdir(roots[1])
	second := newProjectDocsService(".")
	t.Chdir(t.TempDir())
	for i, service := range []docsapp.Service{first, second} {
		if err := projectcli.Run(service, []string{"append", "--kind", "invalid", "--title", "Title", "--summary", "Summary"}); err == nil {
			t.Fatal("invalid append accepted")
		}
		if _, err := os.Stat(filepath.Join(roots[i], ".issueops")); !os.IsNotExist(err) {
			t.Fatalf("invalid append created docs: %v", err)
		}
		routeJSON := captureStdoutForContract(t, func() error { return projectcli.Run(service, []string{"route-docs", "--task", "test", "--json"}) })
		var route docscontract.ProjectDocsRouteResult
		if err := json.Unmarshal([]byte(routeJSON), &route); err != nil || route.RepoRoot != roots[i] {
			t.Fatalf("route %d: %+v %v", i, route, err)
		}
		raw := captureStdoutForContract(t, func() error {
			return projectcli.Run(service, []string{"append", "--kind", "adr", "--title", "Instance decision", "--summary", fmt.Sprintf("owner-%d", i), "--json"})
		})
		var appended docscontract.ProjectDocsAppendResult
		if err := json.Unmarshal([]byte(raw), &appended); err != nil || appended.RepoRoot != roots[i] {
			t.Fatalf("append %d: %+v %v", i, appended, err)
		}
		content, err := os.ReadFile(appended.Path)
		if err != nil || !strings.Contains(string(content), fmt.Sprintf("owner-%d", i)) || len(content) != appended.BytesAppended {
			t.Fatalf("append bytes: %s %v", content, err)
		}
	}
}

func TestProjectDocMCPDryRunAndExplicitRepositoryPreserveTargets(t *testing.T) {
	first, other := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", first)
	deps := issueOpsMCPDependencies()
	session := startHistoryMCPTestSession(t, deps)
	read, err := projectDocsDirectRead(deps)
	if err != nil || read.Exists {
		t.Fatalf("missing read=%+v %v", read, err)
	}
	for _, root := range []string{first, other} {
		args := map[string]any{"repo": root, "rel_path": ".issueops/TESTING.md", "content": "# Testing\n", "summary": "create"}
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_revise", Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("dry run=%+v %v", result, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".issueops")); !os.IsNotExist(err) {
			t.Fatalf("dry run wrote docs: %v", err)
		}
		args["confirm"] = true
		result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_revise", Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("create=%+v %v", result, err)
		}
		raw, err := os.ReadFile(filepath.Join(root, ".issueops", "TESTING.md"))
		if err != nil || !strings.Contains(string(raw), "# Testing") {
			t.Fatalf("create bytes: %s %v", raw, err)
		}
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_append", Arguments: map[string]any{"repo": other, "kind": "caution", "title": "Instance target", "summary": "explicit-other"}})
	if err != nil || result.IsError {
		t.Fatalf("append=%+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(first, ".issueops", "cautions")); !os.IsNotExist(err) {
		t.Fatalf("append leaked to default root: %v", err)
	}
	files, err := os.ReadDir(filepath.Join(other, ".issueops", "cautions"))
	if err != nil || len(files) != 1 {
		t.Fatalf("append files: %+v %v", files, err)
	}
}
