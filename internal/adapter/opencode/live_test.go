package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/domain/nativehost"
	"issueops/internal/port"
)

var liveFixtureOut = flag.String("fixture-out", "", "write the live run's JSON stream to this path")

// TestOpencodeLiveStandaloneSession runs the installed opencode against a
// mock OpenAI-compatible provider with issueops installed into a private HOME.
// It needs ISSUEOPS_OPENCODE_LIVE=1 and an opencode 2.x binary on PATH.
func TestOpencodeLiveStandaloneSession(t *testing.T) {
	if os.Getenv("ISSUEOPS_OPENCODE_LIVE") != "1" {
		t.Skip("set ISSUEOPS_OPENCODE_LIVE=1 to run against the installed opencode")
	}
	opencodeBin, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("opencode is not installed")
	}
	base := t.TempDir()
	home, xdg, state := filepath.Join(base, "home"), filepath.Join(base, "xdg"), filepath.Join(base, "state")
	repo, harnessRoot := filepath.Join(base, "repo"), filepath.Join(base, "harness")
	for _, dir := range []string{home, filepath.Join(xdg, "config"), filepath.Join(xdg, "data"), filepath.Join(xdg, "state"), filepath.Join(xdg, "cache"), state, filepath.Join(harnessRoot, "skills")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(base, "bin", "issueops")
	build := exec.Command("go", "build", "-o", binary, "./cmd/issueops")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build issueops: %v\n%s", err, output)
	}
	hookLog := filepath.Join(base, "hook.log")
	counter := filepath.Join(base, "bin", "issueops-counter")
	counterScript := fmt.Sprintf("#!/bin/sh\nif [ \"${1:-}\" = hook ]; then printf 'hook\\n' >> '%s'; fi\nexec '%s' \"$@\"\n", hookLog, binary)
	if err := os.WriteFile(counter, []byte(counterScript), 0o700); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(repo, ".issueops", "CONSTITUTION.md"), "# Live constitution\n\nThe opencode live test reads this catalog.\n", 0o644)
	if output, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	const skill = "atomic-commit-push"
	if err := os.Symlink(filepath.Join(moduleRoot, "skills", skill), filepath.Join(harnessRoot, "skills", skill)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(moduleRoot, "skills", skill), filepath.Join(home, ".claude", "skills", skill)); err != nil {
		t.Fatal(err)
	}
	xdgConfig := filepath.Join(xdg, "config")
	installer := NewInstaller(testDependencies(), hostprotocol.OpencodeLifecyclePlugin, func(home string) string {
		return nativehost.OpencodeConfigRoot(home, xdgConfig)
	})
	install, err := installer.Install(port.NativeInstallRequest{Root: harnessRoot, Home: home, BinPath: counter, MCPTransport: "stdio", SkillNames: []string{skill}})
	if err != nil || !install.OK {
		t.Fatalf("install = %+v, %v", install, err)
	}

	provider := newScriptedProvider(t, binary, repo)
	server := httptest.NewServer(provider)
	defer server.Close()
	configContent, _ := json.Marshal(map[string]any{
		"model": "mock/m",
		"providers": map[string]any{"mock": map[string]any{
			"package":  "@opencode/ai/providers/openai-compatible",
			"settings": map[string]any{"baseURL": server.URL + "/v1", "apiKey": "mock"},
			"models":   map[string]any{"m": map[string]any{"name": "m"}},
		}},
	})

	env := []string{}
	// The same fixed ambient list as hostprobe isolatedHostEnv, plus TERM.
	for _, name := range []string{"PATH", "USER", "TMPDIR", "LANG", "LANGUAGE", "LC_ALL", "LC_ADDRESS", "LC_COLLATE", "LC_CTYPE", "LC_IDENTIFICATION", "LC_MEASUREMENT", "LC_MESSAGES", "LC_MONETARY", "LC_NAME", "LC_NUMERIC", "LC_PAPER", "LC_TELEPHONE", "LC_TIME", "TERM"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	env = append(env,
		"HOME="+home, "XDG_CONFIG_HOME="+xdgConfig, "XDG_DATA_HOME="+filepath.Join(xdg, "data"),
		"XDG_STATE_HOME="+filepath.Join(xdg, "state"), "XDG_CACHE_HOME="+filepath.Join(xdg, "cache"),
		"ISSUEOPS_STATE_DIR="+state, "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_MODELS_FETCH=1",
		"OPENCODE_CONFIG_CONTENT="+string(configContent), "PWD="+repo,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, opencodeBin, "run", "--standalone", "--format", "json")
	run.Dir, run.Env, run.Stdin = repo, env, strings.NewReader("Run the scripted issueops checks.")
	var stdout, stderr bytes.Buffer
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("opencode run: %v\nstderr:\n%s\nstdout:\n%s", err, stderr.String(), stdout.String())
	}
	if *liveFixtureOut != "" {
		if err := os.WriteFile(*liveFixtureOut, stdout.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	provider.assert(t, hookLog)
}

// scriptedProvider answers chat completions in a fixed order and keeps every
// agent-loop request (one that carries tools; title requests carry none).
type scriptedProvider struct {
	t        *testing.T
	binary   string
	repo     string
	mu       sync.Mutex
	requests []map[string]any
	sleeps   int
	mcpStep  int
	authStep int
}

func newScriptedProvider(t *testing.T, binary, repo string) *scriptedProvider {
	return &scriptedProvider{t: t, binary: binary, repo: repo, mcpStep: -1}
}

func (p *scriptedProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request map[string]any
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	if _, agentLoop := request["tools"]; !agentLoop {
		writeCompletion(w, "", nil, "title")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	step := len(p.requests)
	p.requests = append(p.requests, request)
	switch {
	case step == 0:
		writeCompletion(w, "shell", map[string]any{"command": "sleep 1; '" + p.binary + "' execution whoami --json", "description": "whoami"}, "")
	case step == 1:
		var whoami struct {
			ClaimActorFlags []string `json:"claim_actor_flags"`
		}
		if err := decodeToolJSON(lastToolResult(request), &whoami); err != nil || len(whoami.ClaimActorFlags) == 0 {
			writeCompletion(w, "", nil, "whoami failed")
			return
		}
		p.authStep = step + 1
		command := "'" + p.binary + "' mcp authorize --workspace-root '" + p.repo + "' " + whoami.ClaimActorFlags[0] + " --json"
		writeCompletion(w, "shell", map[string]any{"command": command, "description": "authorize"}, "")
	case p.mcpStep < 0 && slices.Contains(toolNames(request), "issueops_docs_index"):
		p.mcpStep = step + 1
		writeCompletion(w, "issueops_docs_index", map[string]any{}, "")
	case p.mcpStep < 0 && p.sleeps < 3:
		p.sleeps++
		writeCompletion(w, "shell", map[string]any{"command": "sleep 1", "description": "wait for MCP"}, "")
	default:
		writeCompletion(w, "", nil, "done")
	}
}

func (p *scriptedProvider) assert(t *testing.T, hookLog string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mcpStep < 0 || len(p.requests) <= p.mcpStep {
		t.Fatalf("the MCP tools never reached a request: %d agent-loop requests", len(p.requests))
	}
	for index, request := range p.requests {
		if !strings.Contains(systemText(request), ".issueops/CONSTITUTION.md") {
			t.Fatalf("(a) agent-loop request %d has no issueops catalog", index)
		}
	}
	if tools := toolNames(p.requests[p.mcpStep-1]); !slices.Contains(tools, "issueops_harness_inspect") || !slices.Contains(tools, "issueops_docs_index") {
		t.Fatalf("(b) MCP request tools = %q", tools)
	}
	if count := skillEntries(p.requests[0], "atomic-commit-push"); count != 1 {
		t.Fatalf("(c) atomic-commit-push listed %d times in the first request", count)
	}
	var whoami struct {
		Host            string `json:"host"`
		SessionIDSource string `json:"session_id_source"`
		Ancestry        []struct {
			Executable string `json:"executable"`
		} `json:"ancestry"`
	}
	if err := decodeToolJSON(lastToolResult(p.requests[1]), &whoami); err != nil || whoami.Host != "opencode" || whoami.SessionIDSource != "OPENCODE_SESSION_ID" ||
		len(whoami.Ancestry) == 0 || filepath.Base(whoami.Ancestry[0].Executable) != "opencode" {
		t.Fatalf("(d) whoami = %+v err=%v\n%s", whoami, err, lastToolResult(p.requests[1]))
	}
	var authorize struct {
		OK            bool   `json:"ok"`
		AuthorityFile string `json:"authority_file"`
	}
	if err := decodeToolJSON(lastToolResult(p.requests[p.authStep]), &authorize); err != nil || !authorize.OK || authorize.AuthorityFile == "" {
		t.Fatalf("(e) authorize = %+v err=%v\n%s", authorize, err, lastToolResult(p.requests[p.authStep]))
	}
	var docs struct {
		Docs []any `json:"docs"`
	}
	if err := decodeToolJSON(lastToolResult(p.requests[p.mcpStep]), &docs); err != nil || docs.Docs == nil {
		t.Fatalf("(f) docs_index = %v\n%s", err, lastToolResult(p.requests[p.mcpStep]))
	}
	log, err := os.ReadFile(hookLog)
	if err != nil || strings.Count(string(log), "hook\n") != 1 {
		t.Fatalf("(g) hook runs = %q, %v (want exactly one for %d model calls)", log, err, len(p.requests))
	}
}

func writeCompletion(w http.ResponseWriter, tool string, arguments map[string]any, text string) {
	usage := map[string]any{"prompt_tokens": 1234, "completion_tokens": 56, "total_tokens": 1290,
		"prompt_tokens_details": map[string]any{"cached_tokens": 100}, "completion_tokens_details": map[string]any{"reasoning_tokens": 7}}
	chunk := func(delta map[string]any, finish any, withUsage bool) {
		body := map[string]any{"id": "c", "object": "chat.completion.chunk", "created": 0, "model": "m",
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
		if withUsage {
			body["usage"] = usage
		}
		encoded, _ := json.Marshal(body)
		fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	if tool != "" {
		encoded, _ := json.Marshal(arguments)
		chunk(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "call_" + tool, "type": "function",
			"function": map[string]any{"name": tool, "arguments": string(encoded)}}}}, nil, false)
		chunk(map[string]any{}, "tool_calls", true)
	} else {
		chunk(map[string]any{"role": "assistant", "content": text}, nil, false)
		chunk(map[string]any{}, "stop", true)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func toolNames(request map[string]any) []string {
	tools, _ := request["tools"].([]any)
	names := []string{}
	for _, tool := range tools {
		function, _ := tool.(map[string]any)["function"].(map[string]any)
		if name, ok := function["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func messages(request map[string]any) []map[string]any {
	raw, _ := request["messages"].([]any)
	out := []map[string]any{}
	for _, message := range raw {
		if object, ok := message.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

func contentText(message map[string]any) string {
	switch content := message["content"].(type) {
	case string:
		return content
	case []any:
		var text strings.Builder
		for _, part := range content {
			if value, ok := part.(map[string]any)["text"].(string); ok {
				text.WriteString(value)
			}
		}
		return text.String()
	}
	return ""
}

func systemText(request map[string]any) string {
	var text strings.Builder
	for _, message := range messages(request) {
		if message["role"] == "system" {
			text.WriteString(contentText(message))
		}
	}
	return text.String()
}

func lastToolResult(request map[string]any) string {
	all := messages(request)
	for index := len(all) - 1; index >= 0; index-- {
		if all[index]["role"] == "tool" {
			return contentText(all[index])
		}
	}
	return ""
}

// decodeToolJSON reads the JSON object a tool printed; the shell tool may add
// framing text around the command output.
func decodeToolJSON(output string, value any) error {
	start, end := strings.Index(output, "{"), strings.LastIndex(output, "}")
	if start < 0 || end < start {
		return fmt.Errorf("no JSON object in tool output")
	}
	return json.Unmarshal([]byte(output[start:end+1]), value)
}

// skillEntries counts the skill's <available_skills> entries in the system
// prompt. Paths to the skill's files may repeat its name; entries do not.
func skillEntries(request map[string]any, skill string) int {
	return strings.Count(systemText(request), "<name>"+skill+"</name>")
}
