package modelcli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 연결 워크트리에서 실행한 실제 바이너리가 local 설정을 메인 워크트리에 쓰고,
// 그 파일이 두 체크아웃의 git status에 나타나지 않는다.
func TestLocalScopeFromLinkedWorktreeBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the issueops binary")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "issueops")
	build := exec.Command("go", "build", "-o", binary, "./cmd/issueops")
	build.Dir = filepath.Join("..", "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	main, linked := filepath.Join(root, "main"), filepath.Join(root, "linked")
	if err := os.Mkdir(main, 0o755); err != nil {
		t.Fatal(err)
	}
	env := cleanEnv(root)
	git := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		command.Env = env
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(main, "init", "-q")
	git(main, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	git(main, "worktree", "add", "-q", "-b", "feature", linked)

	issueops := func(args ...string) map[string]any {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir, command.Env = linked, env
		out, err := command.Output()
		if err != nil {
			t.Fatalf("issueops %v: %v\n%s", args, err, out)
		}
		var decoded map[string]any
		if err := json.Unmarshal(out, &decoded); err != nil {
			t.Fatalf("issueops %v output is not JSON: %s", args, out)
		}
		return decoded
	}
	set := issueops("model", "set", "--scope", "local", "--host", "codex", "--role", "research", "--model", "gpt-6-luna", "--effort", "low", "--json")
	if !strings.HasSuffix(set["path"].(string), "/main/.issueops/agent-models.local.json") || set["excluded"] != true {
		t.Fatalf("set = %v", set)
	}
	resolved := issueops("model", "resolve", "--host", "codex", "--role", "research", "--json")
	if resolved["model"] != "gpt-6-luna" || resolved["effort"] != "low" || resolved["model_source"] != "local" || resolved["effort_source"] != "local" {
		t.Fatalf("resolve = %v", resolved)
	}
	for _, dir := range []string{main, linked} {
		if status := git(dir, "status", "--short"); status != "" {
			t.Fatalf("%s status shows the local settings file:\n%s", dir, status)
		}
	}
}

// cleanEnv isolates HOME, XDG, and state, and drops GIT_* so the test repo
// is not confused with the checkout running the tests.
func cleanEnv(root string) []string {
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GIT_") || strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_CONFIG_HOME=") || strings.HasPrefix(entry, "ISSUEOPS_STATE_DIR=") || strings.HasPrefix(entry, "CODEX_HOME=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+root, "XDG_CONFIG_HOME="+filepath.Join(root, "xdg"), "ISSUEOPS_STATE_DIR="+filepath.Join(root, "state"), "CODEX_HOME="+filepath.Join(root, "codex"))
}
