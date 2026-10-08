package hookcli

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func hookTempRepoWithDoc(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".issueops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", "ARCHITECTURE.md"), []byte("# Arch\n\n## 경계\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// runHookRawCapture feeds stdinJSON to fn and returns the raw stdout so tests
// can assert on "nothing printed" as well as on JSON payloads.
func runHookRawCapture(t *testing.T, stdinJSON string, fn func() error) string {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	return testsupport.CaptureStdout(t, func() error {
		if err := fn(); err != nil {
			t.Fatalf("hook: %v", err)
		}
		return nil
	})
}

func runHookCapture(t *testing.T, stdinJSON string, fn func() error) map[string]any {
	t.Helper()
	out := runHookRawCapture(t, stdinJSON, fn)
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	return obj
}

func hookAdditionalContext(obj map[string]any) string {
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return ""
	}
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func TestRunHookSessionStartInjectsCatalogClaude(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
	if hso, _ := obj["hookSpecificOutput"].(map[string]any); hso["hookEventName"] != "SessionStart" {
		t.Fatalf("SessionStart must name its event: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "Project docs under .issueops/") || !strings.Contains(ctx, "- .issueops/ARCHITECTURE.md: ") {
		t.Fatalf("SessionStart must inject the compact catalog: %q", ctx)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.HasPrefix(sysMsg, "📚 project docs 1개") || strings.Contains(sysMsg, "\n") {
		t.Fatalf("Claude SessionStart should show a one-line catalog notice via systemMessage: %v", obj["systemMessage"])
	}
}

func TestRunHookSessionStartRecordsBoundedLiveProbeEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(root, "observation.json")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", observationPath)
	repo := hookTempRepoWithDoc(t)
	runHookCapture(t, `{"cwd":"`+repo+`","hook_event_name":"SessionStart","model":"gpt-6-sol","permission_mode":"never","session_id":"session"}`, func() error {
		return runHook([]string{"session-start", "--host", "codex"})
	})

	data, err := os.ReadFile(observationPath + ".hooks")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["event"] != "SessionStart" || got["model"] != "gpt-6-sol" {
		t.Fatalf("probe evidence = %#v", got)
	}
	info, err := os.Stat(observationPath + ".hooks")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("probe evidence mode = %v err=%v", info, err)
	}
}

// Claude Code and Codex both re-run SessionStart with source "compact" after
// compaction, and neither host accepts model-facing context on PostCompact,
// so the compact source must inject exactly like startup.
func TestRunHookSessionStartInjectsCatalogOnCompactSource(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, source := range []string{"compact", "resume", "clear"} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"`+source+`"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
		if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "Project docs under .issueops/") {
			t.Fatalf("source %s must re-establish the catalog: %+v", source, obj)
		}
	}
}

func TestRunHookSessionStartCodexGetsTheClaudeModelTextWithoutSystemMessage(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	codex := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "codex"}) })
	claude := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
	if _, ok := codex["systemMessage"]; ok {
		t.Fatalf("Codex SessionStart must omit systemMessage: %+v", codex)
	}
	if ctx := hookAdditionalContext(codex); ctx == "" || ctx != hookAdditionalContext(claude) {
		t.Fatalf("Codex and Claude must receive the same model text:\ncodex=%q\nclaude=%q", ctx, hookAdditionalContext(claude))
	}
}

func TestRunHookContextEventsEmitNoopWithoutProjectDocs(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, host := range []string{"codex", "claude"} {
		for _, event := range []string{"session-start", "subagent-start", "post-compact"} {
			obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{event, "--host", host}) })
			if len(obj) != 0 {
				t.Fatalf("%s/%s without docs must be an empty object, got %+v", host, event, obj)
			}
		}
	}
}

func TestRunHookPostCompactCarriesOnlyUserFacingCatalog(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, host := range []string{"codex", "claude", ""} {
		args := []string{"post-compact"}
		if host != "" {
			args = append(args, "--host", host)
		}
		obj := runHookCapture(t, `{"cwd":"`+repo+`","trigger":"auto"}`, func() error { return runHook(args) })
		if _, ok := obj["hookSpecificOutput"]; ok {
			t.Fatalf("PostCompact must not emit hookSpecificOutput (%q): %+v", host, obj)
		}
		if sysMsg, _ := obj["systemMessage"].(string); !strings.HasPrefix(sysMsg, "📚 project docs 1개") {
			t.Fatalf("PostCompact (%q) should carry the catalog notice via systemMessage: %+v", host, obj)
		}
	}
}

func TestRunHookJSONOutputUsesSnakeCaseFields(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, event := range []string{"session-start", "post-compact"} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHook([]string{event, "--json"}) })
		if obj["should_inject"] != true {
			t.Fatalf("%s --json must expose should_inject: %+v", event, obj)
		}
		if compact, _ := obj["compact"].(string); !strings.Contains(compact, "- .issueops/ARCHITECTURE.md: ") {
			t.Fatalf("%s --json must expose the compact catalog: %+v", event, obj)
		}
		for _, field := range []string{"ShouldInject", "Compact", "UserView", "ProjectDocs"} {
			if _, ok := obj[field]; ok {
				t.Fatalf("%s --json must not expose PascalCase field %s: %+v", event, field, obj)
			}
		}
	}
}

func TestRunHookContextEventsAcrossIsolatedWorktreesDoNotCreateHarnessState(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	for _, host := range []string{"codex", "claude"} {
		for _, repo := range []string{hookTempRepoWithDoc(t), hookTempRepoWithDoc(t)} {
			for _, event := range []string{"session-start", "subagent-start", "post-compact"} {
				event := event
				runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error {
					return runHook([]string{event, "--host", host})
				})
			}
		}
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("context hooks must not create issueops state, got %v", entries)
	}
}

// ISSUEOPS_DISABLE_HOOKS is the kill-switch for repositories the harness does
// not own: the hook must print nothing at all, not even an empty object.
func TestDisableHooksTurnsContextEventIntoSilentNoop(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_DISABLE_HOOKS", "1")
	repo := hookTempRepoWithDoc(t)
	for _, event := range []string{"session-start", "subagent-start"} {
		out := runHookRawCapture(t, `{"cwd":"`+repo+`","source":"startup","agent_type":"general-purpose"}`, func() error {
			return runHook([]string{event, "--host", "claude"})
		})
		if strings.TrimSpace(out) != "" {
			t.Fatalf("disabled %s must emit nothing: %q", event, out)
		}
	}
}

// A subagent starts with an empty context, so it gets the model-facing catalog
// the main session got, under its own event name and without a systemMessage.
func TestRunHookSubagentStartInjectsTheSessionCatalogWithoutANotice(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, tc := range []struct{ host, agentType string }{{"claude", "general-purpose"}, {"codex", "worker"}, {"claude", ""}} {
		session := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", tc.host}) })
		obj := runHookCapture(t, `{"cwd":"`+repo+`","agent_type":"`+tc.agentType+`"}`, func() error { return runHook([]string{"subagent-start", "--host", tc.host}) })
		if hso, _ := obj["hookSpecificOutput"].(map[string]any); hso["hookEventName"] != "SubagentStart" {
			t.Fatalf("%s SubagentStart must name its event: %+v", tc.host, obj)
		}
		if ctx := hookAdditionalContext(obj); ctx == "" || ctx != hookAdditionalContext(session) {
			t.Fatalf("%s subagent catalog must equal the session catalog:\n%q\n%q", tc.host, ctx, hookAdditionalContext(session))
		}
		if _, ok := obj["systemMessage"]; ok {
			t.Fatalf("%s SubagentStart must not show a notice: %+v", tc.host, obj)
		}
	}
}

func TestRunHookSubagentStartSkipsExploreAndForkAgents(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	for _, agentType := range []string{"Explore", "explorer", "FORK", " fork "} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","agent_type":"`+agentType+`"}`, func() error { return runHook([]string{"subagent-start", "--host", "claude"}) })
		if len(obj) != 0 {
			t.Fatalf("agent %q must get an empty object, got %+v", agentType, obj)
		}
	}
}

func TestTopLevelHelpReturnsErrHelp(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, entry := range []string{"--help", "-h", "help"} {
		if err := runHook([]string{entry}); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("%s must return ErrHelp, got %v", entry, err)
		}
	}
	if err := runHook([]string{"session-start", "--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("subcommand --help must return ErrHelp, got %v", err)
	}
}

func TestRetiredHookSubcommandsAreRejected(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	if err := runHook(nil); err == nil {
		t.Fatal("missing subcommand must fail")
	}
	for _, retired := range []string{"user-prompt", "pre-tool-use", "post-tool-use", "pre-compact", "stop", "failures", "metrics"} {
		err := runHook([]string{retired})
		if err == nil || !strings.Contains(err.Error(), "unknown hook subcommand") {
			t.Fatalf("%s must be rejected as unknown, got %v", retired, err)
		}
	}
}

func TestRunHookClaudeLiveProbeWithoutModel(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "observation.json")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", path)
	repo := hookTempRepoWithDoc(t)
	runHookCapture(t, `{"cwd":"`+repo+`","hook_event_name":"SessionStart","source":"startup"}`, func() error { return runHook([]string{"session-start", "--host", "claude"}) })
	data, err := os.ReadFile(path + ".hooks")
	if err != nil {
		t.Fatal(err)
	}
	var marker map[string]string
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatal(err)
	}
	if marker["event"] != "SessionStart" || marker["model"] != "" {
		t.Fatalf("marker=%v", marker)
	}
}
