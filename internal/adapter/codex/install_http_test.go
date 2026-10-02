package codex

import (
	install "issueops/internal/adapter/install"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func httpRequestFixture(t *testing.T) (string, string) {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	return root, home
}

func TestCodexHTTPInstallReplacesStdioEntryWithOwnerOnlyURLAndBearer(t *testing.T) {
	root, home := httpRequestFixture(t)
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "model = \"x\"\n\n[mcp_servers.other]\ncommand = \"other\"\n\n[mcp_servers.issueops]\ncommand = \"/old/issueops\"\nargs = [\"mcp\"]\nstartup_timeout_sec = 30\n\n[mcp_servers.issueops.env]\nISSUEOPS_ROOT = \"/old\"\n"
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-abc"

	result, err := testInstaller().Install(req)
	if err != nil || !result.OK {
		t.Fatalf("install err=%v result=%+v", err, result)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "model = \"x\"\n\n[mcp_servers.other]\ncommand = \"other\"\n\n[mcp_servers.issueops]\nurl = \"http://127.0.0.1:47831/mcp\"\nhttp_headers = { Authorization = \"Bearer secret-abc\" }\n"
	if string(raw) != want {
		t.Fatalf("config =\n%s\nwant\n%s", raw, want)
	}
	info, err := os.Stat(configPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, %v", info.Mode().Perm(), err)
	}
	if _, err := testInstaller().VerifyActivation(req); err != nil {
		t.Fatalf("http activation readback: %v", err)
	}
	req.MCPBearer = "rotated"
	if _, err := testInstaller().VerifyActivation(req); err == nil || strings.Contains(err.Error(), "rotated") {
		t.Fatalf("readback accepted a different bearer or leaked it: %v", err)
	}
}

func TestCodexHTTPDryRunWritesNothing(t *testing.T) {
	root, home := httpRequestFixture(t)
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	req.MCPTransport, req.MCPURL, req.DryRun = "http", "http://127.0.0.1:47831/mcp", true
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(home, ".codex")) {
		t.Fatal("dry-run created the Codex home")
	}
}
