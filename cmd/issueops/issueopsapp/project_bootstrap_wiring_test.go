package issueopsapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/projectcli"
	bootstrapcontract "issueops/internal/contract/projectbootstrap"
	"strings"
)

func TestProjectBootstrapMCPInstancesKeepRepositoryAndStateSeparate(t *testing.T) {
	var dependencies [2]mcpcli.MCPDependencies
	var sessions [2]*mcp.ClientSession
	var roots, states [2]string
	for i := range dependencies {
		roots[i] = t.TempDir()
		states[i] = filepath.Join(t.TempDir(), "uncreated-state")
		t.Setenv("CLAUDE_PROJECT_DIR", roots[i])
		t.Setenv("ISSUEOPS_STATE_DIR", states[i])
		dependencies[i] = issueOpsMCPDependencies()
		sessions[i] = startHistoryMCPTestSession(t, dependencies[i])
	}
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(t.TempDir(), "wrong-state"))
	var wg sync.WaitGroup
	for i := range dependencies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 2; n++ {
				params := json.RawMessage(`{"name":"project_docs_bootstrap_plan","arguments":{}}`)
				result, callErr := callSDKTool(t, params, dependencies[i])
				if callErr != nil {
					t.Error(callErr)
					return
				}
				text := result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
				var direct bootstrapcontract.ProjectDocsBootstrapResult
				if err := json.Unmarshal([]byte(text), &direct); err != nil {
					t.Error(err)
					return
				}
				sdk, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "project_docs_bootstrap_plan", Arguments: map[string]any{}})
				if err != nil {
					t.Error(err)
					return
				}
				if sdk.IsError {
					t.Errorf("bootstrap failed: %+v", sdk)
					return
				}
				var server bootstrapcontract.ProjectDocsBootstrapResult
				if err := json.Unmarshal([]byte(sdk.Content[0].(*mcp.TextContent).Text), &server); err != nil {
					t.Error(err)
					return
				}
				for _, plan := range []bootstrapcontract.ProjectDocsBootstrapResult{direct, server} {
					if !plan.OK || !plan.DryRun || plan.RepoRoot != roots[i] || plan.LifecycleState.StateRoot != states[i] {
						t.Errorf("instance %d wrong plan: root=%s state=%s ok=%v dry=%v", i, plan.RepoRoot, plan.LifecycleState.StateRoot, plan.OK, plan.DryRun)
					}
				}
			}
		}(i)
	}
	wg.Wait()
	for i := range roots {
		for _, path := range []string{filepath.Join(roots[i], ".issueops"), states[i]} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("dry-run materialized %s: %v", path, err)
			}
		}
	}
}

func TestProjectBootstrapCLIPinsStateAndPreservesCuratedDocs(t *testing.T) {
	root, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	t.Setenv("ISSUEOPS_STATE_DIR", state)
	service := newProjectBootstrapService(root)
	otherState := filepath.Join(t.TempDir(), "other-state")
	t.Setenv("ISSUEOPS_STATE_DIR", otherState)
	run := func(args ...string) bootstrapcontract.ProjectDocsBootstrapResult {
		t.Helper()
		raw := captureStdoutForContract(t, func() error { return projectcli.RunBootstrap(service, args) })
		var result bootstrapcontract.ProjectDocsBootstrapResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	dry := run("--repo", root, "--dry-run", "--json")
	if !dry.DryRun || dry.LifecycleState.StateRoot != state {
		t.Fatalf("wrong dry run: %+v", dry)
	}
	for _, path := range []string{state, filepath.Join(root, ".issueops")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s: %v", path, err)
		}
	}
	written := run("--repo", root, "--json")
	if !written.Write || !written.LifecycleState.NamespaceValid || written.LifecycleState.StateRoot != state {
		t.Fatalf("invalid bootstrap: %+v", written)
	}
	if _, err := os.Stat(written.LifecycleState.ProjectJSONPath); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(root, ".issueops", "TECH_STACK.md"), filepath.Join(root, ".issueops", "ARCHITECTURE.md")}
	curated := map[string]string{}
	for _, path := range paths {
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		end := strings.Index(string(original), "\n---\n")
		if end < 0 {
			t.Fatalf("missing template metadata: %s", path)
		}
		curated[path] = string(original[:end+5]) + "\n# Curated\nKeep these decisions.\n"
		if err := os.WriteFile(path, []byte(curated[path]), 0600); err != nil {
			t.Fatal(err)
		}
	}

	run("--repo", root, "--json")
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != curated[path] {
			t.Fatalf("unsynced overwrite %s: %s %v", path, raw, err)
		}
	}
	run("--repo", root, "--sync", "--json")
	family, err := os.ReadFile(paths[1])
	if err != nil || string(family) != curated[paths[1]] {
		t.Fatalf("family overwritten: %s %v", family, err)
	}
	stack, err := os.ReadFile(paths[0])
	if err != nil || !strings.Contains(string(stack), "TECH_STACK.md") || string(stack) == curated[paths[0]] {
		t.Fatalf("non-family not refreshed: %s %v", stack, err)
	}
	if _, err := os.Stat(otherState); !os.IsNotExist(err) {
		t.Fatalf("other state created: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := projectcli.RunBootstrap(service, []string{"--repo", missing, "--json"}); err == nil {
		t.Fatal("missing repository accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("invalid repo created: %v", err)
	}
}
