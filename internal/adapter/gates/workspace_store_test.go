package gates

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (WorkspaceFileStore, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return WorkspaceFileStore{Root: root, CWD: root}, root
}

func TestWorkspaceStoreCreateReadExists(t *testing.T) {
	store, root := newTestStore(t)
	if store.ExistsFile("sub/GATES.md") {
		t.Fatal("must not exist yet")
	}
	if err := store.Create("sub/dir/GATES.md", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "sub", "dir", "GATES.md"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	got, err := store.Read("sub/dir/GATES.md")
	if err != nil || string(got) != "hello" {
		t.Fatalf("read=%q err=%v", got, err)
	}
	if !store.ExistsFile("sub/dir/GATES.md") || store.ExistsFile("sub/dir") {
		t.Fatal("file exists, directory does not count")
	}
	abs := filepath.Join(root, "sub", "dir", "GATES.md")
	if !store.ExistsFile(abs) {
		t.Fatal("absolute path inside root must resolve")
	}
	if _, err := store.Read("missing.md"); err == nil {
		t.Fatal("reading a missing file must fail")
	}
}

func TestWorkspaceStoreRejectsEscapes(t *testing.T) {
	store, root := newTestStore(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape.md", filepath.Join(outside, "secret.md")} {
		if _, err := store.Read(name); err == nil || !strings.Contains(err.Error(), "outside the authorized workspace") {
			t.Errorf("read %q err=%v", name, err)
		}
		if err := store.Create(name, []byte("y")); err == nil {
			t.Errorf("create %q must fail", name)
		}
		if store.ExistsFile(name) {
			t.Errorf("exists %q must be false", name)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Read("link/secret.md"); err == nil {
			t.Fatal("symlink escape must be refused")
		}
		if err := store.Create("link/new.md", []byte("y")); err == nil {
			t.Fatal("symlink write escape must be refused")
		}
		if _, err := os.Stat(filepath.Join(outside, "new.md")); err == nil {
			t.Fatal("file leaked outside workspace")
		}
	}
}

func TestWorkspaceStoreRelativeToCWD(t *testing.T) {
	store, root := newTestStore(t)
	if err := os.MkdirAll(filepath.Join(root, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	store.CWD = filepath.Join(root, "work")
	if err := store.Create("G.md", []byte("z")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "work", "G.md")); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceStoreWritePreservingMode(t *testing.T) {
	store, root := newTestStore(t)
	path := filepath.Join(root, "exec.md")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WritePreservingMode("exec.md", []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "new" || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("data=%q mode=%v", data, info.Mode().Perm())
	}
	if err := store.WritePreservingMode("fresh.md", []byte("f")); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Stat(filepath.Join(root, "fresh.md"))
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("new file mode=%v", info.Mode().Perm())
	}
}

func TestWorkspaceStoreDiscoverFindsGateFilesInsideRoot(t *testing.T) {
	store, root := newTestStore(t)
	writeTestGateFile(t, root, ".issueops/issues/7/gates.md", "# g\n")
	files, err := store.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f), "issues/7/gates.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("files=%v", files)
	}
	if _, err := store.Discover(t.TempDir()); err == nil {
		t.Fatal("discovery outside workspace must fail")
	}
}

func TestWorkspaceStoreMissingRootFails(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "nope")
	store := WorkspaceFileStore{Root: missing, CWD: missing}
	if _, err := store.Read("a.md"); err == nil || strings.Contains(err.Error(), "outside the authorized workspace") {
		t.Fatalf("missing root must fail on open, not containment: %v", err)
	}
	if _, err := store.Discover(store.CWD); err == nil || strings.Contains(err.Error(), "outside the authorized workspace") {
		t.Fatalf("missing root discover must fail on open, not containment: %v", err)
	}
}
