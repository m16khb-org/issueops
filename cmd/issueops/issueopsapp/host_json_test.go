package issueopsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/installutil"
	"issueops/internal/port"
)

func TestHostJSONAdapterMatrix(t *testing.T) {
	for _, host := range []struct {
		name, userPath, projectPath string
		installer                   interface {
			Install(port.NativeInstallRequest) (port.HostInstallResult, error)
			VerifyActivation(port.NativeInstallRequest) ([]port.NativeActivationEvidence, error)
		}
	}{
		{"agy", ".gemini/config/mcp_config.json", ".agents/mcp_config.json", newAgyInstaller()},
		{"omo", ".omo/mcp.json", ".omo/mcp.json", newOmoInstaller()},
		{"omp", ".omp/agent/mcp.json", ".omp/mcp.json", newOmpInstaller()},
		{"claude", ".claude.json", ".mcp.json", newClaudeInstaller()},
	} {
		for _, scenario := range []string{"install", "dry-run", "malformed"} {
			t.Run(host.name+"/"+scenario, func(t *testing.T) {
				req := port.NativeInstallRequest{Home: t.TempDir(), Root: t.TempDir(), ProjectLocal: true, DryRun: scenario == "dry-run"}
				req.BinPath = filepath.Join(req.Root, "bin", "issueops")
				path := filepath.Join(req.Home, host.userPath)
				content := `{"theme":"dark","mcpServers":{"other":{"command":"keep"},"issueops":{"command":"old"}}}`
				if scenario == "malformed" {
					content = "{"
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				result, err := host.installer.Install(req)
				if scenario == "malformed" {
					if err == nil || result.OK {
						t.Fatalf("malformed config accepted: result=%+v err=%v", result, err)
					}
				} else if err != nil || !result.OK {
					t.Fatalf("install result=%+v err=%v", result, err)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if scenario != "install" {
					if string(raw) != content {
						t.Fatalf("config changed: %q", raw)
					}
					if scenario == "dry-run" {
						if _, err := os.Stat(filepath.Join(req.Root, host.projectPath)); !os.IsNotExist(err) {
							t.Fatalf("dry-run wrote project config: %v", err)
						}
					}
					return
				}
				var config map[string]any
				if err := json.Unmarshal(raw, &config); err != nil {
					t.Fatal(err)
				}
				servers := config["mcpServers"].(map[string]any)
				if config["theme"] != "dark" || !reflect.DeepEqual(servers["other"], map[string]any{"command": "keep"}) {
					t.Fatalf("unrelated config lost: %#v", config)
				}
				want := map[string]any{"command": req.BinPath, "args": []string{"mcp"}, "env": map[string]any{"ISSUEOPS_ROOT": req.Root}}
				if host.name == "claude" || host.name == "omp" {
					want["type"] = "stdio"
				}
				if host.name != "claude" {
					catalog, err := installutil.SemanticSHA256(mcpcatalog.AdvertisedTools())
					if err != nil {
						t.Fatal(err)
					}
					want["env"].(map[string]any)["ISSUEOPS_MCP_CATALOG_SHA256"] = catalog
				}
				actualDigest, err := installutil.SemanticSHA256(servers["issueops"])
				if err != nil {
					t.Fatal(err)
				}
				wantDigest, err := installutil.SemanticSHA256(want)
				if err != nil || actualDigest != wantDigest {
					t.Fatalf("host DTO changed: actual=%#v want=%#v err=%v", servers["issueops"], want, err)
				}
				found := false
				for _, file := range result.Files {
					if file.Path == path {
						found = true
						if file.Kind != host.name+"_user_mcp_config" {
							t.Fatalf("user kind=%s", file.Kind)
						}
					}
				}
				if !found {
					t.Fatal("user MCP file missing from plan")
				}
				for file, mode := range map[string]os.FileMode{path: 0o600, filepath.Join(req.Root, host.projectPath): 0o644} {
					info, err := os.Stat(file)
					if err != nil || info.Mode().Perm() != mode {
						t.Fatalf("file=%s mode=%o info=%v err=%v", file, mode, info, err)
					}
				}
				evidence, err := host.installer.VerifyActivation(req)
				wantCount := 2
				if host.name == "agy" {
					wantCount = 1
				}
				if err != nil || len(evidence) != wantCount {
					t.Fatalf("evidence=%+v err=%v", evidence, err)
				}
				if evidence[0].Host != host.name || evidence[0].Surface != "mcp" || evidence[0].Path != path || evidence[0].SemanticSHA256 != wantDigest {
					t.Fatalf("MCP evidence=%+v", evidence[0])
				}
				if wantCount == 2 && (evidence[1].Host != host.name || evidence[1].Surface != "hooks") {
					t.Fatalf("hook evidence=%+v", evidence[1])
				}
				servers["issueops"].(map[string]any)["command"] = "/stale/issueops"
				changed, err := json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, changed, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := host.installer.VerifyActivation(req); err == nil || !strings.Contains(err.Error(), "canonical binary") {
					t.Fatalf("stale target accepted: %v", err)
				}
			})
		}
	}
}
