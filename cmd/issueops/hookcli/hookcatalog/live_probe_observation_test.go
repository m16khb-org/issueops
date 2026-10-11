package hookcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/hostprotocol"
	hookcontract "issueops/internal/contract/hookprompt"
)

func liveProbeDir(t *testing.T, perm os.FileMode) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "obs")
	if err := os.Mkdir(dir, perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, perm); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRecordLiveProbeSessionStartDisabledIsNoop(t *testing.T) {
	t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "")
	if err := recordLiveProbeSessionStart([]byte("garbage")); err != nil {
		t.Fatalf("probe disabled must ignore input: %v", err)
	}
}

func TestRecordLiveProbeSessionStartWritesMarker(t *testing.T) {
	dir := liveProbeDir(t, 0o700)
	obs := filepath.Join(dir, "obs")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", obs)
	if err := recordLiveProbeSessionStart([]byte(`{"hook_event_name":"SessionStart","model":"m-1"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(obs + ".hooks")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"event\":\"SessionStart\",\"model\":\"m-1\"}\n" {
		t.Fatalf("marker = %q", got)
	}
	// 마커는 배타적으로 생성되므로 두 번째 기록은 거부되고 기존 내용은 유지된다.
	if err := recordLiveProbeSessionStart([]byte(`{"hook_event_name":"SessionStart","model":"m-2"}`)); err != errLiveProbeObservationInvalid {
		t.Fatalf("second write must fail closed: %v", err)
	}
	again, _ := os.ReadFile(obs + ".hooks")
	if string(again) != string(got) {
		t.Fatalf("existing marker overwritten: %q", again)
	}
}

func TestRecordLiveProbeSessionStartRejectsInvalidSetup(t *testing.T) {
	good := `{"hook_event_name":"SessionStart","model":"m"}`
	open := liveProbeDir(t, 0o755)
	cases := map[string]struct {
		path  string
		input string
	}{
		"relative path":   {"obs", good},
		"empty path":      {"", good},
		"empty input":     {filepath.Join(liveProbeDir(t, 0o700), "o"), ""},
		"oversize input":  {filepath.Join(liveProbeDir(t, 0o700), "o"), strings.Repeat("x", maxLiveProbeHookInput+1)},
		"non-0700 parent": {filepath.Join(open, "o"), good},
		"missing parent":  {filepath.Join(t.TempDir(), "nope", "o"), good},
		"wrong event":     {filepath.Join(liveProbeDir(t, 0o700), "o"), `{"hook_event_name":"Stop","model":"m"}`},
		"malformed json":  {filepath.Join(liveProbeDir(t, 0o700), "o"), `{`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
			t.Setenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", tc.path)
			if err := recordLiveProbeSessionStart([]byte(tc.input)); err != errLiveProbeObservationInvalid {
				t.Fatalf("err = %v", err)
			}
			if tc.path != "" && filepath.IsAbs(tc.path) {
				if _, err := os.Stat(tc.path + ".hooks"); err == nil {
					t.Fatal("marker must not be written")
				}
			}
		})
	}
}

func TestRecordLiveProbeSessionStartRejectsSymlinkParent(t *testing.T) {
	real := liveProbeDir(t, 0o700)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", filepath.Join(link, "o"))
	if err := recordLiveProbeSessionStart([]byte(`{"hook_event_name":"SessionStart"}`)); err != errLiveProbeObservationInvalid {
		t.Fatalf("symlink parent must be rejected: %v", err)
	}
}

func TestDecodeLiveProbeSessionStartModel(t *testing.T) {
	cases := []struct {
		name, in, model string
		ok              bool
	}{
		{"model omitted", `{"hook_event_name":"SessionStart"}`, "", true},
		{"model kept", `{"hook_event_name":"SessionStart","model":"opus"}`, "opus", true},
		{"padded model", `{"hook_event_name":"SessionStart","model":" opus"}`, "", false},
		{"newline model", `{"hook_event_name":"SessionStart","model":"a\nb"}`, "", false},
		{"overlong model", `{"hook_event_name":"SessionStart","model":"` + strings.Repeat("a", 257) + `"}`, "", false},
		{"max length model", `{"hook_event_name":"SessionStart","model":"` + strings.Repeat("a", 256) + `"}`, strings.Repeat("a", 256), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, ok := decodeLiveProbeSessionStartModel([]byte(tc.in))
			if model != tc.model || ok != tc.ok {
				t.Fatalf("got %q,%v", model, ok)
			}
		})
	}
}

func TestWriteLiveProbeMarkerFailsWhenParentMissing(t *testing.T) {
	if err := writeLiveProbeMarker(filepath.Join(t.TempDir(), "nope", "m"), []byte("x")); err != errLiveProbeObservationInvalid {
		t.Fatalf("err = %v", err)
	}
}

func withStdin(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; _ = f.Close() })
}

func TestRunSubagentStartSkipsCatalogForSearchAndForkAgents(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".issueops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", "CONSTITUTION.md"), []byte("# Constitution\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"Explore", "explorer", "fork"} {
		t.Run(agent, func(t *testing.T) {
			withStdin(t, `{"agent_type":"`+agent+`"}`)
			built := false
			var printed any
			config := Config{
				BuildCatalog: func(r string) hookcontract.ProjectDocCatalogContext {
					built = true
					return testCatalogService().Build(r)
				},
				FormatContext: hostprotocol.FormatHookContext,
				ResolveTarget: func(string) string { return repo },
				PrintJSON:     func(v any) error { printed = v; return nil },
			}
			if err := RunSubagentStart([]string{"--host", "claude"}, config); err != nil {
				t.Fatal(err)
			}
			if built {
				t.Fatal("catalog must not be built for skipped agent types")
			}
			if m, ok := printed.(map[string]any); !ok || len(m) != 0 {
				t.Fatalf("printed = %#v", printed)
			}
		})
	}
}

func TestRunSubagentStartInjectsCatalogForOtherAgents(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".issueops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", "CONSTITUTION.md"), []byte("# Constitution\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withStdin(t, `{"agent_type":"general-purpose"}`)
	var printed any
	config := Config{BuildCatalog: testCatalogService().Build, FormatContext: hostprotocol.FormatHookContext,
		ResolveTarget: func(string) string { return repo },
		PrintJSON:     func(v any) error { printed = v; return nil },
	}
	if err := RunSubagentStart([]string{"--host", "claude"}, config); err != nil {
		t.Fatal(err)
	}
	m, ok := printed.(map[string]any)
	hso, hasHSO := m["hookSpecificOutput"].(map[string]any)
	if !ok || !hasHSO || hso["hookEventName"] != "SubagentStart" {
		t.Fatalf("subagent must receive a SubagentStart output: %#v", printed)
	}
	if ctxText, _ := hso["additionalContext"].(string); strings.TrimSpace(ctxText) == "" {
		t.Fatalf("additionalContext must carry the catalog: %#v", hso)
	}
	if _, has := m["systemMessage"]; has {
		t.Fatalf("subagent start must not emit systemMessage: %#v", m)
	}
}

func TestRunSubagentStartJSONPrintsRawCatalog(t *testing.T) {
	withStdin(t, `{"agent_type":"fork"}`)
	var printed any
	config := Config{BuildCatalog: testCatalogService().Build, FormatContext: hostprotocol.FormatHookContext,
		ResolveTarget: func(string) string { return t.TempDir() },
		PrintJSON:     func(v any) error { printed = v; return nil },
	}
	// 건너뛰기 대상은 --json이어도 빈 객체여야 한다.
	if err := RunSubagentStart([]string{"--json"}, config); err != nil {
		t.Fatal(err)
	}
	if m, ok := printed.(map[string]any); !ok || len(m) != 0 {
		t.Fatalf("printed = %#v", printed)
	}
	withStdin(t, `{"agent_type":"worker"}`)
	if err := RunSubagentStart([]string{"--json"}, config); err != nil {
		t.Fatal(err)
	}
	if _, ok := printed.(map[string]any); ok {
		t.Fatalf("--json must print the raw catalog, got %#v", printed)
	}
}
