package hostprotocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	modelcontract "issueops/internal/contract/agentmodel"
)

func TestClaudeAgentsJSON(t *testing.T) {
	got, err := ClaudeAgentsJSON([]RoleAgent{
		{Role: modelcontract.RoleResearch, Model: "claude-sonnet-5-5", Effort: "medium"},
		{Role: modelcontract.RoleReaderCheck, Model: "claude-haiku-5-5"},
		{Role: modelcontract.RoleDiffReview},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, got)
	}
	if len(decoded) != 2 {
		t.Fatalf("an agent without a model must be skipped: %v", decoded)
	}
	research := decoded["issueops-research"]
	if research["model"] != "claude-sonnet-5-5" || research["effort"] != "medium" || research["description"] == "" || research["prompt"] == "" {
		t.Fatalf("research agent = %v", research)
	}
	if _, ok := decoded["issueops-reader-check"]["effort"]; ok {
		t.Fatalf("an empty effort must be omitted: %v", decoded["issueops-reader-check"])
	}
	if _, err := ClaudeAgentsJSON([]RoleAgent{{Role: "reviewer", Model: "opus"}}); err == nil {
		t.Fatal("an unknown role must be rejected")
	}
}

func TestCodexRoleFile(t *testing.T) {
	got, err := CodexRoleFile(RoleAgent{Role: modelcontract.RoleResearch, Model: "gpt-6-luna", Effort: "low"})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{`name = "issueops-research"`, `model = "gpt-6-luna"`, `model_reasoning_effort = "low"`} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("role file is missing %s:\n%s", line, got)
		}
	}
	if !strings.Contains(got, "description = \"") || !strings.Contains(got, "developer_instructions = \"") {
		t.Fatalf("role file:\n%s", got)
	}
	noEffort, _ := CodexRoleFile(RoleAgent{Role: modelcontract.RoleResearch, Model: "gpt-6-luna"})
	if strings.Contains(noEffort, "model_reasoning_effort") {
		t.Fatalf("an empty effort must be omitted:\n%s", noEffort)
	}
}

func TestTOMLStringEscapes(t *testing.T) {
	if got := tomlString("a \"b\" \\ c\n<d>'e'"); got != `"a \"b\" \\ c\n<d>'e'"` {
		t.Fatalf("tomlString = %s", got)
	}
}

func TestRoleAgentArgs(t *testing.T) {
	agents := []RoleAgent{{Role: modelcontract.RoleResearch, Model: "gpt-6-luna", Effort: "low"}, {Role: modelcontract.RoleReaderCheck, Model: "gpt-6-luna", Effort: "low"}}
	var written []string
	write := func(content string) (string, error) {
		written = append(written, content)
		return "/state/agent-roles/" + string(rune('a'+len(written)-1)) + ".toml", nil
	}
	got, err := RoleAgentArgs("codex", agents, write)
	want := []string{"-c", `agents.issueops-research.config_file="/state/agent-roles/a.toml"`, "-c", `agents.issueops-reader-check.config_file="/state/agent-roles/b.toml"`}
	if err != nil || !reflect.DeepEqual(got, want) || len(written) != 2 {
		t.Fatalf("codex RoleAgentArgs = %q, %v", got, err)
	}
	claude, err := RoleAgentArgs("claude", []RoleAgent{{Role: modelcontract.RoleResearch, Model: "claude-sonnet-5-5"}}, nil)
	if err != nil || len(claude) != 2 || claude[0] != "--agents" || !json.Valid([]byte(claude[1])) {
		t.Fatalf("claude RoleAgentArgs = %q, %v", claude, err)
	}
	if omo, err := RoleAgentArgs("omo", agents, nil); err != nil || omo != nil {
		t.Fatalf("omo RoleAgentArgs = %q, %v", omo, err)
	}
	if none, err := RoleAgentArgs("claude", nil, nil); err != nil || none != nil {
		t.Fatalf("no agents = %q, %v", none, err)
	}
}

func TestBuildPrintArgv(t *testing.T) {
	if got := BuildPrintArgv("claude", "claude-opus-5-5", "high"); !reflect.DeepEqual(got, []string{"claude", "-p", "--model", "claude-opus-5-5", "--effort", "high"}) {
		t.Fatalf("claude = %q", got)
	}
	if got := BuildPrintArgv("codex", "gpt-6-luna", "low"); !reflect.DeepEqual(got, []string{"codex", "exec", "-m", "gpt-6-luna", "-c", "model_reasoning_effort=low"}) {
		t.Fatalf("codex = %q", got)
	}
	if got := BuildPrintArgv("codex", "gpt-6-luna", ""); !reflect.DeepEqual(got, []string{"codex", "exec", "-m", "gpt-6-luna"}) {
		t.Fatalf("codex without effort = %q", got)
	}
	if got := BuildPrintArgv("omo", "x", "max"); got != nil {
		t.Fatalf("omo = %q", got)
	}
}
