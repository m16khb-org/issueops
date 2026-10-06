package issueopsapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	issueopscore "issueops/internal/adapter/issueops"
	statestore "issueops/internal/adapter/outbound/state"
	model "issueops/internal/contract/issueops"
)

func chdirSharedServer(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

func requireEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("server cwd %s received entries %v", dir, names)
	}
}

func TestMCPHTTPGatesDefaultFilesResolveAgainstRequestCWD(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	serverCWD := chdirSharedServer(t)
	repoA := gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grantA := authorizeSessionForTest(t, repoA, "client-a", self)
	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	client := connectHTTPClient(t, url, bearer, "")

	payload, isError := callHTTPTool(t, client, "gates_init", map[string]any{
		"authority_file": grantA, "workspace_root": repoA, "scope": "default probe", "gates": []any{"G1: probe | CHECK: true | EXPECT: ok"},
	})
	if isError {
		t.Fatalf("gates_init without file payload=%v", payload)
	}
	if _, err := os.Stat(filepath.Join(repoA, ".issueops", "gates", "default-probe.md")); err != nil {
		t.Fatalf("default ledger is not in the request workspace: %v (payload=%v)", err, payload)
	}

	ledgerPath := filepath.Join(repoA, ".issueops", "gates", "default-probe.md")
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	payload, isError = callHTTPTool(t, client, "gates_abandon", map[string]any{
		"authority_file": grantA, "workspace_root": repoA, "file": ".issueops/gates/default-probe.md", "gate_id": "G1", "reason": "probe",
	})
	if isError {
		t.Fatalf("gates_abandon with a relative file payload=%v", payload)
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil || string(data) == string(ledger) {
		t.Fatalf("request ledger not updated: %v %q", err, data)
	}
	requireEmptyDir(t, serverCWD)
}

func TestMCPHTTPGatesPathsStayInsideAuthorizedWorkspace(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	serverCWD := chdirSharedServer(t)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	secret := "# Gates: outside\n\n- [ ] G1: outside secret\n  EVIDENCE: pending\n"
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}
	repoA, repoB := gitRepoForHTTPTest(t), gitRepoForHTTPTest(t)
	if err := os.Symlink(outside, filepath.Join(repoA, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoA, ".issueops", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(repoA, ".issueops", "gates", "linked.md")); err != nil {
		t.Fatal(err)
	}
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grantA := authorizeSessionForTest(t, repoA, "client-a", self)
	grantB := authorizeSessionForTest(t, repoB, "client-b", self)
	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	clientA := connectHTTPClient(t, url, bearer, "")
	clientB := connectHTTPClient(t, url, bearer, "")
	gate := []any{"G1: probe | CHECK: true | EXPECT: ok"}

	for _, file := range []string{
		filepath.Join(outside, "absolute.md"),
		"../" + filepath.Base(outside) + "/dotdot.md",
		filepath.Join(serverCWD, "server.md"),
		"escape/through-symlink.md",
		"escape/new/dir/leaf.md",
	} {
		payload, isError := callHTTPTool(t, clientA, "gates_init", map[string]any{"authority_file": grantA, "workspace_root": repoA, "scope": "escape", "file": file, "gates": gate})
		if !isError {
			t.Fatalf("gates_init %q escaped the workspace: %v", file, payload)
		}
	}
	payload, isError := callHTTPTool(t, clientB, "gates_init", map[string]any{"authority_file": grantB, "workspace_root": repoB, "scope": "cross", "file": filepath.Join(repoA, "cross.md"), "gates": gate})
	if !isError {
		t.Fatalf("client B wrote into workspace A: %v", payload)
	}
	payload, isError = callHTTPTool(t, clientA, "gates_init", map[string]any{"authority_file": grantA, "workspace_root": repoA, "scope": "nested", "file": "new/dir/leaf.md", "gates": gate})
	if isError {
		t.Fatalf("nonexistent in-workspace leaf rejected: %v", payload)
	}
	if _, err := os.Stat(filepath.Join(repoA, "new", "dir", "leaf.md")); err != nil {
		t.Fatalf("nested ledger missing: %v", err)
	}

	payload, isError = callHTTPTool(t, clientA, "gates_status", map[string]any{"authority_file": grantA, "workspace_root": repoA, "files": []any{filepath.Join(outside, "secret.md")}})
	if encoded := payloadText(payload); !isError && strings.Contains(encoded, "outside secret") {
		t.Fatalf("gates_status read an absolute file outside the workspace: %s", encoded)
	}
	payload, _ = callHTTPTool(t, clientA, "gates_status", map[string]any{"authority_file": grantA, "workspace_root": repoA})
	if encoded := payloadText(payload); strings.Contains(encoded, "outside secret") {
		t.Fatalf("gates discovery followed a symlink out of the workspace: %s", encoded)
	}
	payload, _ = callHTTPTool(t, clientA, "gates_abandon", map[string]any{"authority_file": grantA, "workspace_root": repoA, "file": filepath.Join(outside, "secret.md"), "gate_id": "G1", "reason": "escape"})
	if data, err := os.ReadFile(filepath.Join(outside, "secret.md")); err != nil || string(data) != secret {
		t.Fatalf("gates_abandon rewrote a file outside the workspace: %v %q payload=%v", err, data, payload)
	}
	for _, name := range []string{"absolute.md", "dotdot.md", "through-symlink.md", "new"} {
		if _, err := os.Lstat(filepath.Join(outside, name)); err == nil {
			t.Fatalf("outside directory received %s", name)
		}
	}
	requireEmptyDir(t, serverCWD)
}

func TestMCPHTTPBoundLoopAndWorkerWritesSucceed(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	chdirSharedServer(t)
	repo := gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grant := authorizeSessionForTest(t, repo, "client-a", self)
	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	client := connectHTTPClient(t, url, bearer, "")
	started, isError := callHTTPTool(t, client, "loop_start", map[string]any{"authority_file": grant, "repo": repo, "name": "http-fence", "goal": "bound loop write"})
	if isError {
		t.Fatalf("bound loop_start payload=%v", started)
	}
	attempt, isError := callHTTPTool(t, client, "loop_record_attempt", map[string]any{"authority_file": grant, "id": started["id"], "verdict": "pass", "evidence": []any{"http"}})
	if isError {
		t.Fatalf("bound loop_record_attempt payload=%v", attempt)
	}
	job, isError := callHTTPTool(t, client, "worker_run_read_only", map[string]any{"authority_file": grant, "workspace_root": repo, "cwd": repo, "kind": "probe", "argv": []any{"git", "status", "--short"}})
	if isError || job["status"] == "running" {
		t.Fatalf("bound worker_run_read_only payload=%v", job)
	}
}

func payloadText(payload map[string]any) string {
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func TestMCPHTTPCompletedReplacePreviewUsesProductionBaseSync(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-drift", true: "drift"}[drift], func(t *testing.T) {
			t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
			fixtureRoot, record, _, owner := completedReplacementPreviewFixture(t, drift)
			stored, err := issueopscore.ReadIssueOps(fixtureRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (issueopscore.CycleRecordStore{StateRoot: issueOpsStateRoot()}).Save(context.Background(), stored); err != nil {
				t.Fatal(err)
			}
			self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			grant := authorizeSessionForTest(t, record.Execution.Workspace.SourceRoot, "completed-preview", self)
			deps := issueOpsMCPHTTPDependencies()
			if deps.BaseSync == nil {
				t.Fatal("MCP dependencies lack the production base sync inspector")
			}
			deps.OrcaOwner = owner
			deps.ForRequest = issueOpsMCPRequestDependencies(deps)
			url, _ := startProductionHTTPServer(t, deps)
			bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
			if err != nil {
				t.Fatal(err)
			}
			payload, isError := callHTTPTool(t, connectHTTPClient(t, url, bearer, ""), "issueops_execution", map[string]any{
				"action": model.ExecutionActionReplace, "replace_action": model.ExecutionReplacePreview, "id": record.ID,
				"expected_generation": 1, "completion_generation": 1, "cwd": record.Execution.Workspace.Root, "authority_file": grant,
			})
			if drift {
				if !isError || payload["code"] != "post_completion_sync_base_required" {
					t.Fatalf("drifted completed preview payload=%v isError=%v", payload, isError)
				}
				return
			}
			next, _ := payload["next_command"].(string)
			if isError || !strings.Contains(next, "--reseed") || !strings.Contains(next, "--completion-generation 1") {
				t.Fatalf("completed preview payload=%v isError=%v", payload, isError)
			}
		})
	}
}
