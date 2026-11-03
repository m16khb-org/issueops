package install

import (
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/testsupport"
)

func TestEnvironmentAbsCleanResolvesRelativePathsAgainstWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"bin/../bin/issueops", filepath.Join(cwd, "bin", "issueops")},
		{testsupport.AbsPath("a", "b") + string(filepath.Separator) + ".." + string(filepath.Separator) + "c" + string(filepath.Separator), testsupport.AbsPath("a", "c")},
	} {
		if got := (Environment{}).AbsClean(tc.in); got != tc.want {
			t.Errorf("AbsClean(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
