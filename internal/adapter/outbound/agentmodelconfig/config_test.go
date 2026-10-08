package agentmodelconfig

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/agentmodel"
)

func TestGlobalPath(t *testing.T) {
	env := map[string]string{"XDG_CONFIG_HOME": "/xdg"}
	if got := GlobalPath(func(k string) string { return env[k] }, "/home/u"); got != "/xdg/issueops/agent-models.json" {
		t.Fatalf("GlobalPath = %s", got)
	}
	if got := GlobalPath(func(string) string { return "" }, "/home/u"); got != "/home/u/.config/issueops/agent-models.json" {
		t.Fatalf("GlobalPath fallback = %s", got)
	}
}

func TestLocalPathFromLinkedWorktree(t *testing.T) {
	main, linked := linkedRepo(t)
	for _, repo := range []string{main, linked} {
		got, err := LocalPath(context.Background(), repo)
		if err != nil {
			t.Fatal(err)
		}
		if evalPath(t, filepath.Dir(filepath.Dir(got.Path))) != evalPath(t, main) || !strings.HasSuffix(got.Path, "/.issueops/agent-models.local.json") {
			t.Fatalf("LocalPath(%s) = %s, want it under the main worktree %s", repo, got.Path, main)
		}
	}
	if got, err := LocalPath(context.Background(), t.TempDir()); err != nil || got.Path != "" {
		t.Fatalf("outside git LocalPath = %+v, %v", got, err)
	}
}

func TestWriteLocalStaysOutOfGitStatus(t *testing.T) {
	main, linked := linkedRepo(t)
	local, err := LocalPath(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	cfg := contract.Config{Version: 1, Codex: map[contract.Role]contract.Layer{contract.RoleResearch: {Model: "gpt-6-luna", Effort: "low"}}}
	excluded, err := WriteLocal(local, cfg)
	if err != nil || !excluded {
		t.Fatalf("WriteLocal excluded=%v err=%v", excluded, err)
	}
	for _, repo := range []string{main, linked} {
		if out := git(t, repo, "status", "--short"); out != "" {
			t.Fatalf("%s status shows the local settings file:\n%s", repo, out)
		}
	}
	got, err := Read(local.Path)
	if err != nil || !reflect.DeepEqual(got, cfg) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestEnsureExcludeIdempotent(t *testing.T) {
	main, _ := linkedRepo(t)
	local, err := LocalPath(context.Background(), main)
	if err != nil {
		t.Fatal(err)
	}
	first, err := EnsureExcluded(local.CommonDir)
	if err != nil || !first {
		t.Fatalf("first EnsureExcluded = %v, %v", first, err)
	}
	second, err := EnsureExcluded(local.CommonDir)
	if err != nil || second {
		t.Fatalf("second EnsureExcluded = %v, %v", second, err)
	}
	data, _ := os.ReadFile(filepath.Join(local.CommonDir, "info", "exclude"))
	if strings.Count(string(data), excludeLine) != 1 {
		t.Fatalf("exclude file:\n%s", data)
	}
}

func TestReadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-models.json")
	write(t, path, `{"version":1,"claude":{"implement":{"modle":"x"}}}`)
	_, err := Read(path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), `unknown field "modle"`) {
		t.Fatalf("Read err = %v", err)
	}
	for _, body := range []string{`{"version":2}`, `{"version":1,"claude":{"reviewer":{"model":"opus"}}}`, `{"version":1,"omo":{}}`, `{"version":1} {}`} {
		write(t, path, body)
		if _, err := Read(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("Read(%s) err = %v", body, err)
		}
	}
	if cfg, err := Read(filepath.Join(t.TempDir(), "absent.json")); err != nil || !reflect.DeepEqual(cfg, contract.Config{}) {
		t.Fatalf("absent file = %+v, %v", cfg, err)
	}
}

func TestWriteIsDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agent-models.json")
	cfg := contract.Config{Version: 1,
		Codex:  map[contract.Role]contract.Layer{contract.RoleResearch: {Model: "gpt-6-luna"}, contract.RoleDiffReview: {Effort: "xhigh"}},
		Claude: map[contract.Role]contract.Layer{contract.RoleImplement: {Model: "opus", Effort: "high"}}}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := "{\n  \"version\": 1,\n  \"claude\": {\n    \"implement\": {\n      \"model\": \"opus\",\n      \"effort\": \"high\"\n    }\n  },\n  \"codex\": {\n    \"diff-review\": {\n      \"effort\": \"xhigh\"\n    },\n    \"research\": {\n      \"model\": \"gpt-6-luna\"\n    }\n  }\n}\n"
	if string(data) != want {
		t.Fatalf("written file:\n%s", data)
	}
}

func TestCodexCatalog(t *testing.T) {
	home := t.TempDir()
	env := func(k string) string {
		if k == "CODEX_HOME" {
			return home
		}
		return ""
	}
	if got := CodexCatalog(env, "/nowhere"); got != nil {
		t.Fatalf("absent catalog = %v", got)
	}
	write(t, filepath.Join(home, "models_cache.json"), `{"models":[{"slug":"gpt-6-luna"},{"slug":"gpt-6-astra"},{"slug":""}]}`)
	if got := CodexCatalog(env, "/nowhere"); !reflect.DeepEqual(got, []string{"gpt-6-luna", "gpt-6-astra"}) {
		t.Fatalf("catalog = %v", got)
	}
	write(t, filepath.Join(home, "models_cache.json"), `not json`)
	if got := CodexCatalog(env, "/nowhere"); got != nil {
		t.Fatalf("broken catalog = %v", got)
	}
}

func TestWriteRoleFileIsContentAddressed(t *testing.T) {
	dir := t.TempDir()
	first, err := WriteRoleFile(dir, "model = \"gpt-6-luna\"\n")
	if err != nil {
		t.Fatal(err)
	}
	second, err := WriteRoleFile(dir, "model = \"gpt-6-luna\"\n")
	if err != nil || second != first || filepath.Dir(first) != filepath.Join(dir, "agent-roles") || filepath.Ext(first) != ".toml" {
		t.Fatalf("WriteRoleFile = %s, %s, %v", first, second, err)
	}
	other, _ := WriteRoleFile(dir, "model = \"gpt-6-astra\"\n")
	if other == first {
		t.Fatal("different content must get a different path")
	}
}

func linkedRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	linked := filepath.Join(root, "linked")
	if err := os.Mkdir(main, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, main, "init", "-q")
	git(t, main, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	git(t, main, "worktree", "add", "-q", "-b", "feature", linked)
	return main, linked
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = withoutGitEnvironment(os.Environ())
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func evalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestLoadNamesTheBrokenFile(t *testing.T) {
	main, linked := linkedRepo(t)
	xdg := t.TempDir()
	env := func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return xdg
		}
		return ""
	}
	write(t, filepath.Join(xdg, "issueops", "agent-models.json"), `{"version":1,"codex":{"research":{"model":"gpt-6-luna"}}}`)
	write(t, filepath.Join(main, ".issueops", "agent-models.local.json"), `{"version":1,"codex":{"research":{"effort":"low"}}}`)
	files, err := Load(context.Background(), linked, env, "/nowhere")
	if err != nil || files.Global.Codex[contract.RoleResearch].Model != "gpt-6-luna" || files.Local.Codex[contract.RoleResearch].Effort != "low" {
		t.Fatalf("Load = %+v, %v", files, err)
	}
	write(t, filepath.Join(main, ".issueops", "agent-models.local.json"), `{"version":2}`)
	if _, err := Load(context.Background(), linked, env, "/nowhere"); err == nil || !strings.Contains(err.Error(), "agent-models.local.json") {
		t.Fatalf("broken local err = %v", err)
	}
	if files, err := Load(context.Background(), t.TempDir(), env, "/nowhere"); err != nil || files.LocalPath != "" {
		t.Fatalf("outside git Load = %+v, %v", files, err)
	}
}
