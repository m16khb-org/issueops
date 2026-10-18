package hostprotocol

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

const opencodeTestBin = "/private/bin/issueops"

// opencodeSpawnResult scripts one mock Bun.spawn call.
type opencodeSpawnResult struct {
	Throw   bool
	Stdout  string
	Code    int
	TimeOut bool
}

type opencodeSpawnCall struct {
	Argv  []string
	Cwd   string
	Stdin string
}

type opencodeSession struct {
	ParentID  string
	Directory string
	GetFails  bool
}

type opencodeContextCall struct {
	SessionID string
	Agent     string
}

type opencodePluginObservation struct {
	Spawns   []opencodeSpawnCall
	Kills    int
	Errors   []string
	Pushes   [][]map[string]any
	Settled  []goja.PromiseState
	HookName string
}

// runOpencodePlugin evaluates the generated module in goja with mock opencode
// (ctx.session.hook/get, ctx.location) and Bun (spawn, readableStreamToText)
// runtimes, then calls the registered context hook once per call.
func runOpencodePlugin(source string, sessions map[string]opencodeSession, spawn func(index int) opencodeSpawnResult, calls []opencodeContextCall) (opencodePluginObservation, error) {
	const declaration = "export default {"
	if strings.Count(source, declaration) != 1 {
		return opencodePluginObservation{}, fmt.Errorf("generated module default export drifted")
	}
	script := strings.Replace(source, declaration, "globalThis.__issueopsPlugin = {", 1)
	runtime := goja.New()
	observation := opencodePluginObservation{}
	pendingTimeout := false

	if err := runtime.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(runtime.NewTypeError("timer callback is not callable"))
		}
		if pendingTimeout {
			// The scripted hook outlives its limit: fire the timer at once.
			pendingTimeout = false
			if _, err := callback(goja.Undefined()); err != nil {
				panic(err)
			}
		}
		return runtime.ToValue(1)
	}); err != nil {
		return observation, err
	}
	if err := runtime.Set("clearTimeout", func(goja.FunctionCall) goja.Value { return goja.Undefined() }); err != nil {
		return observation, err
	}
	console := runtime.NewObject()
	if err := console.Set("error", func(call goja.FunctionCall) goja.Value {
		observation.Errors = append(observation.Errors, call.Argument(0).String())
		return goja.Undefined()
	}); err != nil {
		return observation, err
	}
	if err := runtime.Set("console", console); err != nil {
		return observation, err
	}

	bun := runtime.NewObject()
	if err := bun.Set("spawn", func(call goja.FunctionCall) goja.Value {
		argv, err := exportStringSlice(call.Argument(0))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		options, err := exportObject(call.Argument(1))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		index := len(observation.Spawns)
		cwd, _ := options["cwd"].(string)
		observation.Spawns = append(observation.Spawns, opencodeSpawnCall{Argv: argv, Cwd: cwd})
		result := spawn(index)
		if result.Throw {
			panic(runtime.NewGoError(errors.New("spawn failed")))
		}
		exited, resolveExit, _ := runtime.NewPromise()
		proc := runtime.NewObject()
		stdin := runtime.NewObject()
		_ = stdin.Set("write", func(call goja.FunctionCall) goja.Value {
			observation.Spawns[index].Stdin += call.Argument(0).String()
			return goja.Undefined()
		})
		_ = stdin.Set("end", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
		if options["stdin"] == "pipe" {
			_ = proc.Set("stdin", stdin)
		}
		_ = proc.Set("stdout", result.Stdout)
		_ = proc.Set("exited", exited)
		_ = proc.Set("kill", func(goja.FunctionCall) goja.Value {
			observation.Kills++
			_ = resolveExit(143)
			return goja.Undefined()
		})
		if result.TimeOut {
			pendingTimeout = true
		} else {
			_ = resolveExit(result.Code)
		}
		return proc
	}); err != nil {
		return observation, err
	}
	if err := bun.Set("readableStreamToText", func(call goja.FunctionCall) goja.Value {
		promise, resolve, _ := runtime.NewPromise()
		_ = resolve(call.Argument(0).String())
		return runtime.ToValue(promise)
	}); err != nil {
		return observation, err
	}
	if err := runtime.Set("Bun", bun); err != nil {
		return observation, err
	}

	if _, err := runtime.RunString(script); err != nil {
		return observation, fmt.Errorf("evaluate generated module: %w", err)
	}
	plugin, ok := runtime.Get("__issueopsPlugin").Export().(map[string]any)
	if !ok || plugin["id"] != "issueops" {
		return observation, fmt.Errorf("generated module default export is not the issueops plugin")
	}
	setup, ok := goja.AssertFunction(runtime.Get("__issueopsPlugin").ToObject(runtime).Get("setup"))
	if !ok {
		return observation, fmt.Errorf("generated plugin has no setup")
	}

	var hook goja.Callable
	session := runtime.NewObject()
	_ = session.Set("hook", func(call goja.FunctionCall) goja.Value {
		observation.HookName = call.Argument(0).String()
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(runtime.NewTypeError("hook handler is not callable"))
		}
		hook = handler
		return goja.Undefined()
	})
	_ = session.Set("get", func(call goja.FunctionCall) goja.Value {
		request, err := exportObject(call.Argument(0))
		if err != nil {
			panic(runtime.NewTypeError(err.Error()))
		}
		id, _ := request["sessionID"].(string)
		promise, resolve, reject := runtime.NewPromise()
		info := sessions[id]
		if info.GetFails {
			_ = reject(runtime.NewGoError(errors.New("session lookup failed")))
			return runtime.ToValue(promise)
		}
		value := map[string]any{"id": id}
		if info.ParentID != "" {
			value["parentID"] = info.ParentID
		}
		if info.Directory != "" {
			value["location"] = map[string]any{"directory": info.Directory}
		}
		_ = resolve(value)
		return runtime.ToValue(promise)
	})
	ctx := runtime.NewObject()
	_ = ctx.Set("session", session)
	_ = ctx.Set("location", map[string]any{"directory": "/repo"})
	if _, err := setup(goja.Undefined(), ctx); err != nil {
		return observation, fmt.Errorf("setup: %w", err)
	}
	if hook == nil {
		return observation, fmt.Errorf("generated plugin registered no session hook")
	}

	for _, call := range calls {
		system := runtime.NewArray(map[string]any{"type": "text", "text": "base"})
		event := runtime.NewObject()
		_ = event.Set("sessionID", call.SessionID)
		_ = event.Set("agent", call.Agent)
		_ = event.Set("system", system)
		returned, err := hook(goja.Undefined(), event)
		if err != nil {
			return observation, fmt.Errorf("context hook threw: %w", err)
		}
		if _, err := runtime.RunString("void 0"); err != nil {
			return observation, err
		}
		promise, ok := returned.Export().(*goja.Promise)
		if !ok {
			return observation, fmt.Errorf("context hook did not return a promise")
		}
		observation.Settled = append(observation.Settled, promise.State())
		exported, _ := system.Export().([]any)
		var pushed []map[string]any
		for _, item := range exported[1:] {
			part, _ := item.(map[string]any)
			pushed = append(pushed, part)
		}
		observation.Pushes = append(observation.Pushes, pushed)
	}
	return observation, nil
}

func catalogPart(text string) []map[string]any {
	return []map[string]any{{"type": "text", "text": text}}
}

func TestGeneratedOpencodeLifecyclePluginExecutesMockContextHook(t *testing.T) {
	source := OpencodeLifecyclePlugin(opencodeTestBin)
	catalog := func(int) opencodeSpawnResult {
		return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"catalog"}`}
	}
	rootArgv := []string{opencodeTestBin, "hook", "session-start", "--repo", "/repo", "--json"}
	tests := []struct {
		name       string
		sessions   map[string]opencodeSession
		spawn      func(int) opencodeSpawnResult
		calls      []opencodeContextCall
		wantSpawns []opencodeSpawnCall
		wantPushes [][]map[string]any
		wantErrors int
		wantKills  int
	}{
		{
			name:       "root session reads the catalog once and pushes it on every call",
			sessions:   map[string]opencodeSession{"ses_root": {Directory: "/repo/sub"}},
			spawn:      catalog,
			calls:      []opencodeContextCall{{"ses_root", "build"}, {"ses_root", "build"}, {"ses_root", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: []string{opencodeTestBin, "hook", "session-start", "--repo", "/repo/sub", "--json"}, Cwd: "/repo/sub"}},
			wantPushes: [][]map[string]any{catalogPart("catalog"), catalogPart("catalog"), catalogPart("catalog")},
		},
		{
			name:     "child session runs subagent-start with its agent type",
			sessions: map[string]opencodeSession{"ses_child": {ParentID: "ses_root"}},
			spawn:    catalog,
			calls:    []opencodeContextCall{{"ses_child", "general"}},
			wantSpawns: []opencodeSpawnCall{{
				Argv: []string{opencodeTestBin, "hook", "subagent-start", "--repo", "/repo", "--json"}, Cwd: "/repo", Stdin: `{"agent_type":"general"}`,
			}},
			wantPushes: [][]map[string]any{catalogPart("catalog")},
		},
		{
			name:     "explore child gets no catalog",
			sessions: map[string]opencodeSession{"ses_child": {ParentID: "ses_root"}},
			spawn:    func(int) opencodeSpawnResult { return opencodeSpawnResult{Stdout: `{}`} },
			calls:    []opencodeContextCall{{"ses_child", "explore"}, {"ses_child", "explore"}},
			wantSpawns: []opencodeSpawnCall{{
				Argv: []string{opencodeTestBin, "hook", "subagent-start", "--repo", "/repo", "--json"}, Cwd: "/repo", Stdin: `{"agent_type":"explore"}`,
			}},
			wantPushes: [][]map[string]any{nil, nil},
		},
		{
			name:       "session lookup failure falls back to the root hook in the plugin location",
			sessions:   map[string]opencodeSession{"ses_x": {GetFails: true}},
			spawn:      catalog,
			calls:      []opencodeContextCall{{"ses_x", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{catalogPart("catalog")},
		},
		{
			name: "no injection pushes nothing",
			spawn: func(int) opencodeSpawnResult {
				return opencodeSpawnResult{Stdout: `{"should_inject":false,"compact":"ignored"}`}
			},
			calls:      []opencodeContextCall{{"ses_a", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{nil},
		},
		{
			name:       "spawn failure is cached as empty and warned once",
			spawn:      func(int) opencodeSpawnResult { return opencodeSpawnResult{Throw: true} },
			calls:      []opencodeContextCall{{"ses_a", "build"}, {"ses_a", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{nil, nil},
			wantErrors: 1,
		},
		{
			name:       "non JSON stdout is a failure",
			spawn:      func(int) opencodeSpawnResult { return opencodeSpawnResult{Stdout: "not json"} },
			calls:      []opencodeContextCall{{"ses_a", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{nil},
			wantErrors: 1,
		},
		{
			name: "non zero exit is a failure",
			spawn: func(int) opencodeSpawnResult {
				return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"catalog"}`, Code: 1}
			},
			calls:      []opencodeContextCall{{"ses_a", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{nil},
			wantErrors: 1,
		},
		{
			name: "timeout kills the hook and is a failure",
			spawn: func(int) opencodeSpawnResult {
				return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"late"}`, TimeOut: true}
			},
			calls:      []opencodeContextCall{{"ses_a", "build"}},
			wantSpawns: []opencodeSpawnCall{{Argv: rootArgv, Cwd: "/repo"}},
			wantPushes: [][]map[string]any{nil},
			wantErrors: 1,
			wantKills:  1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runOpencodePlugin(source, test.sessions, test.spawn, test.calls)
			if err != nil {
				t.Fatal(err)
			}
			if got.HookName != "context" {
				t.Fatalf("registered hook %q, want context", got.HookName)
			}
			if !reflect.DeepEqual(got.Spawns, test.wantSpawns) {
				t.Fatalf("spawns = %#v, want %#v", got.Spawns, test.wantSpawns)
			}
			if !reflect.DeepEqual(got.Pushes, test.wantPushes) {
				t.Fatalf("pushes = %#v, want %#v", got.Pushes, test.wantPushes)
			}
			if len(got.Errors) != test.wantErrors || got.Kills != test.wantKills {
				t.Fatalf("errors = %q kills = %d, want %d errors and %d kills", got.Errors, got.Kills, test.wantErrors, test.wantKills)
			}
			for index, state := range got.Settled {
				if state != goja.PromiseStateFulfilled {
					t.Fatalf("call %d hook promise = %v, want fulfilled", index, state)
				}
			}
		})
	}
}

func TestGeneratedOpencodeLifecyclePluginEvictsBeyondCacheLimit(t *testing.T) {
	source := OpencodeLifecyclePlugin(opencodeTestBin)
	calls := make([]opencodeContextCall, 0, 514)
	for index := range 513 {
		calls = append(calls, opencodeContextCall{SessionID: "ses_" + strconv.Itoa(index), Agent: "build"})
	}
	calls = append(calls, opencodeContextCall{SessionID: "ses_0", Agent: "build"}, opencodeContextCall{SessionID: "ses_512", Agent: "build"})
	got, err := runOpencodePlugin(source, nil, func(int) opencodeSpawnResult {
		return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"catalog"}`}
	}, calls)
	if err != nil {
		t.Fatal(err)
	}
	// 513 distinct sessions spawn once each; the oldest was evicted and
	// spawns again, while the newest is still cached.
	if len(got.Spawns) != 514 {
		t.Fatalf("spawns = %d, want 514", len(got.Spawns))
	}
}

func TestGeneratedOpencodeLifecyclePluginProofRejectsRuntimeMutations(t *testing.T) {
	source := OpencodeLifecyclePlugin(opencodeTestBin)
	if err := verifyOpencodeLifecyclePlugin(source); err != nil {
		t.Fatalf("generated plugin failed baseline proof: %v", err)
	}
	for _, mutation := range []struct{ name, old, new string }{
		{name: "cache dropped", old: "catalogs.set(event.sessionID, catalog)", new: ""},
		{name: "hooks swapped", old: `"root":"session-start","child":"subagent-start"`, new: `"root":"subagent-start","child":"session-start"`},
		{name: "repo flag dropped", old: `"hook", subcommand, "--repo", repo, "--json"`, new: `"hook", subcommand, "--json"`},
		{name: "push await removed", old: "const text = await catalog", new: "const text = catalog"},
		{name: "timeout kill removed", old: "    proc.kill()\n", new: ""},
		{name: "failure no longer swallowed", old: "    return \"\"\n  }\n}", new: "    throw error\n  }\n}"},
		{name: "context hook renamed", old: `ctx.session.hook("context"`, new: `ctx.session.hook("prompt"`},
		{name: "parent check dropped", old: "if (info.parentID) {", new: "if (false) {"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(source, mutation.old, mutation.new, 1)
			if mutated == source {
				t.Fatalf("mutation target %q missing", mutation.old)
			}
			if err := verifyOpencodeLifecyclePlugin(mutated); err == nil {
				t.Fatal("runtime proof accepted mutated generated plugin")
			}
		})
	}
}

func verifyOpencodeLifecyclePlugin(source string) error {
	ok := func(int) opencodeSpawnResult {
		return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"catalog"}`}
	}
	root, err := runOpencodePlugin(source, map[string]opencodeSession{"ses_root": {}}, ok, []opencodeContextCall{{"ses_root", "build"}, {"ses_root", "build"}})
	if err != nil {
		return err
	}
	if root.HookName != "context" ||
		!reflect.DeepEqual(root.Spawns, []opencodeSpawnCall{{Argv: []string{opencodeTestBin, "hook", "session-start", "--repo", "/repo", "--json"}, Cwd: "/repo"}}) ||
		!reflect.DeepEqual(root.Pushes, [][]map[string]any{catalogPart("catalog"), catalogPart("catalog")}) {
		return fmt.Errorf("root session behavior drifted")
	}
	child, err := runOpencodePlugin(source, map[string]opencodeSession{"ses_child": {ParentID: "ses_root"}}, ok, []opencodeContextCall{{"ses_child", "general"}})
	if err != nil {
		return err
	}
	if len(child.Spawns) != 1 || child.Spawns[0].Argv[2] != "subagent-start" {
		return fmt.Errorf("child session behavior drifted")
	}
	slow, err := runOpencodePlugin(source, nil, func(int) opencodeSpawnResult {
		return opencodeSpawnResult{Stdout: `{"should_inject":true,"compact":"late"}`, TimeOut: true}
	}, []opencodeContextCall{{"ses_a", "build"}})
	if err != nil {
		return err
	}
	if slow.Kills != 1 || slow.Pushes[0] != nil {
		return fmt.Errorf("timeout behavior drifted")
	}
	failed, err := runOpencodePlugin(source, nil, func(int) opencodeSpawnResult { return opencodeSpawnResult{Throw: true} }, []opencodeContextCall{{"ses_a", "build"}})
	if err != nil {
		return err
	}
	if failed.Settled[0] != goja.PromiseStateFulfilled {
		return fmt.Errorf("hook failure escaped the context hook")
	}
	return nil
}
