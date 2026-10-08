package omp

import (
	"encoding/json"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/installutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"

	"issueops/internal/port"
)

func TestInstallerWritesNativeOmpSurfaces(t *testing.T) {
	req := ompTestRequest(t)
	req.ProjectLocal = true
	writeOmpTestJSON(t, filepath.Join(req.Home, ".omp", "agent", "mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other"},
		},
	})

	result, err := testInstaller().Install(req)
	if err != nil {
		t.Fatalf("Install returned error: %v\n%+v", err, result)
	}
	if !result.OK || result.Host != "omp" {
		t.Fatalf("unexpected install result: %+v", result)
	}

	assertOmpTestSkillLink(t, filepath.Join(req.Home, ".omp", "agent", "skills", "alpha"), filepath.Join(req.Root, "skills", "alpha"))
	if _, err := os.Lstat(filepath.Join(req.Root, ".omp", "skills")); !os.IsNotExist(err) {
		t.Fatalf("project-local install must not create repo-local omp skill links: %v", err)
	}

	globalMCP := readOmpTestJSON(t, filepath.Join(req.Home, ".omp", "agent", "mcp.json"))
	assertOmpTestMCPServer(t, globalMCP, "issueops", req.BinPath, req.Root)
	servers := globalMCP["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("omp MCP merge removed unrelated server")
	}

	projectMCP := readOmpTestJSON(t, filepath.Join(req.Root, ".omp", "mcp.json"))
	assertOmpTestMCPServer(t, projectMCP, "issueops_project", "./bin/issueops", ".")

	extension := readOmpTestFile(t, filepath.Join(req.Home, ".omp", "agent", "extensions", "issueops.js"))
	if extension != hostprotocol.OmpLifecycleExtension(req.BinPath) {
		t.Fatal("installed lifecycle extension differs from canonical host protocol")
	}

	for _, token := range []string{
		`"--json"`,
		req.BinPath,
	} {
		if !strings.Contains(extension, token) {
			t.Fatalf("omp lifecycle extension missing %q:\n%s", token, extension)
		}
	}
	for _, path := range []string{
		filepath.Join(req.Root, "configs", "omp", "mcp.json"),
		filepath.Join(req.Root, "configs", "omp", "issueops.js"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing omp template %s: %v", path, err)
		}
	}
}

func TestInstallerDryRunPlansWithoutWriting(t *testing.T) {
	req := ompTestRequest(t)
	req.ProjectLocal = true
	req.DryRun = true

	result, err := testInstaller().Install(req)
	if err != nil {
		t.Fatalf("dry-run returned error: %v\n%+v", err, result)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(req.Home, ".omp")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote omp user directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(req.Root, ".omp")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote project-local omp directory: %v", err)
	}
	if !ompTestHasPlannedWrite(result.Files) || !ompTestHasPlannedLink(result.Links) {
		t.Fatalf("dry-run omitted planned files or links: %+v", result)
	}
}

func TestVerifyActivationRejectsTamperedExtension(t *testing.T) {
	req := ompTestRequest(t)
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	evidence, err := testInstaller().VerifyActivation(req)
	if err != nil {
		t.Fatalf("VerifyActivation returned error: %v", err)
	}
	if len(evidence) != 2 || evidence[0].Host != "omp" || evidence[0].Surface != "mcp" ||
		evidence[1].Host != "omp" || evidence[1].Surface != "hooks" {
		t.Fatalf("unexpected omp activation evidence: %+v", evidence)
	}

	extensionPath := filepath.Join(req.Home, ".omp", "agent", "extensions", "issueops.js")
	if err := os.WriteFile(extensionPath, []byte("export default function () {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := testInstaller().VerifyActivation(req); err == nil || !strings.Contains(err.Error(), "lifecycle extension") {
		t.Fatalf("tampered extension must fail strict readback, got %v", err)
	}
}

func TestTrackedTemplatesMatchGeneratedContent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	extension := readOmpTestFile(t, filepath.Join(root, "configs", "omp", "issueops.js"))
	if extension != hostprotocol.OmpLifecycleExtension("./bin/issueops") {
		t.Fatal("tracked omp lifecycle extension drifted from generated template")
	}
	config := readOmpTestJSON(t, filepath.Join(root, "configs", "omp", "mcp.json"))
	got, err := installutil.SemanticSHA256(config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig, err := testInstaller().ompProjectMCPConfig()
	if err != nil {
		t.Fatal(err)
	}
	want, err := installutil.SemanticSHA256(expectedConfig)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("tracked omp MCP config drifted from generated template")
	}
}

func ompTestRequest(t *testing.T) port.NativeInstallRequest {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	skillDir := filepath.Join(root, "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: alpha\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return port.NativeInstallRequest{
		Root:       root,
		Home:       home,
		BinPath:    filepath.Join(root, "bin", "issueops"),
		SkillNames: []string{"alpha"},
	}
}

func writeOmpTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readOmpTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body := readOmpTestFile(t, path)
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func readOmpTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertOmpTestMCPServer(t *testing.T, config map[string]any, name, command, root string) {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers: %+v", config)
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		t.Fatalf("missing MCP server %q: %+v", name, servers)
	}
	if server["type"] != "stdio" || server["command"] != command {
		t.Fatalf("server %q type/command = %v %v, want stdio %s", name, server["type"], server["command"], command)
	}
	env, ok := server["env"].(map[string]any)
	if !ok || env["ISSUEOPS_ROOT"] != root {
		t.Fatalf("server %q ISSUEOPS_ROOT drifted: %+v", name, server)
	}
	wantCatalogSHA256, err := installutil.SemanticSHA256(mcpcatalog.AdvertisedTools())
	if err != nil {
		t.Fatal(err)
	}
	if env["ISSUEOPS_MCP_CATALOG_SHA256"] != wantCatalogSHA256 {
		t.Fatalf(
			"server %q ISSUEOPS_MCP_CATALOG_SHA256 = %v, want %s",
			name,
			env["ISSUEOPS_MCP_CATALOG_SHA256"],
			wantCatalogSHA256,
		)
	}
}

func assertOmpTestSkillLink(t *testing.T, path, target string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %s: %v", path, err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve %s: %v", target, err)
	}
	if resolved != want {
		t.Fatalf("link %s resolves to %s, want %s", path, resolved, want)
	}
}

func ompTestHasPlannedWrite(files []port.InstallFile) bool {
	for _, file := range files {
		if file.WouldWrite {
			return true
		}
	}
	return false
}

func ompTestHasPlannedLink(links []port.InstallLink) bool {
	for _, link := range links {
		if link.WouldCreate {
			return true
		}
	}
	return false
}
