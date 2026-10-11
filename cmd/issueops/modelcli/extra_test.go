package modelcli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	agentmodelapp "issueops/internal/application/agentmodel"
	contract "issueops/internal/contract/agentmodel"
)

func TestHelpPrintsUsageForEveryHelpSpelling(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}} {
		got, err := run(t, newFakeFiles(), args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if text, _ := got["text"].(string); text != usage {
			t.Fatalf("%v: output = %q, want the usage text", args, text)
		}
	}
}

func TestUnknownSubcommandErrorNamesItAndShowsUsage(t *testing.T) {
	_, err := run(t, newFakeFiles(), "bogus")
	usageErr, ok := errors.AsType[UsageError](err)
	if !ok || !strings.Contains(usageErr.Message, `unknown model subcommand "bogus"`) || !strings.Contains(usageErr.Message, "Usage:") {
		t.Fatalf("err = %v", err)
	}
}

func TestFlagParsingFailuresAreUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"show", "--nope"}, "flag provided but not defined: -nope"},
		{"positional argument", []string{"show", "extra"}, `unexpected positional argument "extra"`},
		{"positional after set flags", []string{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "opus", "stray"}, `unexpected positional argument "stray"`},
		{"unknown host in show", []string{"show", "--host", "vim"}, `unknown host "vim"`},
		{"unset without scope", []string{"unset", "--host", "claude", "--role", "implement"}, `--scope must be global or local, got ""`},
		{"unset on a host that is not configurable", []string{"unset", "--scope", "global", "--host", "omo", "--role", "implement"}, `host "omo" is not configurable`},
		{"unset an unknown role", []string{"unset", "--scope", "global", "--host", "claude", "--role", "boss"}, `unknown role "boss"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := newFakeFiles()
			_, err := run(t, files, tt.args...)
			usageErr, ok := errors.AsType[UsageError](err)
			if !ok || !strings.Contains(usageErr.Message, tt.want) {
				t.Fatalf("err = %v, want UsageError containing %q", err, tt.want)
			}
			if len(files.written) != 0 {
				t.Fatalf("a rejected command must not write: %v", files.written)
			}
		})
	}
}

func TestRepoFlagOverridesTheWorkingDirectory(t *testing.T) {
	files := newFakeFiles()
	var out bytes.Buffer
	deps := files.deps(&out)
	deps.Getwd = func() (string, error) { return "", errors.New("cwd must not be consulted") }
	var loaded []string
	load := deps.Models.Load
	deps.Models.Load = func(ctx context.Context, repo string) (agentmodelapp.Settings, error) {
		loaded = append(loaded, repo)
		return load(ctx, repo)
	}
	if err := Run(deps, []string{"show", "--repo", "/other"}); err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0] != "/other" {
		t.Fatalf("loaded repos = %v", loaded)
	}
}

func TestWorkingDirectoryFailureIsAFileErrorForEverySubcommand(t *testing.T) {
	cwdErr := errors.New("getwd: gone")
	for _, args := range [][]string{
		{"show"},
		{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "opus"},
		{"unset", "--scope", "global", "--host", "claude", "--role", "implement"},
		{"resolve", "--host", "claude", "--role", "implement"},
	} {
		files := newFakeFiles()
		var out bytes.Buffer
		deps := files.deps(&out)
		deps.Getwd = func() (string, error) { return "", cwdErr }
		err := Run(deps, args)
		if !errors.Is(err, cwdErr) {
			t.Fatalf("%v: err = %v", args, err)
		}
		if _, isUsage := errors.AsType[UsageError](err); isUsage || len(files.written) != 0 {
			t.Fatalf("%v: usage=%v written=%v", args, isUsage, files.written)
		}
	}
}

func TestShowTextMarksMissingLocalPathAndSources(t *testing.T) {
	files := newFakeFiles()
	files.settings.LocalPath = ""
	got, err := run(t, files, "show", "--host", "claude")
	if err != nil {
		t.Fatal(err)
	}
	text := got["text"].(string)
	if !strings.HasPrefix(text, "local:  -\nglobal: /xdg/issueops/agent-models.json\n") {
		t.Fatalf("header = %q", text)
	}
	if strings.Contains(text, "codex  ") || strings.Count(text, "claude  ") != 7 {
		t.Fatalf("--host claude must list only the 7 claude roles: %q", text)
	}
	for _, role := range []struct{ name, sources string }{
		{"implement", "(default, default)"},
		{"child-implement", "(inherited, inherited)"},
	} {
		var line string
		for l := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(l, "claude  "+role.name+" ") {
				line = l
			}
		}
		if !strings.HasSuffix(line, role.sources) {
			t.Fatalf("%s line = %q, want sources %s", role.name, line, role.sources)
		}
	}
}

func TestShowAcceptsHostsThatCannotBeConfigured(t *testing.T) {
	got, err := run(t, newFakeFiles(), "show", "--host", "omp", "--json")
	if err != nil {
		t.Fatal(err)
	}
	hosts := got["hosts"].(map[string]any)
	if len(hosts) != 1 || len(hosts["omp"].([]any)) != 7 {
		t.Fatalf("hosts = %v", hosts)
	}
}

func TestSetStampsTheConfigVersionAndPrintsTextWithWarnings(t *testing.T) {
	files := newFakeFiles()
	got, err := run(t, files, "set", "--scope", "global", "--host", "claude", "--role", "research", "--model", "mystery")
	if err != nil {
		t.Fatal(err)
	}
	text := got["text"].(string)
	want := "global claude.research = mystery / - (/xdg/issueops/agent-models.json)\nwarning: model \"mystery\" is not a Claude alias or claude- model id\n"
	if text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	if files.written["/xdg/issueops/agent-models.json"].Version != contract.ConfigVersion {
		t.Fatalf("written config = %+v", files.written)
	}
}

func TestSetWithoutAnyValueIsAUsageError(t *testing.T) {
	_, err := run(t, newFakeFiles(), "set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "  ", "--effort", " ")
	usageErr, ok := errors.AsType[UsageError](err)
	if !ok || !strings.Contains(usageErr.Message, "set needs a model or effort") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetDoesNotMutateTheLoadedSettings(t *testing.T) {
	files := newFakeFiles()
	original := contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Model: "opus"}}}
	files.settings.Global = original
	var out bytes.Buffer
	deps := files.deps(&out)
	deps.WriteGlobal = func(string, contract.Config) error { return nil }
	if err := Run(deps, []string{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--effort", "high"}); err != nil {
		t.Fatal(err)
	}
	if original.Claude[contract.RoleImplement] != (contract.Layer{Model: "opus"}) {
		t.Fatalf("set edited the loaded settings in place: %+v", original.Claude)
	}
}

func TestSetWithoutACodexCatalogDoesNotWarnAboutModels(t *testing.T) {
	files := newFakeFiles()
	var out bytes.Buffer
	deps := files.deps(&out)
	deps.CodexCatalog = nil
	if err := Run(deps, []string{"set", "--scope", "global", "--host", "codex", "--role", "research", "--model", "gpt-unknown"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "warning") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLocalScopeNeedsAGitRepository(t *testing.T) {
	for _, args := range [][]string{
		{"set", "--scope", "local", "--host", "claude", "--role", "implement", "--model", "opus"},
	} {
		files := newFakeFiles()
		files.settings.LocalPath = ""
		_, err := run(t, files, args...)
		if err == nil || !strings.Contains(err.Error(), "local settings need a git repository") {
			t.Fatalf("%v: err = %v", args, err)
		}
		if _, isUsage := errors.AsType[UsageError](err); isUsage || len(files.written) != 0 {
			t.Fatalf("usage=%v written=%v", isUsage, files.written)
		}
	}
	files := newFakeFiles()
	files.settings.LocalPath = ""
	files.settings.Local = contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Model: "opus"}}}
	_, err := run(t, files, "unset", "--scope", "local", "--host", "claude", "--role", "implement")
	if err == nil || !strings.Contains(err.Error(), "local settings need a git repository") {
		t.Fatalf("unset err = %v", err)
	}
}

func TestSetLocalReportsWhetherTheExcludeWasAdded(t *testing.T) {
	files := newFakeFiles()
	files.excluded = false
	got, err := run(t, files, "set", "--scope", "local", "--host", "claude", "--role", "implement", "--model", "opus", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if excluded, present := got["excluded"]; !present || excluded != false {
		t.Fatalf("excluded = %v (present=%v), want an explicit false", excluded, present)
	}
	globalFiles := newFakeFiles()
	got, err = run(t, globalFiles, "set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "opus", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := got["excluded"]; present {
		t.Fatalf("a global write has no exclude to report: %v", got)
	}
}

func TestWriteFailuresAreReturnedWithoutReportingSuccess(t *testing.T) {
	writeErr := errors.New("disk full")
	seeded := contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Model: "opus"}}}
	tests := []struct {
		name string
		args []string
	}{
		{"set global", []string{"set", "--scope", "global", "--host", "claude", "--role", "implement", "--model", "opus"}},
		{"set local", []string{"set", "--scope", "local", "--host", "claude", "--role", "implement", "--model", "opus"}},
		{"unset global", []string{"unset", "--scope", "global", "--host", "claude", "--role", "implement"}},
		{"unset local", []string{"unset", "--scope", "local", "--host", "claude", "--role", "implement"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := newFakeFiles()
			files.settings.Global, files.settings.Local = seeded, seeded
			var out bytes.Buffer
			deps := files.deps(&out)
			deps.WriteGlobal = func(string, contract.Config) error { return writeErr }
			deps.WriteLocal = func(context.Context, string, contract.Config) (bool, error) { return false, writeErr }
			err := Run(deps, tt.args)
			if !errors.Is(err, writeErr) || out.Len() != 0 {
				t.Fatalf("err = %v output = %q", err, out.String())
			}
		})
	}
}

func TestUnsetTextReportsWhatWasRemovedOrAbsent(t *testing.T) {
	files := newFakeFiles()
	files.settings.Local = contract.Config{Version: 1, Codex: map[contract.Role]contract.Layer{contract.RoleResearch: {Model: "gpt-6-luna", Effort: "low"}}}
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--field", "effort"}, "unset local codex.research effort (/repo/.issueops/agent-models.local.json)\n"},
		{nil, "unset local codex.research (/repo/.issueops/agent-models.local.json)\n"},
		{[]string{"--field", "model"}, "local codex.research was not set (/repo/.issueops/agent-models.local.json)\n"},
	}
	for _, tt := range tests {
		args := append([]string{"unset", "--scope", "local", "--host", "codex", "--role", "research"}, tt.args...)
		got, err := run(t, files, args...)
		if err != nil {
			t.Fatal(err)
		}
		if got["text"] != tt.want {
			t.Fatalf("%v: text = %q, want %q", tt.args, got["text"], tt.want)
		}
	}
}

func TestUnsetOfAnAbsentFieldLeavesTheFileUntouched(t *testing.T) {
	files := newFakeFiles()
	files.settings.Global = contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Effort: "high"}}}
	got, err := run(t, files, "unset", "--scope", "global", "--host", "claude", "--role", "implement", "--field", "model", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got["removed"] != false || got["field"] != "model" || len(files.written) != 0 {
		t.Fatalf("unset = %v written=%v", got, files.written)
	}
	if files.settings.Global.Claude[contract.RoleImplement] != (contract.Layer{Effort: "high"}) {
		t.Fatalf("settings = %+v", files.settings.Global)
	}
}

func TestUnsetLocalFieldWritesTheRemainderAndReportsExclude(t *testing.T) {
	files := newFakeFiles()
	files.settings.Local = contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{
		contract.RoleImplement: {Model: "opus", Effort: "high"},
		contract.RoleResearch:  {Model: "haiku"},
	}}
	got, err := run(t, files, "unset", "--scope", "local", "--host", "claude", "--role", "implement", "--field", "effort", "--json")
	if err != nil {
		t.Fatal(err)
	}
	local := files.written[files.settings.LocalPath]
	if got["removed"] != true || got["excluded"] != true || local.Version != contract.ConfigVersion {
		t.Fatalf("unset = %v local=%+v", got, local)
	}
	if local.Claude[contract.RoleImplement] != (contract.Layer{Model: "opus"}) || local.Claude[contract.RoleResearch] != (contract.Layer{Model: "haiku"}) {
		t.Fatalf("other fields and roles must survive: %+v", local.Claude)
	}
}

func TestResolveTextAndOptionalAgentArguments(t *testing.T) {
	files := newFakeFiles()
	got, err := run(t, files, "resolve", "--host", "claude", "--role", "implement", "--model", "sonnet")
	if err != nil {
		t.Fatal(err)
	}
	if text := got["text"].(string); !strings.HasPrefix(text, "claude implement: sonnet / ") || !strings.HasSuffix(text, "\n") {
		t.Fatalf("text = %q", text)
	}
	if files.agentCalls != 0 {
		t.Fatalf("role-agent arguments rendered without --agents: %d calls", files.agentCalls)
	}
	json, err := run(t, files, "resolve", "--host", "claude", "--role", "implement", "--model", "sonnet", "--effort", "high", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := json["agent_args"]; present {
		t.Fatalf("agent_args must be omitted without --agents: %v", json)
	}
	argv := json["argv"].([]any)
	if len(argv) != 3 || argv[0] != "claude" || argv[1] != "sonnet" || argv[2] != "high" {
		t.Fatalf("argv = %v", argv)
	}
}

func TestResolveReportsAnEmptyArgvAsAnEmptyList(t *testing.T) {
	files := newFakeFiles()
	var out bytes.Buffer
	deps := files.deps(&out)
	deps.PrintArgv = func(string, string, string) []string { return nil }
	if err := Run(deps, []string{"resolve", "--host", "claude", "--role", "implement", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"argv": []`) && !strings.Contains(out.String(), `"argv":[]`) {
		t.Fatalf("argv must serialize as [], got %s", out.String())
	}
}

func TestResolveSurfacesARoleAgentRenderingFailure(t *testing.T) {
	renderErr := errors.New("cannot render roles")
	files := newFakeFiles()
	var out bytes.Buffer
	deps := files.deps(&out)
	deps.RoleAgentArgs = func(context.Context, string, string) ([]string, error) { return nil, renderErr }
	err := Run(deps, []string{"resolve", "--host", "codex", "--role", "research", "--agents", "--json"})
	if !errors.Is(err, renderErr) || out.Len() != 0 {
		t.Fatalf("err = %v output = %q", err, out.String())
	}
}
