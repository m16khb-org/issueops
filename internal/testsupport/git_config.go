package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

// isolatedGitConfig keeps commits working without the developer's identity.
const isolatedGitConfig = "[user]\n\tname = issueops-test\n\temail = issueops-test@example.invalid\n"

// IsolateGitConfig points git at a scratch global config and an empty system
// config for the rest of the test, so a developer's or CI runner's
// commit.gpgsign, core.hooksPath, or init.defaultBranch cannot change what a
// repo fixture does. GIT_CONFIG_NOSYSTEM alone still lets an explicit
// `git config --system` read the machine's file (the GitHub runner ships
// safe.directory and git-lfs filters there), so GIT_CONFIG_SYSTEM is pointed
// at the null device as well.
// It uses t.Setenv, so the calling test must not be parallel.
func IsolateGitConfig(t testing.TB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(isolatedGitConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}
