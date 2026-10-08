package extensionhost

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
	"issueops/internal/port"
)

type hostCase struct {
	spec      Spec
	extension func(string) string
}

var hostCases = []hostCase{
	{Omo, hostprotocol.OmoLifecycleExtension},
	{Omp, hostprotocol.OmpLifecycleExtension},
}

func forEachHost(t *testing.T, run func(t *testing.T, tc hostCase)) {
	t.Helper()
	for _, tc := range hostCases {
		t.Run(tc.spec.Host, func(t *testing.T) { run(t, tc) })
	}
}

func (tc hostCase) installer() Installer {
	return NewInstaller(tc.spec, testDependencies(), tc.extension)
}

func (tc hostCase) userMCPPath(req port.NativeInstallRequest) string {
	return filepath.Join(req.Home, tc.spec.ConfigRoot, "mcp.json")
}

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

func TestInstallerWritesNativeSurfaces(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		host := tc.spec.Host
		req := testRequest(t)
		req.ProjectLocal = true
		writeTestJSON(t, tc.userMCPPath(req), map[string]any{
			"mcpServers": map[string]any{
				"other": map[string]any{"command": "other"},
			},
		})

		result, err := tc.installer().Install(req)
		if err != nil {
			t.Fatalf("Install returned error: %v\n%+v", err, result)
		}
		if !result.OK || result.Host != host {
			t.Fatalf("unexpected install result: %+v", result)
		}

		assertTestSkillLink(t, filepath.Join(req.Home, tc.spec.SkillsRoot, "alpha"), filepath.Join(req.Root, "skills", "alpha"))
		if _, err := os.Lstat(filepath.Join(req.Root, "."+host, "skills")); !os.IsNotExist(err) {
			t.Fatalf("project-local install must not create repo-local %s skill links: %v", host, err)
		}

		globalMCP := readTestJSON(t, tc.userMCPPath(req))
		assertTestMCPServer(t, tc.spec, globalMCP, "issueops", req.BinPath, req.Root)
		servers := globalMCP["mcpServers"].(map[string]any)
		if _, ok := servers["other"]; !ok {
			t.Fatalf("%s MCP merge removed unrelated server", tc.spec.DisplayName)
		}

		projectMCP := readTestJSON(t, filepath.Join(req.Root, "."+host, "mcp.json"))
		assertTestMCPServer(t, tc.spec, projectMCP, "issueops_project", "./bin/issueops", ".")

		extension := readTestFile(t, filepath.Join(req.Home, tc.spec.ConfigRoot, "extensions", "issueops.js"))
		if extension != tc.extension(req.BinPath) {
			t.Fatal("installed lifecycle extension differs from canonical host protocol")
		}
		for _, token := range []string{`"--json"`, req.BinPath} {
			if !strings.Contains(extension, token) {
				t.Fatalf("%s lifecycle extension missing %q:\n%s", tc.spec.DisplayName, token, extension)
			}
		}
		for _, path := range []string{
			filepath.Join(req.Root, "configs", host, "mcp.json"),
			filepath.Join(req.Root, "configs", host, "issueops.js"),
		} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("missing %s template %s: %v", tc.spec.DisplayName, path, err)
			}
		}
	})
}

func TestInstallerDryRunPlansWithoutWriting(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		req := testRequest(t)
		req.ProjectLocal = true
		req.DryRun = true

		result, err := tc.installer().Install(req)
		if err != nil {
			t.Fatalf("dry-run returned error: %v\n%+v", err, result)
		}
		if !result.OK || !result.DryRun {
			t.Fatalf("unexpected dry-run result: %+v", result)
		}
		if _, err := os.Stat(filepath.Join(req.Home, "."+tc.spec.Host)); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s user directory: %v", tc.spec.DisplayName, err)
		}
		if _, err := os.Stat(filepath.Join(req.Root, "."+tc.spec.Host)); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote project-local %s directory: %v", tc.spec.DisplayName, err)
		}
		if !testHasPlannedWrite(result.Files) || !testHasPlannedLink(result.Links) {
			t.Fatalf("dry-run omitted planned files or links: %+v", result)
		}
		want := "dry-run: planned " + tc.spec.DisplayName + " native skills, MCP config, and lifecycle extension without writing"
		if !reflect.DeepEqual(result.Messages[len(result.Messages)-1:], []string{want}) {
			t.Fatalf("dry-run messages = %q, want last %q", result.Messages, want)
		}
	})
}

func TestVerifyActivationRejectsTamperedExtension(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		host := tc.spec.Host
		req := testRequest(t)
		if _, err := tc.installer().Install(req); err != nil {
			t.Fatal(err)
		}
		evidence, err := tc.installer().VerifyActivation(req)
		if err != nil {
			t.Fatalf("VerifyActivation returned error: %v", err)
		}
		if len(evidence) != 2 || evidence[0].Host != host || evidence[0].Surface != "mcp" ||
			evidence[1].Host != host || evidence[1].Surface != "hooks" {
			t.Fatalf("unexpected %s activation evidence: %+v", tc.spec.DisplayName, evidence)
		}

		extensionPath := filepath.Join(req.Home, tc.spec.ConfigRoot, "extensions", "issueops.js")
		if err := os.WriteFile(extensionPath, []byte("export default function () {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = tc.installer().VerifyActivation(req)
		want := tc.spec.DisplayName + " lifecycle extension does not match the canonical managed content"
		if err == nil || err.Error() != want {
			t.Fatalf("tampered extension must fail strict readback with %q, got %v", want, err)
		}
	})
}

func TestCatalogDigestErrorsUseDisplayName(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		name := tc.spec.DisplayName
		for _, c := range []struct {
			digest func() (string, error)
			want   string
		}{
			{nil, name + " MCP catalog digest is not configured"},
			{func() (string, error) { return "", nil }, "compute " + name + " MCP catalog digest: empty digest"},
		} {
			deps := testDependencies()
			deps.MCPCatalogSHA256 = c.digest
			_, err := NewInstaller(tc.spec, deps, tc.extension).mcpServer("x", ".")
			if err == nil || err.Error() != c.want {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		}
	})
}

func TestTrackedTemplatesMatchGeneratedContent(t *testing.T) {
	forEachHost(t, func(t *testing.T, tc hostCase) {
		root := filepath.Join("..", "..", "..")
		extension := readTestFile(t, filepath.Join(root, "configs", tc.spec.Host, "issueops.js"))
		if extension != tc.extension("./bin/issueops") {
			t.Fatalf("tracked %s lifecycle extension drifted from generated template", tc.spec.DisplayName)
		}
		config := readTestJSON(t, filepath.Join(root, "configs", tc.spec.Host, "mcp.json"))
		got, err := installutil.SemanticSHA256(config)
		if err != nil {
			t.Fatal(err)
		}
		expectedConfig, err := tc.installer().projectMCPConfig()
		if err != nil {
			t.Fatal(err)
		}
		want, err := installutil.SemanticSHA256(expectedConfig)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("tracked %s MCP config drifted from generated template", tc.spec.DisplayName)
		}
	})
}

func testRequest(t *testing.T) port.NativeInstallRequest {
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

func writeTestJSON(t *testing.T, path string, value any) {
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

func readTestJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body := readTestFile(t, path)
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertTestMCPServer(t *testing.T, spec Spec, config map[string]any, name, command, root string) {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers: %+v", config)
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		t.Fatalf("missing MCP server %q: %+v", name, servers)
	}
	if server["command"] != command {
		t.Fatalf("server %q command = %v, want %s", name, server["command"], command)
	}
	if wantType, _ := server["type"].(string); wantType != spec.StdioType {
		t.Fatalf("server %q type = %v, want %q", name, server["type"], spec.StdioType)
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

func assertTestSkillLink(t *testing.T, path, target string) {
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

func testHasPlannedWrite(files []port.InstallFile) bool {
	for _, file := range files {
		if file.WouldWrite {
			return true
		}
	}
	return false
}

func testHasPlannedLink(links []port.InstallLink) bool {
	for _, link := range links {
		if link.WouldCreate {
			return true
		}
	}
	return false
}
