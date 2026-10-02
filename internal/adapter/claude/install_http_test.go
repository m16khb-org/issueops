package claude

import (
	"encoding/json"
	install "issueops/internal/adapter/install"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readClaudeJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(readClaudeTestFile(t, path)), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestClaudeHTTPInstallMergesOnlyIssueOpsEntryOwnerOnly(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	userPath := filepath.Join(home, ".claude.json")
	writeClaudeTestFile(t, userPath, `{"numStartups":3,"mcpServers":{"other":{"type":"stdio","command":"other"},"issueops":{"type":"stdio","command":"/old/issueops","args":["mcp"],"env":{"ISSUEOPS_ROOT":"/old"}}}}`)
	projectPath := filepath.Join(root, ".mcp.json")
	writeClaudeTestFile(t, projectPath, `{"mcpServers":{"keep":{"command":"keep"},"issueops_project":{"type":"stdio","command":"./bin/issueops","args":["mcp"]}}}`)
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	req.ProjectLocal = true
	req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"

	if result, err := testInstaller().Install(req); err != nil || !result.OK {
		t.Fatalf("install err=%v result=%+v", err, result)
	}
	user := readClaudeJSON(t, userPath)
	want := map[string]any{
		"numStartups": float64(3),
		"mcpServers": map[string]any{
			"other":    map[string]any{"type": "stdio", "command": "other"},
			"issueops": map[string]any{"type": "http", "url": "http://127.0.0.1:47831/mcp", "headers": map[string]any{"Authorization": "Bearer secret-abc"}},
		},
	}
	if !reflect.DeepEqual(user, want) {
		t.Fatalf("user config = %#v", user)
	}
	if info, err := os.Stat(userPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("user config mode = %v, %v", info.Mode().Perm(), err)
	}
	project := readClaudeJSON(t, projectPath)
	if !reflect.DeepEqual(project, map[string]any{"mcpServers": map[string]any{"keep": map[string]any{"command": "keep"}}}) {
		t.Fatalf("project config must drop only the duplicate issueops entry: %#v", project)
	}
	if _, err := testInstaller().VerifyActivation(req); err != nil {
		t.Fatalf("http activation readback: %v", err)
	}
}

func TestClaudeHTTPDryRunWritesNothing(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	req.ProjectLocal = true
	req.MCPTransport, req.MCPURL, req.DryRun = "http", "http://127.0.0.1:47831/mcp", true
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(home, ".claude.json"), filepath.Join(home, ".claude"), filepath.Join(root, ".mcp.json"), filepath.Join(root, "configs")} {
		if exists(path) {
			t.Fatalf("dry-run wrote %s", path)
		}
	}
}
