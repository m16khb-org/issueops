package issueopsapp

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"testing"

	auditcontract "issueops/internal/contract/audit"
	policycontract "issueops/internal/contract/policy"
	"issueops/internal/testsupport"
)

func capturePolicyInstance() func([]string) error {
	return newPolicyCommand().Run
}

func TestPolicyCommandsKeepCapturedWorkspaceAndAuditPath(t *testing.T) {
	type instance struct {
		root, audit string
		run         func([]string) error
	}
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	var instances []instance
	for range 2 {
		root := t.TempDir()
		audit := filepath.Join(root, "audit.jsonl")
		t.Setenv("PWD", root)
		t.Setenv("ISSUEOPS_AUDIT_LOG", audit)
		t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(root, "state"))
		instances = append(instances, instance{root, audit, capturePolicyInstance()})
	}
	ambient := t.TempDir()
	t.Setenv("PWD", ambient)
	t.Setenv("ISSUEOPS_AUDIT_LOG", filepath.Join(ambient, "audit.jsonl"))
	for _, item := range instances {
		out := testsupport.CaptureStdout(t, func() error { return item.run([]string{"check", "--json", "--", "git", "status"}) })
		var result policycontract.CommandPolicyEvaluation
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		if result.WorkspaceRoot != item.root || result.CWD != item.root || !result.Allowed {
			t.Fatalf("captured workspace lost: root=%s result=%+v", item.root, result)
		}
		out = testsupport.CaptureStdout(t, func() error { return item.run([]string{"audit", "--json", "--", "git", "status"}) })
		var record auditcontract.CommandAuditRecord
		if err := json.Unmarshal([]byte(out), &record); err != nil {
			t.Fatal(err)
		}
		if record.LogPath != item.audit || record.Policy.WorkspaceRoot != item.root {
			t.Fatalf("captured audit context lost: %+v", record)
		}
		if _, err := os.Stat(item.audit); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(ambient, "audit.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("ambient audit path touched: %v", err)
	}
}

func TestPolicyMCPInstancesKeepStateAndReloadOverrides(t *testing.T) {
	repo := makeGitRepoForContract(t)
	policyFile := filepath.Join(repo, ".issueops", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyFile), 0700); err != nil {
		t.Fatal(err)
	}
	var deps [2]mcpcli.MCPDependencies
	var clients [2]*mcp.ClientSession
	var audits [2]string
	for i, base := range []string{"first-parent", "second-parent"} {
		state := t.TempDir()
		t.Setenv("ISSUEOPS_STATE_DIR", state)
		t.Setenv("ISSUEOPS_AUDIT_LOG", filepath.Join(state, "command.jsonl"))
		record, err := startIssueOpsFixture(issueOpsStateRoot(), model.IssueOpsStartRequest{Repo: repo, Branch: "79-child"})
		if err != nil {
			t.Fatal(err)
		}
		record.IssueURL = "https://github.com/acme/repo/issues/79"
		record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: base, LinkVerified: true, CreatedAt: record.CreatedAt}
		if _, err = core.WriteIssueOps(issueOpsStateRoot(), record); err != nil {
			t.Fatal(err)
		}
		deps[i] = issueOpsMCPDependencies()
		clients[i] = startHistoryMCPTestSession(t, deps[i])
		audits[i] = filepath.Join(state, "command.jsonl")
	}
	ambient := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", ambient)
	t.Setenv("ISSUEOPS_AUDIT_LOG", filepath.Join(ambient, "command.jsonl"))
	invoke := func(i int, sdk bool, name string, args map[string]any) string {
		t.Helper()
		if sdk {
			result, err := clients[i].CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("SDK tool failure: %+v", result)
			}
			return result.Content[0].(*mcp.TextContent).Text
		}
		raw, err := json.Marshal(map[string]any{"name": name, "arguments": args})
		if err != nil {
			t.Fatal(err)
		}
		result, protocolErr := mcpcli.HandleToolCallWithDependencies(raw, deps[i])
		if protocolErr != nil {
			t.Fatal(protocolErr)
		}
		return result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
	}
	for _, sdk := range []bool{false, true} {
		for i := range deps {
			args := map[string]any{"workspace_root": repo, "cwd": repo, "argv": []string{"glab", "mr", "create", "--target-branch", "first-parent"}, "write_allowed": true, "network_allowed": true}
			for _, name := range []string{"command_policy_check", "command_fake_run", "command_policy_audit"} {
				raw := invoke(i, sdk, name, args)
				var policy policycontract.CommandPolicyEvaluation
				if name == "command_policy_check" {
					if err := json.Unmarshal([]byte(raw), &policy); err != nil {
						t.Fatal(err)
					}
				} else {
					var envelope struct {
						Policy   policycontract.CommandPolicyEvaluation `json:"policy"`
						LogPath  string                                 `json:"log_path"`
						Executed bool                                   `json:"executed"`
					}
					if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
						t.Fatal(err)
					}
					policy = envelope.Policy
					if envelope.Executed {
						t.Fatal("policy observation executed a provider command")
					}
					if name == "command_policy_audit" && envelope.LogPath != audits[i] {
						t.Fatalf("wrong audit path: %+v", envelope)
					}
				}
				mismatch := containsString(policy.DenyReasons, "pr_target_branch_mismatch")
				if mismatch != (i == 1) || policy.Allowed != (i == 0) {
					t.Fatalf("captured state lost: sdk=%v instance=%d tool=%s result=%+v", sdk, i, name, policy)
				}
			}
			args = map[string]any{"workspace_root": repo, "cwd": repo, "argv": []string{"repo-tool"}}
			for _, content := range []string{`{"additional_read_only_commands":["repo-tool"]}`, `{broken`} {
				if err := os.WriteFile(policyFile, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				raw := invoke(i, sdk, "command_policy_check", args)
				var policy policycontract.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(raw), &policy); err != nil {
					t.Fatal(err)
				}
				expected := content != "{broken"
				if policy.Allowed != expected || (!expected && len(policy.Warnings) == 0) {
					t.Fatalf("override not reread: sdk=%v policy=%+v", sdk, policy)
				}
			}
			if err := os.Remove(policyFile); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(ambient, "command.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("ambient audit touched: %v", err)
	}
}
