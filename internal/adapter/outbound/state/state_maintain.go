package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"issueops/internal/adapter/outbound/sqlstore"
	statecontract "issueops/internal/contract/state"
)

// MaintenanceStores locates the SQLite stores that state maintenance covers: the
// base state directory, issueops_v1, the worker store, loop, and every project
// store.
type MaintenanceStores struct {
	base       string
	workerRoot string
}

// NewMaintenanceStores returns the store locator for base. workerOverride, when
// set, replaces the default worker store directory (base/worker) with its
// absolute path.
func NewMaintenanceStores(base, workerOverride string) MaintenanceStores {
	workerRoot := filepath.Join(base, "worker")
	if dir := workerOverride; dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			workerRoot = abs
		}
	}
	return MaintenanceStores{base: base, workerRoot: workerRoot}
}

func (stores MaintenanceStores) projectRoots() ([]string, error) {
	projectsDir := filepath.Join(stores.base, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("discover project stores %s: %w", projectsDir, err)
	}
	roots := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, entry.Name())
		info, err := os.Lstat(filepath.Join(dir, "issueops.db"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("discover project store %s: %w", dir, err)
		}
		if info.Mode().IsRegular() {
			roots = append(roots, dir)
		}
	}
	return roots, nil
}

// Roots returns every store root to maintain: the fixed roots followed by each
// discovered project store. A failure while scanning the projects directory is an
// error.
func (stores MaintenanceStores) Roots() ([]string, error) {
	roots := []string{stores.base, filepath.Join(stores.base, "issueops_v1"), stores.workerRoot, filepath.Join(stores.base, "loop")}
	projectRoots, err := stores.projectRoots()
	if err != nil {
		return nil, err
	}
	return append(roots, projectRoots...), nil
}

// Exists reports whether root contains an issueops.db store.
func (MaintenanceStores) Exists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "issueops.db"))
	return err == nil
}

// Maintain opens the store in root and runs its maintenance (WAL checkpoint and
// file mode enforcement), returning the result.
func (MaintenanceStores) Maintain(root string) (statecontract.StoreMaintainResult, error) {
	database, err := sqlstore.Open(root)
	if err != nil {
		return statecontract.StoreMaintainResult{}, err
	}
	return database.Maintain()
}
