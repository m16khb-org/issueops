package gitbin

import (
	"errors"
	"path/filepath"
	"testing"
)

// The shim's choice: DEVELOPER_DIR first, then the xcode-select link; a
// developer dir without git keeps the shim.
func TestShimTargetFollowsTheDeveloperDirectoryTheShimWouldUse(t *testing.T) {
	// shimTarget joins with filepath, so the fixtures use the OS separator.
	p := filepath.FromSlash
	present := map[string]bool{
		p("/Applications/Xcode.app/Contents/Developer/usr/bin/git"): true,
		p("/Library/Developer/CommandLineTools/usr/bin/git"):        true,
	}
	exists := func(path string) bool { return present[path] }
	link := func(string) (string, error) { return p("/Library/Developer/CommandLineTools"), nil }
	env := func(dir string) func(string) string {
		return func(key string) string {
			if key == "DEVELOPER_DIR" {
				return dir
			}
			return ""
		}
	}
	if got := shimTarget(env(""), link, exists); got != p("/Library/Developer/CommandLineTools/usr/bin/git") {
		t.Fatalf("xcode-select link: %q", got)
	}
	if got := shimTarget(env(p("/Applications/Xcode.app/Contents/Developer")), link, exists); got != p("/Applications/Xcode.app/Contents/Developer/usr/bin/git") {
		t.Fatalf("DEVELOPER_DIR must win: %q", got)
	}
	noLink := func(string) (string, error) { return "", errors.New("absent") }
	if got := shimTarget(env(""), noLink, exists); got != "" {
		t.Fatalf("no developer dir keeps the shim: %q", got)
	}
	if got := shimTarget(env(p("/opt/empty")), link, exists); got != "" {
		t.Fatalf("a developer dir without git keeps the shim: %q", got)
	}
}

func TestPathHonoursPATHOnEveryCall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := Path(); got != "git" {
		t.Fatalf("git absent from PATH resolves to %q", got)
	}
}
