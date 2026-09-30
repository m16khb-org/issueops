package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli/resources"
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
	preflightapp "issueops/internal/application/preflight"
	docsmodel "issueops/internal/contract/docs"
	inspectmodel "issueops/internal/contract/inspect"
	model "issueops/internal/contract/issueops"
	webmodel "issueops/internal/contract/webfetch"
	"issueops/internal/port"
)

type instanceAssistantEffects struct{ owner string }

func (e instanceAssistantEffects) NormalizeRoot(root string) (string, error) {
	if root != e.owner {
		return "", fmt.Errorf("crossed root: got %s want %s", root, e.owner)
	}
	return root, nil
}
func (e instanceAssistantEffects) Diff(root string, staged bool) (string, error) { return e.owner, nil }
func (e instanceAssistantEffects) Run(root string, argv []string) (string, int, bool) {
	return e.owner, 2, true
}

type instancePreflight struct{ owner string }

func (e instancePreflight) Observe(target, root string) preflightapp.Observation {
	return preflightapp.Observation{GitOK: target == e.owner && root == e.owner, RepoRoot: e.owner}
}

func TestMCPDirectAndSDKKeepProjectAndExecutionServicesIsolated(t *testing.T) {
	owners := []string{"/first-instance", "/second-instance"}
	deps := make([]MCPDependencies, 2)
	sessions := make([]*mcp.ClientSession, 2)
	for i, owner := range owners {
		pid := 100 + i
		deps[i] = MCPDependencies{Catalog: testMCPCatalog(), DefaultTarget: owner,
			Inspect:   func(repo string) any { return map[string]any{"owner": owner} },
			Preflight: preflightapp.Service{Observer: instancePreflight{owner}},
			Skills: func(root, name string) []inspectmodel.SkillInfo {
				if name != owner {
					return nil
				}
				return []inspectmodel.SkillInfo{{Name: name, Path: root}}
			},
			Compatibility: func() any { return map[string]any{"owner": owner} },
			Commit:        commitapp.Service{Effects: instanceAssistantEffects{owner}}, Lint: lintapp.Service{Effects: instanceAssistantEffects{owner}},
			Fetch: func(context.Context, webmodel.Request) (webmodel.Result, error) {
				return webmodel.Result{OK: true, URL: owner}, nil
			},
			Resources: resources.Config{IssueOpsRoot: owner, Version: owner, SkillName: owner,
				ReadHarnessFile: func(parts ...string) (string, error) { return owner + "/" + strings.Join(parts, "/"), nil },
				DocsIndex: func(root, version string) docsmodel.DocsIndexResult {
					return docsmodel.DocsIndexResult{OK: root == owner && version == owner, IssueOpsRoot: root, Version: version}
				},
			},
			Execution: ExecutionDeps{IssueOpsStateRoot: func() string { return owner },
				ObserveNativeProcessAncestry: func(int) ([]model.NativeProcessReceipt, error) { return []model.NativeProcessReceipt{{PID: pid}}, nil },
				ExecuteExecution: func(ctx context.Context, root string, req model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
					if root != owner || len(req.Actor.ProcessAncestry) != 1 || req.Actor.ProcessAncestry[0].PID != pid {
						return nil, fmt.Errorf("crossed execution dependencies")
					}
					return map[string]any{"owner": owner}, nil
				},
			},
		}
		sessions[i] = startMCPTransportTestSession(t, "stdio", deps[i])
	}
	calls := []MCPToolCall{
		{Name: "harness_inspect", Arguments: map[string]any{}},
		{Name: "atomic_commit_preflight", Arguments: map[string]any{}},
		{Name: "commit_policy", Arguments: map[string]any{}},
		{Name: "skill_manifest", Arguments: map[string]any{}},
		{Name: "docs_index", Arguments: map[string]any{}},
		{Name: "contract_schema", Arguments: map[string]any{}},
		{Name: "commit_suggest", Arguments: map[string]any{}},
		{Name: "lint_diagnose", Arguments: map[string]any{"command_argv": []string{"check"}}},
		{Name: "web_fetch_resilient", Arguments: map[string]any{"url": "https://example.test"}},
		{Name: "issueops_execution", Arguments: map[string]any{"action": "status", "id": "io-example"}},
	}
	for i, owner := range owners {
		t.Run(owner, func(t *testing.T) {
			for _, call := range calls {
				raw, err := json.Marshal(map[string]any{"name": call.Name, "arguments": call.Arguments})
				if err != nil {
					t.Fatal(err)
				}
				direct, rpcErr := HandleToolCallWithDependencies(raw, deps[i])
				if rpcErr != nil {
					t.Fatalf("%s direct: %v", call.Name, rpcErr)
				}
				encoded, _ := json.Marshal(direct)
				if strings.Contains(string(encoded), `"ok":false`) || !strings.Contains(string(encoded), owner) || strings.Contains(string(encoded), owners[1-i]) {
					t.Fatalf("%s direct=%s", call.Name, encoded)
				}
				result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: call.Name, Arguments: call.Arguments})
				if err != nil || result.IsError {
					t.Fatalf("%s SDK result=%+v err=%v", call.Name, result, err)
				}
				text := toolResultText(result)
				if strings.Contains(text, `"ok": false`) || !strings.Contains(text, owner) || strings.Contains(text, owners[1-i]) {
					t.Fatalf("%s SDK=%s", call.Name, text)
				}
			}
		})
	}
}
