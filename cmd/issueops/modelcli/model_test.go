package modelcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agentmodelapp "issueops/internal/application/agentmodel"
	contract "issueops/internal/contract/agentmodel"
)

type fakeFiles struct {
	settings   agentmodelapp.Settings
	loadErr    error
	written    map[string]contract.Config
	excluded   bool
	agentCalls int
}

func (f *fakeFiles) deps(out *bytes.Buffer) Dependencies {
	return Dependencies{
		Models: agentmodelapp.Service{Load: func(context.Context, string) (agentmodelapp.Settings, error) {
			return f.settings, f.loadErr
		}},
		WriteGlobal: func(path string, cfg contract.Config) error {
			f.written[path] = cfg
			f.settings.Global = cfg
			return nil
		},
		WriteLocal: func(_ context.Context, _ string, cfg contract.Config) (bool, error) {
			f.written[f.settings.LocalPath] = cfg
			f.settings.Local = cfg
			return f.excluded, nil
		},
		CodexCatalog: func() []string { return []string{"gpt-6-luna"} },
		RoleAgentArgs: func(_ context.Context, host, _ string) ([]string, error) {
			f.agentCalls++
			return []string{"-c", "agents.issueops-research.config_file=\"/state/agent-roles/x.toml\""}, nil
		},
		PrintArgv: func(host, model, effort string) []string { return []string{host, model, effort} },
		Getwd:     func() (string, error) { return "/repo", nil },
		Stdout:    out,
	}
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{
		settings: agentmodelapp.Settings{LocalPath: "/repo/.issueops/agent-models.local.json", GlobalPath: "/xdg/issueops/agent-models.json"},
		written:  map[string]contract.Config{}, excluded: true,
	}
}

func run(t *testing.T, files *fakeFiles, args ...string) (map[string]any, error) {
	t.Helper()
	var out bytes.Buffer
	err := Run(files.deps(&out), args)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if json.Unmarshal(out.Bytes(), &decoded) != nil {
		return map[string]any{"text": out.String()}, nil
	}
	return decoded, nil
}

func TestSetThenResolve(t *testing.T) {
	files := newFakeFiles()
	got, err := run(t, files, "set", "--scope", "global", "--host", "codex", "--role", "research", "--model", "gpt-6-luna", "--effort", "low", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got["path"] != "/xdg/issueops/agent-models.json" || len(got["warnings"].([]any)) != 0 {
		t.Fatalf("set = %v", got)
	}
	if files.written["/xdg/issueops/agent-models.json"].Codex[contract.RoleResearch] != (contract.Layer{Model: "gpt-6-luna", Effort: "low"}) {
		t.Fatalf("written = %+v", files.written)
	}
	resolved, err := run(t, files, "resolve", "--host", "codex", "--role", "research", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["model"] != "gpt-6-luna" || resolved["effort"] != "low" || resolved["model_source"] != "global" || resolved["argv"].([]any)[0] != "codex" {
		t.Fatalf("resolve = %v", resolved)
	}
	if _, ok := resolved["agent_args"]; ok || files.agentCalls != 0 {
		t.Fatal("resolve without --agents must not render or write role agents")
	}
}

func TestSetMergesFieldsAndReportsLocalExclude(t *testing.T) {
	files := newFakeFiles()
	files.settings.Local = contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Model: "opus"}}}
	got, err := run(t, files, "set", "--scope", "local", "--host", "claude", "--role", "implement", "--effort", "xhigh", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got["excluded"] != true || files.settings.Local.Claude[contract.RoleImplement] != (contract.Layer{Model: "opus", Effort: "xhigh"}) {
		t.Fatalf("set = %v local=%+v", got, files.settings.Local)
	}
	if warnings := got["warnings"].([]any); len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestSetWarnsButSavesUnknownModels(t *testing.T) {
	files := newFakeFiles()
	got, err := run(t, files, "set", "--scope", "global", "--host", "codex", "--role", "diff-review", "--model", "gpt-6-astra", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(got["warnings"].([]any)) != 1 || files.settings.Global.Codex[contract.RoleDiffReview].Model != "gpt-6-astra" {
		t.Fatalf("set = %v", got)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--effort", "ultra"},
		{"set", "--scope", "global", "--host", "omo", "--role", "implement", "--model", "x"},
		{"set", "--scope", "team", "--host", "claude", "--role", "implement", "--model", "x"},
		{"set", "--scope", "global", "--host", "claude", "--role", "implement"},
		{"unset", "--scope", "global", "--host", "claude", "--role", "implement", "--field", "name"},
		{"resolve", "--host", "claude", "--role", "reviewer"},
		{"resolve", "--host", "claude", "--role", "implement", "--round", "-1"},
		{"bogus"},
	} {
		_, err := run(t, newFakeFiles(), args...)
		var usage UsageError
		if !errors.As(err, &usage) {
			t.Fatalf("%v: err = %v, want UsageError", args, err)
		}
	}
	_, err := run(t, newFakeFiles(), "set", "--scope", "global", "--host", "claude", "--role", "implement", "--effort", "ultra")
	if !strings.Contains(err.Error(), "low, medium, high, xhigh, max") {
		t.Fatalf("effort error must list the allowed values: %v", err)
	}
}

func TestBrokenSettingsAreFileErrors(t *testing.T) {
	files := newFakeFiles()
	files.loadErr = errors.New("/xdg/issueops/agent-models.json: unsupported version 2")
	for _, args := range [][]string{
		{"show", "--json"},
		{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "opus"},
		{"resolve", "--host", "claude", "--role", "implement", "--json"},
	} {
		_, err := run(t, files, args...)
		var usage UsageError
		if err == nil || errors.As(err, &usage) || !strings.Contains(err.Error(), "/xdg/issueops/agent-models.json") {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	if len(files.written) != 0 {
		t.Fatalf("a broken settings file must not be overwritten: %v", files.written)
	}
}

func TestUnset(t *testing.T) {
	files := newFakeFiles()
	files.settings.Global = contract.Config{Version: 1, Codex: map[contract.Role]contract.Layer{
		contract.RoleResearch: {Model: "gpt-6-luna", Effort: "low"},
	}}
	if _, err := run(t, files, "unset", "--scope", "global", "--host", "codex", "--role", "research", "--field", "effort", "--json"); err != nil {
		t.Fatal(err)
	}
	if files.settings.Global.Codex[contract.RoleResearch] != (contract.Layer{Model: "gpt-6-luna"}) {
		t.Fatalf("after field unset = %+v", files.settings.Global)
	}
	got, err := run(t, files, "unset", "--scope", "global", "--host", "codex", "--role", "research", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got["removed"] != true || files.settings.Global.Codex != nil {
		t.Fatalf("after role unset = %v %+v", got, files.settings.Global)
	}
	got, _ = run(t, files, "unset", "--scope", "global", "--host", "codex", "--role", "research", "--json")
	if got["removed"] != false {
		t.Fatalf("unsetting an absent role = %v", got)
	}
}

func TestShowListsEveryRoleWithSources(t *testing.T) {
	files := newFakeFiles()
	files.settings.Local = contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleResearch: {Effort: "low"}}}
	got, err := run(t, files, "show", "--json")
	if err != nil {
		t.Fatal(err)
	}
	hosts := got["hosts"].(map[string]any)
	claude := hosts["claude"].([]any)
	if len(hosts) != 2 || len(claude) != 7 || len(hosts["codex"].([]any)) != 7 {
		t.Fatalf("show = %v", got)
	}
	research := claude[5].(map[string]any)
	if research["role"] != "research" || research["effort"] != "low" || research["effort_source"] != "local" {
		t.Fatalf("research = %v", research)
	}
	if got["local_path"] != "/repo/.issueops/agent-models.local.json" || got["global_path"] != "/xdg/issueops/agent-models.json" {
		t.Fatalf("paths = %v", got)
	}
	text, err := run(t, files, "show", "--host", "codex")
	if err != nil || !strings.Contains(text["text"].(string), "codex  research") {
		t.Fatalf("text show = %v, %v", text, err)
	}
}

func TestResolveAgentsAndRounds(t *testing.T) {
	files := newFakeFiles()
	got, err := run(t, files, "resolve", "--host", "claude", "--role", "diff-review", "--round", "3", "--tier", "docs-only", "--agents", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got["effort"] != "high" || got["effort_source"] != "inherited" || files.agentCalls != 1 || len(got["agent_args"].([]any)) != 2 {
		t.Fatalf("resolve = %v", got)
	}
	flagged, err := run(t, files, "resolve", "--host", "claude", "--role", "implement", "--model", "sonnet", "--json")
	if err != nil || flagged["model"] != "sonnet" || flagged["model_source"] != "flag" {
		t.Fatalf("flag resolve = %v, %v", flagged, err)
	}
}
