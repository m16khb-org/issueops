package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupFinishEnvironmentDistinguishesAbsenceFromWrongFileType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("not a worktree"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path            string
		present, failed bool
	}{
		{root, true, false}, {filepath.Join(root, "absent"), false, false}, {file, false, true}, {link, false, true},
	} {
		present, err := (CleanupFinishEnvironment{}).Directory(tc.path)
		if present != tc.present || (err != nil) != tc.failed {
			t.Fatalf("path=%s present=%t err=%v", tc.path, present, err)
		}
	}
}
