package hostprotocol

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestGeneratedOmpLifecycleExtensionExecutesActualMockPiModule(t *testing.T) {
	sessionStartArgv := []string{"hook", "session-start", "--repo", "/repo", "--json"}
	exported := map[string]any{ompSessionIDEnv: mockLifecycleSessionID}
	tests := []struct {
		name        string
		event       string
		agentKind   string
		exec        mockLifecycleExec
		wantArgv    []string
		wantWarn    int
		wantSend    int
		wantContent string
		wantEnv     map[string]any
	}{
		{
			name: "main session start exports session id before injecting hidden context", event: "session_start", agentKind: "main",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`},
			wantArgv: sessionStartArgv, wantSend: 1, wantContent: "catalog", wantEnv: exported,
		},
		{
			name: "main session switch exports session id and reinjects", event: "session_switch", agentKind: "main",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"switched"}`},
			wantArgv: sessionStartArgv, wantSend: 1, wantContent: "switched", wantEnv: exported,
		},
		{
			name: "compact without accepted flag invokes post-compact without exporting", event: "session_compact", agentKind: "main",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"restored"}`},
			wantArgv: []string{"hook", "post-compact", "--repo", "/repo", "--json"}, wantSend: 1, wantContent: "restored", wantEnv: map[string]any{},
		},
		{
			name: "subagent session start runs hook without exporting", event: "session_start", agentKind: "sub",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`},
			wantArgv: sessionStartArgv, wantSend: 1, wantContent: "catalog", wantEnv: map[string]any{},
		},
		{
			name: "subagent session switch runs hook without exporting", event: "session_switch", agentKind: "sub",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":false}`},
			wantArgv: sessionStartArgv, wantEnv: map[string]any{},
		},
		{
			name: "missing agent kind runs hook without exporting", event: "session_start",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":false}`},
			wantArgv: sessionStartArgv, wantEnv: map[string]any{},
		},
		{name: "malformed hook json still exports for main", event: "session_start", agentKind: "main", exec: mockLifecycleExec{Stdout: `{`}, wantArgv: sessionStartArgv, wantWarn: 1, wantEnv: exported},
		{name: "nonzero hook exit still exports for main", event: "session_start", agentKind: "main", exec: mockLifecycleExec{Code: 7}, wantArgv: sessionStartArgv, wantWarn: 1, wantEnv: exported},
		{name: "hook execution error still exports for main", event: "session_switch", agentKind: "main", exec: mockLifecycleExec{Err: errors.New("exec failed")}, wantArgv: sessionStartArgv, wantWarn: 1, wantEnv: exported},
		{name: "empty compact sends nothing", event: "session_start", agentKind: "main", exec: mockLifecycleExec{Stdout: `{"should_inject":true,"compact":""}`}, wantArgv: sessionStartArgv, wantEnv: exported},
	}

	source := OmpLifecycleExtension("/private/bin/issueops")
	contract := parseGeneratedLifecycleContract(t, source)
	if contract.Events["session_start"] != (generatedLifecycleRule{Subcommand: "session-start"}) ||
		contract.Events["session_switch"] != (generatedLifecycleRule{Subcommand: "session-start"}) ||
		contract.Events["session_compact"] != (generatedLifecycleRule{Subcommand: "post-compact"}) ||
		len(contract.Events) != 3 ||
		contract.Message != (generatedLifecycleMessage{CustomType: "issueops:project-docs"}) ||
		contract.SessionEnv == nil ||
		!reflect.DeepEqual(*contract.SessionEnv, generatedLifecycleSessionEnv{
			Variable: ompSessionIDEnv, AgentKind: "main", Events: []string{"session_start", "session_switch"},
		}) {
		t.Fatalf("unexpected omp lifecycle contract: %+v", contract)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := executeLifecycleModuleAs(source, test.event, map[string]any{}, test.agentKind, test.exec)
			if err != nil {
				t.Fatal(err)
			}
			want := lifecycleExecCall{Binary: "/private/bin/issueops", Argv: test.wantArgv, Options: map[string]any{"cwd": "/repo"}}
			if !reflect.DeepEqual(got.ExecCalls, []lifecycleExecCall{want}) {
				t.Fatalf("exec calls = %#v, want %#v", got.ExecCalls, want)
			}
			if !reflect.DeepEqual(got.ExecEnv, []map[string]any{test.wantEnv}) {
				t.Fatalf("process.env at hook exec = %#v, want %#v", got.ExecEnv, test.wantEnv)
			}
			if !reflect.DeepEqual(got.ProcessEnv, test.wantEnv) {
				t.Fatalf("process.env after handler = %#v, want %#v", got.ProcessEnv, test.wantEnv)
			}
			if len(got.Notifications) != test.wantWarn {
				t.Fatalf("notifications = %#v, want count %d", got.Notifications, test.wantWarn)
			}
			if len(got.Messages) != test.wantSend {
				t.Fatalf("messages = %#v, want count %d", got.Messages, test.wantSend)
			}
			if got.HandlerState != goja.PromiseStateFulfilled || got.HandlerResult != nil {
				t.Fatalf("handler promise = %v %#v, want fulfilled undefined", got.HandlerState, got.HandlerResult)
			}
			if test.wantSend == 1 {
				want := lifecycleMessageCall{
					Message: map[string]any{"customType": "issueops:project-docs", "content": test.wantContent, "display": false},
					Options: map[string]any{"triggerTurn": false},
				}
				if !reflect.DeepEqual(got.Messages[0], want) {
					t.Fatalf("message = %#v, want %#v", got.Messages[0], want)
				}
			}
		})
	}
}

func TestGeneratedOmpLifecycleExtensionProofRejectsRuntimeMutations(t *testing.T) {
	source := OmpLifecycleExtension("/private/bin/issueops")
	if err := verifyOmpLifecycleModule(source); err != nil {
		t.Fatalf("generated module failed baseline proof: %v", err)
	}
	mutations := []struct {
		name string
		old  string
		new  string
	}{
		{name: "main agent filter removed", old: "if (ctx.agent?.kind !== sessionEnv.agent_kind) return", new: ""},
		{name: "session env export removed", old: "      exportIssueopsSessionEnv(eventName, ctx)\n", new: ""},
		{name: "switch event dropped", old: `,"session_switch":{"subcommand":"session-start","accepted_only":false}`, new: ""},
		{name: "compact made accepted only", old: `"post-compact","accepted_only":false`, new: `"post-compact","accepted_only":true`},
		{name: "hook argv changed", old: `["hook", rule.subcommand, "--repo", ctx.cwd, "--json"]`, new: `["hook", rule.subcommand, "--json"]`},
		{name: "display made visible", old: `"display":false`, new: `"display":true`},
		{name: "await removed", old: "const result = await pi.exec(", new: "const result = pi.exec("},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(source, mutation.old, mutation.new, 1)
			if mutated == source {
				t.Fatalf("mutation target %q missing", mutation.old)
			}
			if err := verifyOmpLifecycleModule(mutated); err == nil {
				t.Fatal("runtime proof accepted mutated generated module")
			}
		})
	}
}

func verifyOmpLifecycleModule(source string) error {
	wantMessage := []lifecycleMessageCall{{
		Message: map[string]any{"customType": "issueops:project-docs", "content": "catalog", "display": false},
		Options: map[string]any{"triggerTurn": false},
	}}
	for _, check := range []struct {
		event, kind, subcommand string
		wantEnv                 map[string]any
	}{
		{event: "session_start", kind: "main", subcommand: "session-start", wantEnv: map[string]any{ompSessionIDEnv: mockLifecycleSessionID}},
		{event: "session_switch", kind: "main", subcommand: "session-start", wantEnv: map[string]any{ompSessionIDEnv: mockLifecycleSessionID}},
		{event: "session_start", kind: "sub", subcommand: "session-start", wantEnv: map[string]any{}},
		{event: "session_compact", kind: "main", subcommand: "post-compact", wantEnv: map[string]any{}},
	} {
		got, err := executeLifecycleModuleAs(source, check.event, map[string]any{}, check.kind, mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`})
		if err != nil {
			return err
		}
		wantExec := []lifecycleExecCall{{
			Binary:  "/private/bin/issueops",
			Argv:    []string{"hook", check.subcommand, "--repo", "/repo", "--json"},
			Options: map[string]any{"cwd": "/repo"},
		}}
		if !reflect.DeepEqual(got.ExecCalls, wantExec) || !reflect.DeepEqual(got.Messages, wantMessage) || len(got.Notifications) != 0 ||
			!reflect.DeepEqual(got.ExecEnv, []map[string]any{check.wantEnv}) ||
			got.HandlerState != goja.PromiseStateFulfilled || got.HandlerResult != nil {
			return fmt.Errorf("%s lifecycle behavior for %s agent drifted", check.event, check.kind)
		}
	}
	return nil
}
