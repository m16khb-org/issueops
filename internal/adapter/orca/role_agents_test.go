package orca

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/domain/shelltoken"
	"issueops/internal/port"
)

// 역할 에이전트 인자는 셸 인용을 정확히 한 번 거친다. Orca가 --command를 셸로
// 다시 나누면 원래 인자가 그대로 나와야 한다.
func TestOwnerAgentCommandRoleAgents(t *testing.T) {
	agents := `{"issueops-research":{"description":"it's read-only","prompt":"say \"hi\" $HOME","model":"claude-sonnet-5-5","effort":"medium"}}`
	command, ok := ownerAgentCommand("claude", "claude-opus-5-5", "high", false, []string{"--agents", agents})
	if !ok {
		t.Fatal("ownerAgentCommand rejected role agents")
	}
	tokens := shelltoken.SplitCommandTokens(command)
	want := []string{"claude", "--model", "claude-opus-5-5", "--effort", "high", "--dangerously-skip-permissions", "--agents", agents}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("tokens = %q\nwant   %q", tokens, want)
	}
	if !json.Valid([]byte(tokens[len(tokens)-1])) {
		t.Fatalf("the --agents value is not JSON after re-splitting: %s", tokens[len(tokens)-1])
	}

	codexArg := `agents.issueops-research.config_file="/state/agent-roles/abc.toml"`
	command, ok = ownerAgentCommand("codex", "gpt-6.1-sol", "high", true, []string{"-c", codexArg})
	if !ok || !strings.HasSuffix(command, " '-c' '"+codexArg+"'") || strings.Count(command, codexArg) != 1 {
		t.Fatalf("codex command = %s", command)
	}

	if _, ok := ownerAgentCommand("claude", "m", "", false, []string{"--agents", "a\x00b"}); ok {
		t.Fatal("a NUL launch argument must be rejected")
	}
	if command, ok := ownerAgentCommand("claude", "m", "", false, []string{""}); !ok || strings.HasSuffix(command, "''") {
		t.Fatalf("an empty argument must not be rendered: %q", command)
	}
}

func TestOwnerAgentCommandWithAllRoleAgentsStaysBelow8KiB(t *testing.T) {
	var roles []string
	for _, role := range []string{"plan-review", "diff-review", "review-escalate", "research", "reader-check"} {
		roles = append(roles, `"issueops-`+role+`":{"description":"`+strings.Repeat("d", 120)+`","prompt":"`+strings.Repeat("p", 320)+`","model":"claude-opus-5-5","effort":"xhigh"}`)
	}
	command, ok := ownerAgentCommand("claude", "claude-opus-5-5", "high", false, []string{"--agents", "{" + strings.Join(roles, ",") + "}"})
	if !ok || len(command) >= 8<<10 {
		t.Fatalf("command length = %d", len(command))
	}
}

func TestClientCreateTerminalAppendsRoleAgentArgs(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
	command := `claude --model 'opus' --dangerously-skip-permissions '--agents' '{"issueops-research":{"model":"sonnet"}}'`
	runner.responses["orca terminal create --worktree id:worktree-1 --command "+command+" --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-create","worktreeId":"worktree-1"}}}`)}
	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
		WorktreeID: "worktree-1", Agent: "claude", Model: "opus",
		ExtraArgs: []string{"--agents", `{"issueops-research":{"model":"sonnet"}}`},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProbeRequiresClaudeAgentsFlag(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["claude"] = "/usr/local/bin/claude"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["claude --help"] = CommandOutput{Stdout: []byte("--model --dangerously-skip-permissions")}
	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "claude"})
	if err != nil || result.Ready || result.Code != "host_role_agents_unsupported" {
		t.Fatalf("claude without --agents probe = %#v err=%v", result, err)
	}
}
