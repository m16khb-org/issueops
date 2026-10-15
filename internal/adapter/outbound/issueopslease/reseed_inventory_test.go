package issueopslease

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestReseedWriteFingerprintFileRejectsChangedUntrackedFile(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, path string)
	}{
		{
			name: "replaced",
			change: func(t *testing.T, path string) {
				replacement := path + ".replacement"
				if err := os.WriteFile(replacement, []byte("same"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "resized",
			change: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("different-size"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "untracked")
			if err := os.WriteFile(path, []byte("same"), 0o600); err != nil {
				t.Fatal(err)
			}
			entry, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			test.change(t, path)

			err = reseedWriteFingerprintFile(sha256.New(), path, entry)
			if err == nil || !strings.Contains(err.Error(), "untracked file changed while snapshotting") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestReseedWorkspaceSnapshotStreamsLargeUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	reseedInventoryGit(t, root, "init", "--initial-branch", "reseed")
	reseedInventoryGit(t, root, "config", "user.email", "test@example.invalid")
	reseedInventoryGit(t, root, "config", "user.name", "Test")
	reseedInventoryGit(t, root, "commit", "--allow-empty", "--message", "initial")
	large := filepath.Join(root, "large.bin")
	file, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(32 << 20); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := reseedWorkspaceSnapshot(leasecontract.Workspace{Root: root, Branch: "reseed"}); err != nil {
		t.Fatal(err)
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("reseed snapshot allocated %d bytes while hashing a 32 MiB untracked file", allocated)
	}
}

func TestReseedWorkspaceSnapshotAllowsSuccessfulGitDiffWarnings(t *testing.T) {
	root := t.TempDir()
	reseedInventoryGit(t, root, "init", "--initial-branch", "reseed")
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("tracked.txt filter=warning\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reseedInventoryGit(t, root, "config", "user.email", "test@example.invalid")
	reseedInventoryGit(t, root, "config", "user.name", "Test")
	reseedInventoryGit(t, root, "config", "filter.warning.clean", "sh -c 'cat; printf warning-from-clean-filter >&2'")
	reseedInventoryGit(t, root, "config", "filter.warning.smudge", "cat")
	reseedInventoryGit(t, root, "add", ".gitattributes", "tracked.txt")
	reseedInventoryGit(t, root, "commit", "--message", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := reseedWorkspaceSnapshot(leasecontract.Workspace{Root: root, Branch: "reseed"}); err != nil {
		t.Fatalf("successful git diff warnings must not invalidate the reseed snapshot: %v", err)
	}
}

func reseedInventoryGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
