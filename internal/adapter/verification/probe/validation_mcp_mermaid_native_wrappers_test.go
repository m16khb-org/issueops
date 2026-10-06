package probe

import (
	"issueops/internal/adapter/verification/probe/invariants"
	mcpsmoke "issueops/internal/adapter/verification/probe/mcpsmoke"
	"issueops/internal/adapter/verification/probe/qagate"
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/hostprotocol"
)

func TestValidationMCPMermaidNativeWrappersUseDefaultSurfaces(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeNativeIntegrationFixture(t, root, home)

	mcpDeps := mcpsmoke.MCPValidationDeps{
		RunSDKSmoke: func(string, string, []string, time.Duration) selfverify.StepResult {
			return selfverify.StepResult{Label: "MCP smoke", OK: true, Stdout: validMCPResponses()}
		},
	}
	if step := mcpsmoke.ValidateMCPWithDeps("fake-harness", root, mcpDeps); !step.OK || !strings.Contains(step.Stdout, "atomic_commit_preflight") {
		t.Fatalf("expected MCP wrapper success, got %+v", step)
	}

	if step := ValidateNativeIntegration(root); !step.OK || !strings.Contains(step.Stdout, "duplicate_mcp_warning_fixture") {
		t.Fatalf("expected native integration wrapper success, got %+v", step)
	}
}

func TestLintMermaidBlocksEnforcesGeniusThinkRules(t *testing.T) {
	good := "```mermaid\nflowchart LR\n    A[\"한글 노드<br/>설명\"] --> B[\"Next\"]\n    subgraph \"계획 레이어\"\n    end\n```\n"
	if issues := qagate.LintMermaidBlocks("good.md", good); len(issues) != 0 {
		t.Fatalf("valid mermaid was rejected: %+v", issues)
	}

	bad := "```mermaid\nflowchart LR\n    A[한글 노드<br>설명] --> B[Next]\n    subgraph 계획 레이어\n    end\n```\n"
	issues := qagate.LintMermaidBlocks("bad.md", bad)
	for _, want := range []string{"bad.md:3 mermaid uses <br>; use <br/>", "bad.md:3 mermaid node text must start with a quote", "bad.md:4 mermaid subgraph title must be quoted"} {
		if !slices.Contains(issues, want) {
			t.Fatalf("missing %q in issues: %+v", want, issues)
		}
	}

	documentedBadExample := "## 잘못된 예시 (파싱 에러 발생)\n\n```mermaid\nflowchart LR\n    A[한글 노드<br>설명]\n```\n"
	if issues := qagate.LintMermaidBlocks("genius.md", documentedBadExample); len(issues) != 0 {
		t.Fatalf("documented bad example should be ignored: %+v", issues)
	}
}

func TestForbiddenNameHitsSkipsRuntimeStateDirs(t *testing.T) {
	root := t.TempDir()
	runtimeFiles := []string{
		filepath.Join(".cache", "go-build", "log.txt"),
		filepath.Join(".claude", "hooks", ".logs", "hook-log.jsonl"),
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".codegraph", "daemon.log"),
		filepath.Join(".omc", "project-memory.json"),
		filepath.Join(".omx", "state.json"),
		filepath.Join("bin", "issueops"),
		filepath.Join("cache", "projects.json"),
	}
	for _, rel := range runtimeFiles {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("local m"+"16kh runtime state"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sourcePath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(sourcePath, []byte("source m"+"16kh leak"), 0o600); err != nil {
		t.Fatal(err)
	}

	hits := invariants.ForbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != "AGENTS.md contains m"+"16kh" {
		t.Fatalf("expected only source hit, got %+v", hits)
	}
}

func writeNativeIntegrationFixture(t *testing.T, root, home string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "omo", "mcp.json"),
		filepath.Join(root, "configs", "omo", "issueops.js"),
		filepath.Join(root, "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "shared", "SKILL.md"),
	} {
		writeFileForWrapperTest(t, path, "ok\n")
	}
	writeFileForWrapperTest(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.issueops]\ncommand = \"issueops\"\n")
	writeFileForWrapperTest(t, filepath.Join(home, ".codex", "hooks.json"), fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":"'%s' hook session-start --host codex","timeout":5,"type":"command"}]}]}}`, filepath.Join(root, "bin", "issueops")))
	writeFileForWrapperTest(t, filepath.Join(home, ".omo", "mcp.json"), fmt.Sprintf(`{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, filepath.Join(root, "bin", "issueops"), root))
	writeFileForWrapperTest(t, filepath.Join(home, ".omo", "extensions", "issueops.js"), hostprotocol.OmoLifecycleExtension(filepath.Join(root, "bin", "issueops")))
}

func writeFileForWrapperTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
