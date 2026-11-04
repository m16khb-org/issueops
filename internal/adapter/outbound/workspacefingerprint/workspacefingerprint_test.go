package workspacefingerprint

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
)

func TestWriteFileRejectsChangedUntrackedFile(t *testing.T) {
	t.Parallel()

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
			listed, listedKnown := osfile.PathIdentity(path, entry)
			test.change(t, path)

			err = writeFile(sha256.New(), path, entry, listed, listedKnown)
			if err == nil || !strings.Contains(err.Error(), "untracked file changed while snapshotting") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func digestFixture(t *testing.T, content string) Inputs {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Inputs{Root: root, CommonDir: ".git", Branch: "main", Head: "abc", Index: []byte("idx"), Tracked: "t", Staged: "s", Untracked: []string{"new.txt"}}
}

func TestDigestIsDeterministic(t *testing.T) {
	t.Parallel()

	base := digestFixture(t, "one")
	first, err := Digest(base)
	if err != nil || first == "" {
		t.Fatalf("digest = %q, %v", first, err)
	}
	if again, err := Digest(base); err != nil || again != first {
		t.Fatalf("digest is not deterministic: %q, %v", again, err)
	}
}

func TestDigestBindsEveryInput(t *testing.T) {
	t.Parallel()

	base := digestFixture(t, "one")
	first, _ := Digest(base)
	mutations := map[string]func(*Inputs){
		"root":      func(in *Inputs) { *in = digestFixture(t, "one") },
		"commonDir": func(in *Inputs) { in.CommonDir = "other" },
		"branch":    func(in *Inputs) { in.Branch = "feature" },
		"head":      func(in *Inputs) { in.Head = "def" },
		"index":     func(in *Inputs) { in.Index = []byte("idx2") },
		"tracked":   func(in *Inputs) { in.Tracked = "t2" },
		"staged":    func(in *Inputs) { in.Staged = "s2" },
		"untracked": func(in *Inputs) { in.Untracked = nil },
	}
	for name, mutate := range mutations {
		changed := base
		mutate(&changed)
		if got, err := Digest(changed); err != nil || got == first {
			t.Errorf("%s change did not move the digest: %q, %v", name, got, err)
		}
	}
}

func TestDigestBindsUntrackedContent(t *testing.T) {
	t.Parallel()

	base := digestFixture(t, "one")
	first, _ := Digest(base)
	if err := os.WriteFile(filepath.Join(base.Root, "new.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := Digest(base); err != nil || got == first {
		t.Errorf("untracked content change did not move the digest: %q, %v", got, err)
	}
}

func TestDigestRefusesUnsafeOrIrregularUntrackedEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, relative := range map[string]string{
		"escape":    "../escape.txt",
		"directory": "dir",
		"missing":   "missing.txt",
	} {
		if _, err := Digest(Inputs{Root: root, Untracked: []string{relative}}); err == nil {
			t.Errorf("%s: untracked entry %q was accepted", name, relative)
		}
	}
}

func TestReadIndexTreatsMissingAsEmptyAndUnreadableAsFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if data, err := ReadIndex(root, "index"); err != nil || data != nil {
		t.Fatalf("missing index = %q, %v; want nil", data, err)
	}
	if err := os.WriteFile(filepath.Join(root, "index"), []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadIndex(root, "index"); err != nil || string(data) != "bytes" {
		t.Fatalf("relative index = %q, %v", data, err)
	}
	if data, err := ReadIndex("", filepath.Join(root, "index")); err != nil || string(data) != "bytes" {
		t.Fatalf("absolute index = %q, %v", data, err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir-index"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIndex(root, "dir-index"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unreadable index must fail closed, err=%v", err)
	}
}

func fakeDiffGit(fail string) GitRaw {
	return func(_ string, args ...string) (int, string, string) {
		switch {
		case args[0] == "diff" && args[1] == "--binary":
			return code(fail, "tracked"), "tracked-diff", "tracked-err"
		case args[0] == "diff":
			return code(fail, "staged"), "staged-diff", "staged-err"
		default:
			return code(fail, "untracked"), "b.txt\x00a.txt\x00", "untracked-err"
		}
	}
}

func code(fail, step string) int {
	if fail == step {
		return 1
	}
	return 0
}

func TestReadDiffsReturnsDiffsAndSortedUntrackedList(t *testing.T) {
	t.Parallel()

	tracked, staged, untracked, err := ReadDiffs(fakeDiffGit(""), "root")
	if err != nil || tracked != "tracked-diff" || staged != "staged-diff" || strings.Join(untracked, ",") != "a.txt,b.txt" {
		t.Fatalf("diffs = %q %q %v, %v", tracked, staged, untracked, err)
	}
	empty := func(string, ...string) (int, string, string) { return 0, "", "" }
	if _, _, untracked, err := ReadDiffs(empty, "root"); err != nil || untracked != nil {
		t.Fatalf("empty listing = %v, %v; want nil", untracked, err)
	}
}

func TestReadDiffsNamesTheFailingCommand(t *testing.T) {
	t.Parallel()

	for fail, want := range map[string]string{
		"tracked":   "read tracked diff: tracked-err",
		"staged":    "read staged diff: staged-err",
		"untracked": "list untracked files: untracked-err",
	} {
		if _, _, _, err := ReadDiffs(fakeDiffGit(fail), "root"); err == nil || err.Error() != want {
			t.Errorf("%s failure = %v; want %q", fail, err, want)
		}
	}
}
