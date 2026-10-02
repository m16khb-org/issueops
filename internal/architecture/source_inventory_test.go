package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

type dddSourceSnapshot struct {
	inventory dddInventory
	files     map[string]*ast.File // Read-only ASTs shared by architecture checks.
}

type dddSourceSnapshotResult struct {
	snapshot dddSourceSnapshot
	err      error
}

type dddSourceSnapshotCache struct {
	mu    sync.Mutex
	roots map[string]dddSourceSnapshotResult
	read  func(string) (dddSourceSnapshot, error)
}

func (cache *dddSourceSnapshotCache) load(root string) (dddSourceSnapshot, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	root = filepath.Clean(root)
	if result, ok := cache.roots[root]; ok {
		return result.snapshot, result.err
	}
	snapshot, err := cache.read(root)
	if cache.roots == nil {
		cache.roots = make(map[string]dddSourceSnapshotResult)
	}
	cache.roots[root] = dddSourceSnapshotResult{snapshot: snapshot, err: err}
	return snapshot, err
}

var sharedDDDSourceSnapshots = dddSourceSnapshotCache{
	read: func(root string) (dddSourceSnapshot, error) {
		return readDDDSourceSnapshot(root, parseDDDSource)
	},
}

func cachedDDDSourceSnapshot(t *testing.T, root string) dddSourceSnapshot {
	t.Helper()
	snapshot, err := sharedDDDSourceSnapshots.load(root)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func parseDDDSource(path string, data []byte) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), path, data, 0)
}

func readDDDSourceSnapshot(root string, parse func(string, []byte) (*ast.File, error)) (dddSourceSnapshot, error) {
	result := dddSourceSnapshot{
		inventory: dddInventory{Sources: []dddSource{}, Artifacts: []dddArtifactEntry{}},
		files:     make(map[string]*ast.File),
	}
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parse(path, data)
			if err != nil {
				return err
			}
			symbols, err := declarationsFromAST(file)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			owner, task := dddOwner(rel)
			result.inventory.Sources = append(result.inventory.Sources, dddSource{
				Path: rel, Owner: owner, Task: task, Symbols: symbols,
			})
			result.files[rel] = file
			return nil
		})
		if err != nil {
			return dddSourceSnapshot{}, err
		}
	}
	for _, top := range []string{"scripts", "configs", "skills"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !dddArtifact(top, path) {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			artifact, ok := dddArtifactOwner(filepath.ToSlash(rel))
			if !ok {
				return fmt.Errorf("non-Go artifact has no owner or task: %s", rel)
			}
			result.inventory.Artifacts = append(result.inventory.Artifacts, artifact)
			return nil
		})
		if err != nil {
			return dddSourceSnapshot{}, err
		}
	}
	sort.Slice(result.inventory.Sources, func(i, j int) bool {
		return result.inventory.Sources[i].Path < result.inventory.Sources[j].Path
	})
	sort.Slice(result.inventory.Artifacts, func(i, j int) bool {
		return result.inventory.Artifacts[i].Path < result.inventory.Artifacts[j].Path
	})
	return result, nil
}
