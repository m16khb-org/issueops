package architecture

import (
	"errors"
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDDDSourceSnapshotCacheReusesParsedSources(t *testing.T) {
	root := dddSourceFixture(t)
	loads := 0
	parses := map[string]int{}
	cache := dddSourceSnapshotCache{read: func(root string) (dddSourceSnapshot, error) {
		loads++
		return readDDDSourceSnapshot(root, func(path string, data []byte) (*ast.File, error) {
			parses[path]++
			return parseDDDSource(path, data)
		})
	}}
	for range 3 {
		snapshot, err := cache.load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.inventory.Sources) != 2 || len(snapshot.files) != 2 || len(snapshot.inventory.Artifacts) != 1 {
			t.Fatalf("incomplete snapshot: %+v", snapshot.inventory)
		}
	}
	if loads != 1 || len(parses) != 2 {
		t.Fatalf("loads=%d parsed files=%d, want 1 and 2", loads, len(parses))
	}
	for path, count := range parses {
		if count != 1 {
			t.Fatalf("%s parsed %d times", path, count)
		}
	}
}

func TestDDDSourceSnapshotCacheSeparatesRoots(t *testing.T) {
	first, second := dddSourceFixture(t), dddSourceFixture(t)
	path := filepath.Join(second, "cmd", "one.go")
	if err := os.WriteFile(path, []byte("package fixture\nfunc Different() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := dddSourceSnapshotCache{read: func(root string) (dddSourceSnapshot, error) {
		return readDDDSourceSnapshot(root, parseDDDSource)
	}}
	one, err := cache.load(first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := cache.load(second)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(one.inventory.Sources, two.inventory.Sources) {
		t.Fatal("different repository roots shared source declarations")
	}
	if !reflect.DeepEqual(two.inventory.Sources[0].Symbols, []string{"func Different"}) {
		t.Fatalf("second root declarations: %v", two.inventory.Sources[0].Symbols)
	}
}

func TestDDDInventoryCacheReturnsIndependentViews(t *testing.T) {
	root := dddSourceFixture(t)
	first := collectDDDInventory(t, root)
	want, err := readDDDSourceSnapshot(root, parseDDDSource)
	if err != nil {
		t.Fatal(err)
	}
	first.Sources[0].Path = "mutated"
	first.Sources[0].Symbols[0] = "mutated"
	first.Artifacts[0].Path = "mutated"
	if got := collectDDDInventory(t, root); !reflect.DeepEqual(got, want.inventory) {
		t.Fatalf("consumer mutated the cached inventory: %+v", got)
	}
}

func TestDDDSourceSnapshotReportsReadParseAndOwnershipErrors(t *testing.T) {
	if _, err := readDDDSourceSnapshot(t.TempDir(), parseDDDSource); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source directory: %v", err)
	}
	root := dddSourceFixture(t)
	parseFailure := errors.New("fixture parser failure")
	loads := 0
	cache := dddSourceSnapshotCache{read: func(root string) (dddSourceSnapshot, error) {
		loads++
		return readDDDSourceSnapshot(root, func(string, []byte) (*ast.File, error) {
			return nil, parseFailure
		})
	}}
	for range 2 {
		if _, err := cache.load(root); !errors.Is(err, parseFailure) {
			t.Fatalf("parse error was lost: %v", err)
		}
	}
	if loads != 1 {
		t.Fatalf("failed snapshot was loaded %d times", loads)
	}
	path := filepath.Join(root, "scripts", "unassigned.sh")
	if err := os.WriteFile(path, []byte("exit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDDDSourceSnapshot(root, parseDDDSource); err == nil || !strings.Contains(err.Error(), "unassigned.sh") {
		t.Fatalf("unassigned artifact was accepted: %v", err)
	}
}

func TestDDDSourceSnapshotFreshCollectionSeesChangedSource(t *testing.T) {
	root := dddSourceFixture(t)
	before := collectDDDInventory(t, root)
	path := filepath.Join(root, "cmd", "one.go")
	if err := os.WriteFile(path, []byte("package fixture\nfunc Added() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh, err := readDDDSourceSnapshot(root, parseDDDSource)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before.Sources, fresh.inventory.Sources) ||
		!reflect.DeepEqual(fresh.inventory.Sources[0].Symbols, []string{"func Added"}) {
		t.Fatalf("fresh collection missed changed bytes: %+v", fresh.inventory.Sources)
	}
}

func dddSourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"cmd", "internal/domain/example", "scripts", "configs", "skills"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		"cmd/one.go":                        "package fixture\nfunc First() {}\n",
		"internal/domain/example/a.go":      "package fixture\ntype Example struct{}\n",
		"internal/domain/example/a_test.go": "ignored test source",
		"configs/upstream.json":             "{}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
