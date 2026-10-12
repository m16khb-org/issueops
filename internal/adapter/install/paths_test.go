package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentAbsCleanResolvesRelativePathsAgainstWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"bin/../bin/issueops", filepath.Join(cwd, "bin", "issueops")},
		{"/a/b/../c/", "/a/c"},
	} {
		if got := (Environment{}).AbsClean(tc.in); got != tc.want {
			t.Errorf("AbsClean(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
