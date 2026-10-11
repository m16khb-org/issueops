package hostprobe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestOmoAndOmpRunRejectInvalidConfigurationBeforeLaunching(t *testing.T) {
	type runnerCase struct {
		host string
		run  func(binary, ext string, deps Dependencies, request port.HostProbeRequest) port.HostProbeResult
		ext  func(string) string
		req  func() port.HostProbeRequest
	}
	runners := []runnerCase{
		{"omo", func(b, e string, d Dependencies, r port.HostProbeRequest) port.HostProbeResult {
			return newTestOmoRunner(b, e, d).Run(t.Context(), r)
		}, func(b string) string { a, _ := filepath.Abs(b); return omoTestLifecycleExtension(a) }, omoProbeRequest},
		{"omp", func(b, e string, d Dependencies, r port.HostProbeRequest) port.HostProbeResult {
			return newTestOmpRunner(b, e, d).Run(t.Context(), r)
		}, func(b string) string { a, _ := filepath.Abs(b); return ompTestLifecycleExtension(a) }, ompProbeRequest},
	}
	for _, rc := range runners {
		launched := false
		deps := func(mutate func(*Dependencies)) Dependencies {
			d := Dependencies{
				LookPath: func(string) (string, error) { return "/opt/bin/" + rc.host, nil },
				Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
					launched = true
					return CommandOutput{}, nil
				}},
				Getenv: func(string) string { return "" },
			}
			if mutate != nil {
				mutate(&d)
			}
			return d
		}
		cases := []struct {
			name   string
			binary string
			ext    string
			mutate func(*Dependencies)
			model  string
			tool   string
			code   string
		}{
			{name: "no harness binary", binary: "", ext: "x", code: "harness_binary_missing"},
			{name: "blank extension", binary: "issueops", ext: "  ", code: "lifecycle_extension_missing"},
			{name: "tampered extension", binary: "issueops", ext: "not the canonical extension", code: "mock_extension_invalid"},
			{name: "default model", binary: "issueops", ext: rc.ext("issueops"), model: "default", code: "explicit_model_required"},
			{name: "executable missing", binary: "issueops", ext: rc.ext("issueops"), mutate: func(d *Dependencies) {
				d.LookPath = func(string) (string, error) { return "", errors.New("nf") }
			}, code: "executable_not_found"},
			{name: "relative home", binary: "issueops", ext: rc.ext("issueops"), mutate: func(d *Dependencies) {
				d.Getenv = func(name string) string {
					if name == "HOME" {
						return "relative/home"
					}
					return ""
				}
			}, code: "auth_copy_failed"},
			{name: "temp dir failure", binary: "issueops", ext: rc.ext("issueops"), mutate: func(d *Dependencies) {
				d.TempDir = func(string, string) (string, error) { return "", errors.New("disk") }
			}, code: "episode_root_create_failed"},
		}
		for _, tc := range cases {
			t.Run(rc.host+"/"+tc.name, func(t *testing.T) {
				launched = false
				request := rc.req()
				if tc.model != "" {
					request.Model = tc.model
				}
				result := rc.run(tc.binary, tc.ext, deps(tc.mutate), request)
				if result.Completed || result.Cause != "harness_environment" || result.Code != tc.code || result.EvidenceSource != rc.host+"_runner" {
					t.Fatalf("result = %+v", result)
				}
				if launched {
					t.Fatal("host process was launched despite rejected configuration")
				}
			})
		}
	}
}

func TestOmoAndOmpPreflightReportEnvironmentFailures(t *testing.T) {
	type preflighter interface {
		Preflight(context.Context, port.HostProbeRequest) port.HostProbePreflight
	}
	build := func(host string, mutate func(*Dependencies)) preflighter {
		d := Dependencies{
			LookPath: func(string) (string, error) { return "/opt/bin/" + host, nil },
			Process:  fakeRunner{out: CommandOutput{Stdout: []byte(host + " 1.0\n")}},
			Getenv:   func(string) string { return "" },
		}
		if mutate != nil {
			mutate(&d)
		}
		if host == "omo" {
			return newTestOmoRunner("issueops", omoTestLifecycleExtension(), d)
		}
		return newTestOmpRunner("issueops", ompTestLifecycleExtension(), d)
	}
	for _, host := range []string{"omo", "omp"} {
		t.Run(host+" missing executable", func(t *testing.T) {
			got := build(host, func(d *Dependencies) { d.LookPath = func(string) (string, error) { return "", errors.New("nf") } }).
				Preflight(t.Context(), port.HostProbeRequest{Model: "p/m"})
			if got.Installed || got.Ready || got.Code != "executable_not_found" {
				t.Fatalf("got %+v", got)
			}
		})
		t.Run(host+" invalid auth home", func(t *testing.T) {
			got := build(host, func(d *Dependencies) {
				d.Getenv = func(name string) string {
					if name == "HOME" {
						return "rel"
					}
					return ""
				}
			}).Preflight(t.Context(), port.HostProbeRequest{Model: "p/m"})
			if !got.Installed || got.Ready || got.Code != "auth_copy_failed" {
				t.Fatalf("got %+v", got)
			}
		})
		t.Run(host+" temp dir failure", func(t *testing.T) {
			got := build(host, func(d *Dependencies) { d.TempDir = func(string, string) (string, error) { return "", errors.New("x") } }).
				Preflight(t.Context(), port.HostProbeRequest{Model: "p/m"})
			if got.Ready || got.Code != "episode_root_create_failed" {
				t.Fatalf("got %+v", got)
			}
		})
	}
	t.Run("omo nonzero exit", func(t *testing.T) {
		got := build("omo", func(d *Dependencies) { d.Process = fakeRunner{out: CommandOutput{ExitCode: 2}} }).
			Preflight(t.Context(), port.HostProbeRequest{Model: "p/m"})
		if got.Ready || got.Code != "version_probe_failed" {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestResolveOmoAuthRejectsRelativeLocationsAndLooseFiles(t *testing.T) {
	env := func(values map[string]string) Dependencies {
		return Dependencies{Getenv: func(name string) string { return values[name] }}
	}
	if _, err := resolveOmoAuth(env(map[string]string{"HOME": "rel"})); err == nil || err.Error() != "omo_home_invalid" {
		t.Fatalf("relative HOME err = %v", err)
	}
	if _, err := resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": "rel"})); err == nil || err.Error() != "omo_agent_dir_invalid" {
		t.Fatalf("relative agent dir err = %v", err)
	}
	snapshot, err := resolveOmoAuth(env(nil))
	if err != nil || snapshot.present {
		t.Fatalf("no HOME: %+v %v", snapshot, err)
	}
	dir := t.TempDir()
	snapshot, err = resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": dir}))
	if err != nil || snapshot.present {
		t.Fatalf("absent auth file: %+v %v", snapshot, err)
	}
	auth := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(auth, []byte(`{"k":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": dir}))
	if err != nil || !snapshot.present || string(snapshot.data) != `{"k":1}` {
		t.Fatalf("private auth: %+v %v", snapshot, err)
	}
	if err := os.Chmod(auth, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": dir})); err == nil || err.Error() != "omo_auth_invalid" {
		t.Fatalf("world-readable auth err = %v", err)
	}
	if err := os.Chmod(auth, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auth, []byte(strings.Repeat("x", MaxOutputBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": dir})); err == nil || err.Error() != "omo_auth_invalid" {
		t.Fatalf("oversize auth err = %v", err)
	}
	if err := os.Remove(auth); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(auth, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOmoAuth(env(map[string]string{"OMO_CODING_AGENT_DIR": dir})); err == nil || err.Error() != "omo_auth_invalid" {
		t.Fatalf("directory auth err = %v", err)
	}
}

func TestOmoStreamFailureMapsKnownCodes(t *testing.T) {
	cases := map[string][2]string{
		"host_stream_no_call":         {"unknown", "no_call"},
		"host_stream_invalid":         {"transport", "host_stream_invalid"},
		"host_stream_multiple_calls":  {"transport", "host_stream_multiple_calls"},
		"host_stream_non_target_call": {"transport", "host_stream_non_target_call"},
		"host_stream_tool_error":      {"transport", "host_stream_tool_error"},
		"host_stream_model_missing":   {"transport", "host_stream_model_missing"},
		"unexpected parser explosion": {"transport", "host_stream_invalid"},
	}
	for in, want := range cases {
		cause, code := omoStreamFailure(errors.New(in))
		if cause != want[0] || code != want[1] {
			t.Errorf("%s -> %q/%q, want %v", in, cause, code, want)
		}
	}
}

func TestResolveCodexSourceHome(t *testing.T) {
	deps := func(values map[string]string) Dependencies {
		return Dependencies{Getenv: func(name string) string { return values[name] }}
	}
	if got, err := resolveCodexSourceHome(deps(map[string]string{"CODEX_HOME": " /x/codex ", "HOME": "/h"})); err != nil || got != "/x/codex" {
		t.Fatalf("CODEX_HOME = %q, %v", got, err)
	}
	if got, err := resolveCodexSourceHome(deps(map[string]string{"HOME": "/h"})); err != nil || got != "/h/.codex" {
		t.Fatalf("HOME fallback = %q, %v", got, err)
	}
	for name, values := range map[string]map[string]string{
		"relative CODEX_HOME": {"CODEX_HOME": "rel"},
		"no home":             {},
		"relative HOME":       {"HOME": "rel"},
	} {
		if _, err := resolveCodexSourceHome(deps(values)); err == nil || err.Error() != "codex_home_invalid" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPrepareCodexHomesRejectRelativePaths(t *testing.T) {
	deps := normalizeDependencies(Dependencies{Getenv: func(string) string { return "/h" }})
	if _, err := prepareCodexSmokeHome("rel", "/bin/x", "/obs", deps); err == nil || err.Error() != "codex_smoke_path_invalid" {
		t.Fatalf("smoke err = %v", err)
	}
	if _, err := prepareCodexLiveHome("/root", "bin", "/obs", deps); err == nil || err.Error() != "codex_live_path_invalid" {
		t.Fatalf("live err = %v", err)
	}
	if _, err := prepareCodexLiveHome("/root", "/bin/x", "obs", deps); err == nil || err.Error() != "codex_live_path_invalid" {
		t.Fatalf("live obs err = %v", err)
	}
}

func TestPrepareCodexLiveHomeWritesQuotedSessionStartHookAndLinksAuth(t *testing.T) {
	source := t.TempDir()
	authPath := filepath.Join(source, "auth.json")
	if err := os.WriteFile(authPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	deps := normalizeDependencies(Dependencies{Getenv: func(name string) string {
		if name == "CODEX_HOME" {
			return source
		}
		return ""
	}})
	home, err := prepareCodexLiveHome(root, "/opt/it's/issueops", "/tmp/obs.json", deps)
	if err != nil {
		t.Fatal(err)
	}
	if home != filepath.Join(root, "codex-home") {
		t.Fatalf("home = %q", home)
	}
	link, err := os.Readlink(filepath.Join(home, "auth.json"))
	if err != nil || link != authPath {
		t.Fatalf("auth link = %q, %v", link, err)
	}
	data, err := os.ReadFile(filepath.Join(home, "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc codexSmokeHookDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	hooks := doc.Hooks["SessionStart"]
	if len(doc.Hooks) != 1 || len(hooks) != 1 || len(hooks[0].Hooks) != 1 {
		t.Fatalf("doc = %s", data)
	}
	hook := hooks[0].Hooks[0]
	want := "/usr/bin/env ISSUEOPS_CHILD_SMOKE_HOOKS=1 ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE='/tmp/obs.json' '/opt/it'\"'\"'s/issueops' hook session-start --host codex"
	if hook.Type != "command" || hook.Timeout != 5 || hook.Command != want {
		t.Fatalf("hook = %+v\nwant command %s", hook, want)
	}
	info, _ := os.Stat(filepath.Join(home, "hooks.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("hooks.json mode = %v", info.Mode().Perm())
	}
}

func TestLinkCodexSmokeAuthPolicy(t *testing.T) {
	runtimeHome := t.TempDir()
	source := t.TempDir()
	if err := linkCodexSmokeAuth(runtimeHome, source); err != nil {
		t.Fatalf("absent auth should be allowed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(runtimeHome, "auth.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("link created without source: %v", err)
	}
	auth := filepath.Join(source, "auth.json")
	if err := os.WriteFile(auth, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(auth, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := linkCodexSmokeAuth(runtimeHome, source); err == nil || err.Error() != "codex_auth_invalid" {
		t.Fatalf("loose auth err = %v", err)
	}
	if err := os.Remove(auth); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(source, "real.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, auth); err != nil {
		t.Fatal(err)
	}
	if err := linkCodexSmokeAuth(runtimeHome, source); err == nil || err.Error() != "codex_auth_invalid" {
		t.Fatalf("symlinked auth err = %v", err)
	}
}

func TestWriteCodexProbeHomeRefusesExistingRuntimeHome(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "codex-home"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCodexProbeHome(root, t.TempDir(), codexSmokeHookDocument{}); err == nil {
		t.Fatal("pre-existing codex-home must not be reused")
	}
}

func TestProjectActivatedCodexSmokeHooksContract(t *testing.T) {
	const bin = "/opt/issueops"
	managed := shellSingleQuote(bin) + " hook session-start --host codex"
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	hookJSON := func(command string, extra string) string {
		return `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":` + jsonString(command) + `,"timeout":5` + extra + `}]}]}}`
	}
	doc, err := projectActivatedCodexSmokeHooks(write(t, hookJSON(managed, "")), bin, "/tmp/o's")
	if err != nil {
		t.Fatal(err)
	}
	got := doc.Hooks["SessionStart"][0].Hooks[0].Command
	if want := "/usr/bin/env ISSUEOPS_CHILD_SMOKE_HOOKS=1 ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE='/tmp/o'\"'\"'s' " + managed; got != want {
		t.Fatalf("command = %q", got)
	}
	// An unrelated co-resident hook is dropped, not copied.
	mixed := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"touch /x","timeout":1},{"type":"command","command":` + jsonString(managed) + `,"timeout":5}]}]}}`
	doc, err = projectActivatedCodexSmokeHooks(write(t, mixed), bin, "/o")
	if err != nil || len(doc.Hooks["SessionStart"][0].Hooks) != 1 {
		t.Fatalf("mixed: %+v %v", doc, err)
	}
	rejected := map[string]struct{ body, code string }{
		"extra hook field":  {hookJSON(managed, `,"statusMessage":"x"`), "codex_smoke_hook_structure_invalid"},
		"wrong timeout":     {strings.Replace(hookJSON(managed, ""), `"timeout":5`, `"timeout":9`, 1), "codex_smoke_hook_contract_invalid"},
		"wrong type":        {strings.Replace(hookJSON(managed, ""), `"type":"command"`, `"type":"prompt"`, 1), "codex_smoke_hook_contract_invalid"},
		"no managed hook":   {`{"hooks":{"SessionStart":[]}}`, "codex_smoke_hook_cardinality_invalid"},
		"duplicate managed": {`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":` + jsonString(managed) + `,"timeout":5},{"type":"command","command":` + jsonString(managed) + `,"timeout":5}]}]}}`, "codex_smoke_hook_cardinality_invalid"},
	}
	for name, tc := range rejected {
		if _, err := projectActivatedCodexSmokeHooks(write(t, tc.body), bin, "/o"); err == nil || err.Error() != tc.code {
			t.Errorf("%s: err = %v, want %s", name, err, tc.code)
		}
	}
	if _, err := projectActivatedCodexSmokeHooks(write(t, "{"), bin, "/o"); err == nil {
		t.Error("malformed hooks.json accepted")
	}
	if _, err := projectActivatedCodexSmokeHooks(filepath.Join(t.TempDir(), "absent.json"), bin, "/o"); err == nil {
		t.Error("missing hooks.json accepted")
	}
}

func TestReplaceEnvironmentValueDedupesAndSorts(t *testing.T) {
	got := replaceEnvironmentValue([]string{"Z=1", "CODEX_HOME=old", "A=2", "CODEX_HOME=dup"}, "CODEX_HOME", "new")
	if strings.Join(got, "|") != "A=2|CODEX_HOME=new|Z=1" {
		t.Fatalf("got %v", got)
	}
	got = replaceEnvironmentValue([]string{"B=1"}, "A", "x")
	if strings.Join(got, "|") != "A=x|B=1" {
		t.Fatalf("append: %v", got)
	}
}

func TestObservedMCPResultOnlyAcceptsCompletedProbeServerCalls(t *testing.T) {
	decode := func(raw string) any {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	accepted := map[string]string{
		"codex item":     `{"type":"item.completed","item":{"type":"mcp_tool_call","server":"issueops_probe","status":"completed","result":{"content":"x"}}}`,
		"pi tool_result": `{"type":"tool_result","tool":{"name":"mcp_issueops_probe_x"},"result":{"content":"x"}}`,
	}
	for name, raw := range accepted {
		if r, ok := observedMCPResult(decode(raw)); !ok || r == nil {
			t.Errorf("%s rejected", name)
		}
	}
	rejected := map[string]string{
		"not an object":    `[1]`,
		"other server":     `{"type":"item.completed","item":{"type":"mcp_tool_call","server":"other","status":"completed","result":{}}}`,
		"not completed":    `{"type":"item.completed","item":{"type":"mcp_tool_call","server":"issueops_probe","status":"failed","result":{}}}`,
		"not an mcp call":  `{"type":"item.completed","item":{"type":"command_execution","server":"issueops_probe","status":"completed","result":{}}}`,
		"missing result":   `{"type":"item.completed","item":{"type":"mcp_tool_call","server":"issueops_probe","status":"completed"}}`,
		"null result":      `{"type":"item.completed","item":{"type":"mcp_tool_call","server":"issueops_probe","status":"completed","result":null}}`,
		"other tool":       `{"type":"tool_result","tool":{"name":"bash"},"result":{}}`,
		"tool null result": `{"type":"tool_result","tool":{"name":"issueops_probe"},"result":null}`,
		"unrelated event":  `{"type":"turn.completed"}`,
	}
	for name, raw := range rejected {
		if _, ok := observedMCPResult(decode(raw)); ok {
			t.Errorf("%s accepted", name)
		}
	}
	if _, ok := observedMCPResult("str"); ok {
		t.Error("non-object accepted")
	}
}

func TestSemanticResponseProjectionRules(t *testing.T) {
	decode := func(raw string) any {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	same := func(a, b string) bool {
		da, ea := semanticResponseDigest([]any{decode(a)})
		db, eb := semanticResponseDigest([]any{decode(b)})
		if ea != nil || eb != nil {
			t.Fatalf("unexpected errors %v %v", ea, eb)
		}
		return da == db
	}
	if !same(`{"content":"hi"}`, `{"content":[{"type":"text","text":"hi"}]}`) {
		t.Error("string content and text block must be equivalent")
	}
	if !same(`{"content":[{"text":"hi"}]}`, `{"content":"hi","isError":false}`) {
		t.Error("untyped text block with explicit false error flag must be equivalent")
	}
	if !same(`{"content":"hi","structuredContent":{"a":1}}`, `{"content":"hi","structured_content":{"a":1}}`) {
		t.Error("structured content spellings must be equivalent")
	}
	if same(`{"content":"hi","structuredContent":{"a":1}}`, `{"content":"hi","structuredContent":{"a":2}}`) {
		t.Error("different structured content must differ")
	}
	if same(`{"content":"hi"}`, `{"content":"bye"}`) {
		t.Error("different text must differ")
	}
	invalid := map[string]string{
		"not object":         `"x"`,
		"no content":         `{"isError":false}`,
		"numeric content":    `{"content":5}`,
		"non-object block":   `{"content":["x"]}`,
		"image block":        `{"content":[{"type":"image","data":"x"}]}`,
		"block without text": `{"content":[{"type":"text"}]}`,
		"isError not bool":   `{"content":"x","isError":"no"}`,
		"is_error not bool":  `{"content":"x","is_error":1}`,
	}
	for name, raw := range invalid {
		if _, err := semanticResponseDigest([]any{decode(raw)}); err == nil || err.Error() != "semantic_response_invalid" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := semanticResponseDigest(nil); err == nil || err.Error() != "semantic_response_missing" {
		t.Errorf("empty err = %v", err)
	}
}

func TestDecodeHookMarkerEventsValidation(t *testing.T) {
	got, err := decodeHookMarkerEvents([]byte(`{"event":"SessionStart","model":"m1"}` + "\n" + `{"event":"PreToolUse"}` + "\n" + `{"event":"SessionStart","model":""}`))
	if err != nil || !got.SessionStartObserved || !got.PreToolUseObserved || got.Model != "m1" {
		t.Fatalf("got %+v, %v", got, err)
	}
	for name, raw := range map[string]string{
		"empty":              ``,
		"unknown event":      `{"event":"Stop"}`,
		"unknown field":      `{"event":"SessionStart","extra":1}`,
		"malformed":          `{"event":`,
		"conflicting models": `{"event":"SessionStart","model":"a"}` + "\n" + `{"event":"SessionStart","model":"b"}`,
		"padded model":       `{"event":"SessionStart","model":" padded "}`,
		"over-long model":    `{"event":"SessionStart","model":"` + strings.Repeat("m", 300) + `"}`,
	} {
		if _, err := decodeHookMarkerEvents([]byte(raw)); err == nil || err.Error() != "hook_observation_invalid" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestObserveRecordedHookEventsFileRules(t *testing.T) {
	if _, err := observeRecordedHookEvents("relative/obs"); err == nil || err.Error() != "hook_observation_invalid" {
		t.Fatalf("relative err = %v", err)
	}
	dir := t.TempDir()
	obs := filepath.Join(dir, "obs.json")
	got, err := observeRecordedHookEvents(obs)
	if err != nil || got != (hostStreamObservation{}) {
		t.Fatalf("absent marker: %+v %v", got, err)
	}
	marker := obs + ".hooks"
	if err := os.WriteFile(marker, []byte(`{"event":"SessionStart","model":"m"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(marker, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := observeRecordedHookEvents(obs); err == nil || err.Error() != "hook_observation_invalid" {
		t.Fatalf("loose-mode marker err = %v", err)
	}
	if err := os.Chmod(marker, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = observeRecordedHookEvents(obs)
	if err != nil || !got.SessionStartObserved || got.Model != "m" {
		t.Fatalf("valid marker: %+v %v", got, err)
	}
	if _, statErr := os.Lstat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("marker must be consumed, stat err = %v", statErr)
	}
	if err := os.WriteFile(marker, []byte(`{"event":"Bogus"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := observeRecordedHookEvents(obs); err == nil || err.Error() != "hook_observation_invalid" {
		t.Fatalf("bogus marker err = %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := observeRecordedHookEvents(obs); err == nil || err.Error() != "hook_observation_invalid" {
		t.Fatalf("directory marker err = %v", err)
	}
}

func TestMergeHookObservationModelConflict(t *testing.T) {
	stream := hostStreamObservation{Model: "a", SessionStartObserved: false}
	if err := mergeHookObservation(&stream, hostStreamObservation{Model: "b", SessionStartObserved: true}); err == nil || err.Error() != "host_model_mismatch" {
		t.Fatalf("err = %v", err)
	}
	if stream.SessionStartObserved {
		t.Fatal("conflicting merge must not apply hook flags")
	}
	empty := hostStreamObservation{}
	if err := mergeHookObservation(&empty, hostStreamObservation{Model: "b", PreToolUseObserved: true}); err != nil || empty.Model != "b" || !empty.PreToolUseObserved {
		t.Fatalf("adopt: %+v %v", empty, err)
	}
}

func TestValidHostRuntimeObservationRequiresEveryEvidenceField(t *testing.T) {
	good := hostStreamObservation{SessionStartObserved: true, Model: "m", AmbientToolCount: 1, MCPCallCount: 1, ResponseSHA256: strings.Repeat("a", 64)}
	if !validHostRuntimeObservation(good) {
		t.Fatal("complete observation rejected")
	}
	breakers := map[string]func(*hostStreamObservation){
		"no session start": func(o *hostStreamObservation) { o.SessionStartObserved = false },
		"pre tool use":     func(o *hostStreamObservation) { o.PreToolUseObserved = true },
		"no model":         func(o *hostStreamObservation) { o.Model = "" },
		"ambient tools":    func(o *hostStreamObservation) { o.AmbientToolCount = 2 },
		"two mcp calls":    func(o *hostStreamObservation) { o.MCPCallCount = 2 },
		"bad digest":       func(o *hostStreamObservation) { o.ResponseSHA256 = "xyz" },
	}
	for name, mutate := range breakers {
		o := good
		mutate(&o)
		if validHostRuntimeObservation(o) {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestObserveHostStreamRejectsStructuralViolations(t *testing.T) {
	for name, stream := range map[string]string{
		"empty":                    "",
		"oversize":                 strings.Repeat(" ", MaxOutputBytes+1),
		"whitespace only":          "   \n",
		"duplicate claude tool id": `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t","name":"x"},{"type":"tool_use","id":"t","name":"y"}]}}`,
		"tool_use without id":      `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"x"}]}}`,
		"item without type":        `{"type":"item.completed","item":{}}`,
	} {
		if _, err := observeHostStream([]byte(stream)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	got, err := observeHostStream([]byte(`{"type":"hook.completed","hook":{"event_name":"pre_tool_use"}}`))
	if err != nil || !got.PreToolUseObserved || got.ResponseSHA256 != "" || got.MCPCallCount != 0 {
		t.Fatalf("pretooluse: %+v %v", got, err)
	}
}

func TestWritePrivateFileCreatesPrivateParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "f")
	if err := writePrivateFile(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Dir(filepath.Dir(path)), filepath.Dir(path)} {
		if info, _ := os.Stat(p); info.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v", p, info.Mode().Perm())
		}
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v", info.Mode().Perm())
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(filepath.Join(blocker, "child"), []byte("x")); err == nil {
		t.Fatal("writing beneath a regular file succeeded")
	}
}
