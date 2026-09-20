package omo

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

type generatedLifecycleContract struct {
	SchemaVersion int                               `json:"schema_version"`
	Events        map[string]generatedLifecycleRule `json:"events"`
	Message       generatedLifecycleMessage         `json:"message"`
	Warning       string                            `json:"warning"`
}

type generatedLifecycleRule struct {
	Subcommand   string `json:"subcommand"`
	AcceptedOnly bool   `json:"accepted_only"`
}

type generatedLifecycleMessage struct {
	CustomType  string `json:"custom_type"`
	Display     bool   `json:"display"`
	TriggerTurn bool   `json:"trigger_turn"`
}

type mockLifecycleExec struct {
	Code   int
	Stdout string
	Err    error
}

type lifecycleExecCall struct {
	Binary  string
	Argv    []string
	Options map[string]any
}

type lifecycleMessageCall struct {
	Message map[string]any
	Options map[string]any
}

type lifecycleNotification struct {
	Message string
	Level   string
}

type lifecycleModuleObservation struct {
	ExecCalls     []lifecycleExecCall
	Messages      []lifecycleMessageCall
	Notifications []lifecycleNotification
	HandlerState  goja.PromiseState
	HandlerResult any
}

func TestGeneratedLifecycleExtensionExecutesActualMockPiModule(t *testing.T) {
	tests := []struct {
		name        string
		event       string
		accepted    bool
		exec        mockLifecycleExec
		wantArgv    []string
		wantWarn    int
		wantSend    int
		wantContent string
	}{
		{
			name: "session start invokes session-start and injects hidden context", event: "session_start",
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`},
			wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}, wantSend: 1, wantContent: "catalog",
		},
		{
			name: "accepted compact invokes post-compact", event: "session_compact", accepted: true,
			exec:     mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"restored"}`},
			wantArgv: []string{"hook", "post-compact", "--repo", "/repo", "--json"}, wantSend: 1, wantContent: "restored",
		},
		{name: "rejected compact invokes nothing", event: "session_compact"},
		{name: "malformed hook json warns without sending", event: "session_start", exec: mockLifecycleExec{Stdout: `{`}, wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}, wantWarn: 1},
		{name: "nonzero hook exit warns without sending", event: "session_start", exec: mockLifecycleExec{Code: 7}, wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}, wantWarn: 1},
		{name: "hook execution error warns without sending", event: "session_start", exec: mockLifecycleExec{Err: errors.New("exec failed")}, wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}, wantWarn: 1},
		{name: "no inject sends nothing", event: "session_start", exec: mockLifecycleExec{Stdout: `{"should_inject":false,"compact":"catalog"}`}, wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}},
		{name: "empty compact sends nothing", event: "session_start", exec: mockLifecycleExec{Stdout: `{"should_inject":true,"compact":""}`}, wantArgv: []string{"hook", "session-start", "--repo", "/repo", "--json"}},
	}

	source := LifecycleExtension("/private/bin/issueops")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := executeLifecycleModule(source, test.event, test.accepted, test.exec)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantArgv == nil {
				if len(got.ExecCalls) != 0 {
					t.Fatalf("exec calls = %#v", got.ExecCalls)
				}
			} else {
				want := lifecycleExecCall{Binary: "/private/bin/issueops", Argv: test.wantArgv, Options: map[string]any{"cwd": "/repo"}}
				if !reflect.DeepEqual(got.ExecCalls, []lifecycleExecCall{want}) {
					t.Fatalf("exec calls = %#v, want %#v", got.ExecCalls, want)
				}
			}
			if len(got.Notifications) != test.wantWarn {
				t.Fatalf("notifications = %#v, want count %d", got.Notifications, test.wantWarn)
			}
			if len(got.Messages) != test.wantSend {
				t.Fatalf("messages = %#v, want count %d", got.Messages, test.wantSend)
			}
			if test.wantArgv != nil && got.HandlerState != goja.PromiseStateFulfilled {
				t.Fatalf("handler promise state = %v, want fulfilled", got.HandlerState)
			}
			if test.wantArgv != nil && got.HandlerResult != nil {
				t.Fatalf("handler promise result = %#v, want undefined", got.HandlerResult)
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

func TestGeneratedLifecycleExtensionProofRejectsRuntimeMutations(t *testing.T) {
	source := LifecycleExtension("/private/bin/issueops")
	if err := verifyLifecycleModule(source); err != nil {
		t.Fatalf("generated module failed baseline proof: %v", err)
	}
	mutations := []struct {
		name string
		old  string
		new  string
	}{
		{name: "accepted filter removed", old: "if (rule.accepted_only && !event.accepted) return", new: ""},
		{name: "hook argv changed", old: `["hook", rule.subcommand, "--repo", ctx.cwd, "--json"]`, new: `["hook", rule.subcommand, "--json"]`},
		{name: "public sendMessage renamed", old: "pi.sendMessage(", new: "pi.sendCustomMessage("},
		{name: "display made visible", old: `"display":false`, new: `"display":true`},
		{name: "trigger turn enabled", old: `"trigger_turn":false`, new: `"trigger_turn":true`},
		{name: "await removed", old: "const result = await pi.exec(", new: "const result = pi.exec("},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(source, mutation.old, mutation.new, 1)
			if mutated == source {
				t.Fatalf("mutation target %q missing", mutation.old)
			}
			if err := verifyLifecycleModule(mutated); err == nil {
				t.Fatal("runtime proof accepted mutated generated module")
			}
		})
	}
}

func verifyLifecycleModule(source string) error {
	rejected, err := executeLifecycleModule(source, "session_compact", false, mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"rejected"}`})
	if err != nil {
		return err
	}
	if len(rejected.ExecCalls) != 0 || len(rejected.Messages) != 0 || len(rejected.Notifications) != 0 {
		return fmt.Errorf("rejected compact produced side effects")
	}
	accepted, err := executeLifecycleModule(source, "session_start", true, mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`})
	if err != nil {
		return err
	}
	wantExec := []lifecycleExecCall{{
		Binary:  "/private/bin/issueops",
		Argv:    []string{"hook", "session-start", "--repo", "/repo", "--json"},
		Options: map[string]any{"cwd": "/repo"},
	}}
	wantMessage := []lifecycleMessageCall{{
		Message: map[string]any{"customType": "issueops:project-docs", "content": "catalog", "display": false},
		Options: map[string]any{"triggerTurn": false},
	}}
	if !reflect.DeepEqual(accepted.ExecCalls, wantExec) || !reflect.DeepEqual(accepted.Messages, wantMessage) || len(accepted.Notifications) != 0 ||
		accepted.HandlerState != goja.PromiseStateFulfilled || accepted.HandlerResult != nil {
		return fmt.Errorf("accepted lifecycle behavior drifted")
	}
	return nil
}

func executeLifecycleModule(source, eventName string, accepted bool, execResult mockLifecycleExec) (lifecycleModuleObservation, error) {
	const declaration = "export default function agentHarness(pi)"
	if strings.Count(source, declaration) != 1 {
		return lifecycleModuleObservation{}, fmt.Errorf("generated module default export drifted")
	}
	script := strings.Replace(source, declaration, "function agentHarness(pi)", 1) + "\nglobalThis.__issueopsExtension = agentHarness\n"
	runtime := goja.New()
	observation := lifecycleModuleObservation{}
	handlers := map[string]goja.Callable{}

	pi := runtime.NewObject()
	if err := pi.Set("on", func(call goja.FunctionCall) goja.Value {
		event := call.Argument(0).String()
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(runtime.NewTypeError("handler is not callable"))
		}
		handlers[event] = handler
		return goja.Undefined()
	}); err != nil {
		return lifecycleModuleObservation{}, err
	}
	if err := pi.Set("exec", func(call goja.FunctionCall) goja.Value {
		argv, err := exportStringSlice(call.Argument(1))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		options, err := exportObject(call.Argument(2))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		observation.ExecCalls = append(observation.ExecCalls, lifecycleExecCall{Binary: call.Argument(0).String(), Argv: argv, Options: options})
		promise, resolve, reject := runtime.NewPromise()
		if execResult.Err != nil {
			if err := reject(runtime.NewGoError(execResult.Err)); err != nil {
				panic(runtime.NewGoError(err))
			}
		} else if err := resolve(map[string]any{"code": execResult.Code, "stdout": execResult.Stdout}); err != nil {
			panic(runtime.NewGoError(err))
		}
		return runtime.ToValue(promise)
	}); err != nil {
		return lifecycleModuleObservation{}, err
	}
	if err := pi.Set("sendMessage", func(call goja.FunctionCall) goja.Value {
		message, err := exportObject(call.Argument(0))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		options, err := exportObject(call.Argument(1))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		observation.Messages = append(observation.Messages, lifecycleMessageCall{Message: message, Options: options})
		return goja.Undefined()
	}); err != nil {
		return lifecycleModuleObservation{}, err
	}

	if _, err := runtime.RunString(script); err != nil {
		return lifecycleModuleObservation{}, fmt.Errorf("evaluate generated module: %w", err)
	}
	register, ok := goja.AssertFunction(runtime.Get("__issueopsExtension"))
	if !ok {
		return lifecycleModuleObservation{}, fmt.Errorf("generated module did not expose default extension")
	}
	if _, err := register(goja.Undefined(), pi); err != nil {
		return lifecycleModuleObservation{}, fmt.Errorf("register generated module: %w", err)
	}
	handler := handlers[eventName]
	if handler == nil {
		return lifecycleModuleObservation{}, fmt.Errorf("generated module did not register %s", eventName)
	}
	ui := runtime.NewObject()
	if err := ui.Set("notify", func(call goja.FunctionCall) goja.Value {
		observation.Notifications = append(observation.Notifications, lifecycleNotification{Message: call.Argument(0).String(), Level: call.Argument(1).String()})
		return goja.Undefined()
	}); err != nil {
		return lifecycleModuleObservation{}, err
	}
	ctx := runtime.NewObject()
	if err := ctx.Set("cwd", "/repo"); err != nil {
		return lifecycleModuleObservation{}, err
	}
	if err := ctx.Set("ui", ui); err != nil {
		return lifecycleModuleObservation{}, err
	}
	event := runtime.ToValue(map[string]any{"accepted": accepted})
	returned, err := handler(goja.Undefined(), event, ctx)
	if err != nil {
		return lifecycleModuleObservation{}, fmt.Errorf("invoke %s: %w", eventName, err)
	}
	if _, err := runtime.RunString("void 0"); err != nil {
		return lifecycleModuleObservation{}, fmt.Errorf("drain %s jobs: %w", eventName, err)
	}
	if returned != nil && !goja.IsUndefined(returned) {
		promise, ok := returned.Export().(*goja.Promise)
		if !ok {
			return lifecycleModuleObservation{}, fmt.Errorf("%s handler did not return a promise", eventName)
		}
		observation.HandlerState = promise.State()
		if result := promise.Result(); result != nil && !goja.IsUndefined(result) {
			observation.HandlerResult = result.Export()
		}
	}
	return observation, nil
}

func exportStringSlice(value goja.Value) ([]string, error) {
	raw, ok := value.Export().([]any)
	if !ok {
		return nil, fmt.Errorf("value is not an array")
	}
	result := make([]string, len(raw))
	for i, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("array item %d is not a string", i)
		}
		result[i] = text
	}
	return result, nil
}

func exportObject(value goja.Value) (map[string]any, error) {
	object, ok := value.Export().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("value is not an object")
	}
	return object, nil
}

func parseGeneratedLifecycleContract(t *testing.T, source string) generatedLifecycleContract {
	t.Helper()
	const prefix = "const issueopsLifecycleContract = "
	start := strings.Index(source, prefix)
	if start < 0 {
		t.Fatal("generated extension has no lifecycle contract")
	}
	remaining := source[start+len(prefix):]
	end := strings.IndexByte(remaining, '\n')
	if end < 0 {
		t.Fatal("generated lifecycle contract is not line-delimited")
	}
	var contract generatedLifecycleContract
	if err := json.Unmarshal([]byte(remaining[:end]), &contract); err != nil {
		t.Fatalf("decode generated lifecycle contract: %v", err)
	}
	return contract
}
