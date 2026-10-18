package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/adapter/hostprotocol"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/installutil"
	"issueops/internal/domain/nativehost"
	"issueops/internal/port"
)

func testDependencies() Dependencies {
	return Dependencies{
		MergeJSONMapFile:                installutil.MergeJSONMapFile,
		RemoveJSONMapEntry:              installutil.RemoveJSONMapEntry,
		VerifyJSONMapEntry:              installutil.VerifyJSONMapEntry,
		CaptureNativeActivationEvidence: installutil.CaptureNativeActivationEvidence,
		MCPCatalogSHA256:                func() (string, error) { return installutil.SemanticSHA256(mcpcatalog.AdvertisedTools()) },
		NewInstallPlan:                  func(host string, dry bool) port.InstallPlan { return installutil.NewPlan(host, dry) },
		PlanHostSkillLinks:              installutil.PlanHostSkillLinks,
		SemanticSHA256:                  installutil.SemanticSHA256,
		WriteJSONPlan:                   installutil.WriteJSONPlan,
		WriteTextPlan:                   installutil.WriteTextPlan,
	}
}

func testInstaller() Installer {
	return NewInstaller(testDependencies(), hostprotocol.OpencodeLifecyclePlugin, func(home string) string {
		return nativehost.OpencodeConfigRoot(home, "")
	})
}

func testRequest(t *testing.T) port.NativeInstallRequest {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	skillDir := filepath.Join(root, "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: alpha\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return port.NativeInstallRequest{Root: root, Home: home, BinPath: filepath.Join(root, "bin", "issueops"), SkillNames: []string{"alpha"}}
}

func configDir(req port.NativeInstallRequest) string {
	return filepath.Join(req.Home, ".config", "opencode")
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func server(t *testing.T, config map[string]any, name string) map[string]any {
	t.Helper()
	mcp, _ := config["mcp"].(map[string]any)
	servers, _ := mcp["servers"].(map[string]any)
	entry, ok := servers[name].(map[string]any)
	if !ok {
		t.Fatalf("missing mcp.servers.%s: %+v", name, config)
	}
	return entry
}

func catalogSHA256(t *testing.T) string {
	t.Helper()
	digest, err := installutil.SemanticSHA256(mcpcatalog.AdvertisedTools())
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestInstallerPreservesUserConfigAndPlugins(t *testing.T) {
	req := testRequest(t)
	req.ProjectLocal = true
	dir := configDir(req)
	writeFile(t, filepath.Join(dir, "opencode.json"), `{"model":"mine/m","mcp":{"servers":{"other":{"type":"local","command":["x"]}}},"plugins":["./plugins/other.js"]}`, 0o644)
	otherPlugin := "export default { id: \"other\", setup() {} }\n"
	writeFile(t, filepath.Join(dir, "plugins", "other.js"), otherPlugin, 0o644)

	result, err := testInstaller().Install(req)
	if err != nil || !result.OK || result.Host != "opencode" {
		t.Fatalf("install = %+v, %v", result, err)
	}
	config := readJSON(t, filepath.Join(dir, "opencode.json"))
	if config["model"] != "mine/m" || !reflect.DeepEqual(config["plugins"], []any{"./plugins/other.js"}) {
		t.Fatalf("unrelated keys changed: %+v", config)
	}
	if other := server(t, config, "other"); other["type"] != "local" {
		t.Fatalf("other server changed: %+v", other)
	}
	want := map[string]any{
		"type": "local", "command": []any{req.BinPath, "mcp"}, "codemode": false,
		"environment": map[string]any{"ISSUEOPS_ROOT": req.Root, "ISSUEOPS_MCP_CATALOG_SHA256": catalogSHA256(t)},
	}
	if got := server(t, config, "issueops"); !reflect.DeepEqual(got, want) {
		t.Fatalf("issueops entry = %+v, want %+v", got, want)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "plugins", "other.js")); string(body) != otherPlugin {
		t.Fatalf("other plugin changed: %q", body)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "plugins", "issueops.js")); string(body) != hostprotocol.OpencodeLifecyclePlugin(req.BinPath) {
		t.Fatal("installed plugin differs from the generator")
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Join(dir, "skills", "alpha")); err != nil || !strings.HasSuffix(resolved, filepath.Join("skills", "alpha")) {
		t.Fatalf("skill link = %q, %v", resolved, err)
	}
	project := readJSON(t, filepath.Join(req.Root, ".opencode", "opencode.json"))
	if got := server(t, project, "issueops_project"); !reflect.DeepEqual(got["command"], []any{"./bin/issueops", "mcp"}) {
		t.Fatalf("project entry = %+v", got)
	}
	for _, path := range []string{filepath.Join(req.Root, "configs", "opencode", "opencode.json"), filepath.Join(req.Root, "configs", "opencode", "issueops.js")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing template %s: %v", path, err)
		}
	}
	if _, err := testInstaller().VerifyActivation(req); err != nil {
		t.Fatalf("activation readback: %v", err)
	}
}

func TestInstallerHTTPEntryNarrowsModeAndKeepsSecretOutOfProject(t *testing.T) {
	req := testRequest(t)
	req.ProjectLocal = true
	req.MCPTransport, req.MCPURL, req.MCPBearer = "http", "http://127.0.0.1:47831/mcp", "secret-bearer"
	dir := configDir(req)
	userConfig := filepath.Join(dir, "opencode.json")
	writeFile(t, userConfig, `{}`, 0o644)
	projectConfig := filepath.Join(req.Root, ".opencode", "opencode.json")
	writeFile(t, projectConfig, `{"mcp":{"servers":{"issueops_project":{"type":"local"},"keep":{}}}}`, 0o644)

	if result, err := testInstaller().Install(req); err != nil || !result.OK {
		t.Fatalf("install = %+v, %v", result, err)
	}
	info, err := os.Stat(userConfig)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("user config mode = %v, %v", info.Mode().Perm(), err)
	}
	want := map[string]any{
		"type": "remote", "url": req.MCPURL, "oauth": false, "codemode": false,
		"headers": map[string]any{"Authorization": "Bearer secret-bearer", "X-Issueops-Mcp-Catalog-Sha256": catalogSHA256(t)},
	}
	if got := server(t, readJSON(t, userConfig), "issueops"); !reflect.DeepEqual(got, want) {
		t.Fatalf("http entry = %+v", got)
	}
	project := readJSON(t, projectConfig)
	servers := project["mcp"].(map[string]any)["servers"].(map[string]any)
	if _, ok := servers["issueops_project"]; ok || servers["keep"] == nil {
		t.Fatalf("project-local HTTP must drop only issueops_project: %+v", servers)
	}
	if _, err := testInstaller().VerifyActivation(req); err != nil {
		t.Fatalf("activation readback: %v", err)
	}
}

func TestInstallerRejectsJsoncShadow(t *testing.T) {
	req := testRequest(t)
	writeFile(t, filepath.Join(configDir(req), "opencode.jsonc"), "{\n  // user comment\n  \"mcp\": {\"servers\": {\"issueops\": {\"type\": \"local\"},},},\n}\n", 0o644)
	result, err := testInstaller().Install(req)
	if err == nil || !strings.Contains(err.Error(), "opencode.jsonc defines mcp.servers.issueops") {
		t.Fatalf("shadowing jsonc accepted: %+v, %v", result, err)
	}
	if body, _ := os.ReadFile(filepath.Join(configDir(req), "opencode.jsonc")); !strings.Contains(string(body), "// user comment") {
		t.Fatal("opencode.jsonc was rewritten")
	}
}

func TestInstallerRefusesCommentedConfigWithoutRewritingIt(t *testing.T) {
	commented := "{\n  // my theme\n  \"theme\": \"opencode\",\n}\n"
	for _, tc := range []struct {
		name   string
		path   func(req port.NativeInstallRequest) string
		adjust func(req *port.NativeInstallRequest)
	}{
		{name: "user config", path: func(req port.NativeInstallRequest) string { return filepath.Join(configDir(req), "opencode.json") }},
		{name: "project config under http", path: func(req port.NativeInstallRequest) string {
			return filepath.Join(req.Root, ".opencode", "opencode.json")
		},
			adjust: func(req *port.NativeInstallRequest) {
				req.ProjectLocal, req.MCPTransport, req.MCPURL, req.MCPBearer = true, "http", "http://127.0.0.1:47831/mcp", "secret-bearer"
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := testRequest(t)
			if tc.adjust != nil {
				tc.adjust(&req)
			}
			path := tc.path(req)
			writeFile(t, path, commented, 0o644)
			_, err := testInstaller().Install(req)
			if err == nil || !strings.Contains(err.Error(), path+": opencode.json has comments or trailing commas") || !strings.Contains(err.Error(), "opencode.jsonc") {
				t.Fatalf("commented opencode.json accepted or unhelpful error: %v", err)
			}
			if body, _ := os.ReadFile(path); string(body) != commented {
				t.Fatalf("opencode.json was rewritten: %q", body)
			}
		})
	}
}

func TestInstallerWarnsOnUnreadableJsonc(t *testing.T) {
	req := testRequest(t)
	writeFile(t, filepath.Join(configDir(req), "opencode.jsonc"), "{ not json", 0o644)
	result, err := testInstaller().Install(req)
	if err != nil || !result.OK {
		t.Fatalf("an unreadable jsonc must only warn: %+v, %v", result, err)
	}
	if !strings.Contains(strings.Join(result.Messages, "\n"), "opencode.jsonc could not be read; make sure it does not define mcp.servers.issueops") {
		t.Fatalf("messages = %q", result.Messages)
	}
}

func TestNormalizeJsoncKeepsStringsAndDropsCommentsAndTrailingCommas(t *testing.T) {
	src := `{
  "$schema": "https://opencode.ai/config.json", // trailing note
  /* block
     comment */ "path": "a/*b*/c//d",
  "quote": "say \"hi\" // not a comment",
  "list": [1, 2,],
}`
	var got map[string]any
	if err := json.Unmarshal(normalizeJSONC([]byte(src)), &got); err != nil {
		t.Fatalf("normalized jsonc is not JSON: %v\n%s", err, normalizeJSONC([]byte(src)))
	}
	want := map[string]any{
		"$schema": "https://opencode.ai/config.json", "path": "a/*b*/c//d",
		"quote": `say "hi" // not a comment`, "list": []any{1.0, 2.0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized = %+v, want %+v", got, want)
	}
}

func TestInstallerDryRunPlansWithoutWriting(t *testing.T) {
	req := testRequest(t)
	req.ProjectLocal, req.DryRun = true, true
	result, err := testInstaller().Install(req)
	if err != nil || !result.OK || !result.DryRun {
		t.Fatalf("dry-run = %+v, %v", result, err)
	}
	if entries, _ := os.ReadDir(req.Home); len(entries) != 0 {
		t.Fatalf("dry-run wrote under home: %v", entries)
	}
	for _, path := range []string{filepath.Join(req.Root, ".opencode"), filepath.Join(req.Root, "configs")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s: %v", path, err)
		}
	}
	kinds := map[string]bool{}
	for _, file := range result.Files {
		kinds[file.Kind] = file.WouldWrite
	}
	for _, kind := range []string{"opencode_user_mcp_config", "opencode_user_lifecycle_plugin", "opencode_project_mcp_template", "opencode_lifecycle_extension_template", "opencode_project_mcp_config"} {
		if !kinds[kind] {
			t.Fatalf("dry-run did not plan %s: %+v", kind, result.Files)
		}
	}
}

func TestVerifyActivationRejectsTamperedPlugin(t *testing.T) {
	req := testRequest(t)
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir(req), "plugins", "issueops.js"), "tampered", 0o644)
	if _, err := testInstaller().VerifyActivation(req); err == nil || !strings.Contains(err.Error(), "lifecycle plugin does not match") {
		t.Fatalf("tampered plugin accepted: %v", err)
	}
}

func TestTrackedTemplatesMatchGeneratedContent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	plugin, err := os.ReadFile(filepath.Join(root, "configs", "opencode", "issueops.js"))
	if err != nil || string(plugin) != hostprotocol.OpencodeLifecyclePlugin("./bin/issueops") {
		t.Fatalf("tracked opencode plugin drifted from the generator: %v", err)
	}
	req := testRequest(t)
	if _, err := testInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	generated, tracked := readJSON(t, filepath.Join(req.Root, "configs", "opencode", "opencode.json")), readJSON(t, filepath.Join(root, "configs", "opencode", "opencode.json"))
	if !reflect.DeepEqual(generated, tracked) {
		t.Fatalf("tracked opencode project template drifted: %+v", tracked)
	}
}
