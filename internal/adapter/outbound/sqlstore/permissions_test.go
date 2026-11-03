package sqlstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/testsupport"
)

func TestEnsurePrivateRootCreatesMissingParents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "parent", "state")
	if err := ensurePrivateRoot(root); err != nil {
		t.Fatal(err)
	}
	assertMode(t, root, 0o700)
	assertMode(t, filepath.Dir(root), 0o700)
}

func TestEnsurePrivateRootPreservesNonDirectoryError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(root, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, root, 0o644)
	err := ensurePrivateRoot(root)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "mkdir" || pathErr.Path != root || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("non-directory error changed: %v", err)
	}
	assertMode(t, root, 0o644)
}

func TestOpenRepairsPermissiveRootAndKnownFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, dir, 0o755)
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	journal := filepath.Join(dir, spanDBFile+"-journal")
	if err := os.WriteFile(journal, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("leave me"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, unrelated, 0o644)
	testsupport.MakePermissive(t, dir, 0o755)
	for _, path := range existingKnownSQLiteFiles(t, dir) {
		testsupport.MakePermissive(t, path, 0o644)
	}

	again, err := Open(dir)
	if err != nil {
		t.Fatalf("cached Open: %v", err)
	}
	if again != d {
		t.Fatal("cached Open returned a different handle")
	}
	assertMode(t, dir, 0o700)
	for _, path := range existingKnownSQLiteFiles(t, dir) {
		assertMode(t, path, 0o600)
	}
	assertMode(t, unrelated, 0o644)
}

func TestOpenRejectsSymlinkRootWithoutChangingTarget(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, target, 0o755)
	root := filepath.Join(parent, "root")
	if err := os.Symlink(target, root); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(root); err == nil {
		t.Fatal("Open accepted a symlink root")
	} else if !strings.Contains(err.Error(), root) {
		t.Fatalf("root error lacks path %q: %v", root, err)
	}
	assertMode(t, target, 0o755)
}

func TestOpenRejectsSymlinkKnownFileWithoutChangingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.db")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, target, 0o644)
	if err := os.Symlink(target, filepath.Join(dir, dataDBFile)); err != nil {
		t.Fatal(err)
	}

	dataPath := filepath.Join(dir, dataDBFile)
	if _, err := Open(dir); err == nil {
		t.Fatal("Open accepted a symlink database file")
	} else if !strings.Contains(err.Error(), dataPath) {
		t.Fatalf("database error lacks path %q: %v", dataPath, err)
	}
	assertMode(t, target, 0o644)
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "target" {
		t.Fatalf("symlink target changed: data=%q err=%v", data, err)
	}
}

func TestOpenRejectsNonRegularKnownFile(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, dataDBFile)
	if err := os.Mkdir(dataPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("Open accepted a non-regular database path")
	} else if !strings.Contains(err.Error(), dataPath) {
		t.Fatalf("database error lacks path %q: %v", dataPath, err)
	}
}

func TestMaintainUsesExactPermissionRepair(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(dir, spanDBFile+"-journal")
	if err := os.WriteFile(journal, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(dir, "unrelated.db")
	if err := os.WriteFile(unrelated, []byte("leave me"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, unrelated, 0o644)
	var wantFixed []string
	for _, path := range existingKnownSQLiteFiles(t, dir) {
		testsupport.MakePermissive(t, path, 0o644)
		wantFixed = append(wantFixed, filepath.Base(path))
	}
	sort.Strings(wantFixed)

	result, err := d.Maintain()
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	for _, path := range existingKnownSQLiteFiles(t, dir) {
		assertMode(t, path, 0o600)
	}
	assertMode(t, unrelated, 0o644)
	sort.Strings(result.PermissionsFixed)
	if !reflect.DeepEqual(result.PermissionsFixed, wantFixed) {
		t.Fatalf("PermissionsFixed=%v want=%v", result.PermissionsFixed, wantFixed)
	}
}

func existingKnownSQLiteFiles(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	for _, base := range [...]string{dataDBFile, spanDBFile} {
		for _, suffix := range sqliteFileSuffixes {
			path := filepath.Join(dir, base+suffix)
			if _, err := os.Lstat(path); err == nil {
				paths = append(paths, path)
			} else if !os.IsNotExist(err) {
				t.Fatalf("lstat %s: %v", path, err)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if got := osfile.Perm(path, info); got != osPerm(want) {
		t.Fatalf("%s mode=%#o want=%#o", path, got, osPerm(want))
	}
}

// osPerm renders a Unix mode as osfile.Perm reports it. Windows has no
// separate group or other bits: a non-private object mirrors its owner bits.
func osPerm(unix os.FileMode) os.FileMode {
	if runtime.GOOS != "windows" || unix&0o077 == 0 {
		return unix
	}
	owner := unix & 0o700
	return owner | owner>>3 | owner>>6
}
