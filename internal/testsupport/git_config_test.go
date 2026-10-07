package testsupport

import (
	"os/exec"
	"strings"
	"testing"
)

func TestIsolateGitConfigHidesTheDevelopersGlobalConfig(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	IsolateGitConfig(t)
	out, err := exec.Command("git", "config", "--global", "--list").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "user.name=issueops-test\nuser.email=issueops-test@example.invalid" {
		t.Fatalf("global config = %q", got)
	}
	if out, err := exec.Command("git", "config", "--system", "--list").CombinedOutput(); err == nil && strings.TrimSpace(string(out)) != "" {
		t.Fatalf("system config still visible: %q", out)
	}
}
