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

type MaintenanceStores struct {
	base       string
	workerRoot string
}

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

func (stores MaintenanceStores) Roots() ([]string, error) {
	roots := []string{stores.base, filepath.Join(stores.base, "issueops_v1"), stores.workerRoot, filepath.Join(stores.base, "loop")}
	projectRoots, err := stores.projectRoots()
	if err != nil {
		return nil, err
	}
	return append(roots, projectRoots...), nil
}

func (MaintenanceStores) Exists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "issueops.db"))
	return err == nil
}

func (MaintenanceStores) Maintain(root string) (statecontract.StoreMaintainResult, error) {
	database, err := sqlstore.Open(root)
	if err != nil {
		return statecontract.StoreMaintainResult{}, err
	}
	return database.Maintain()
}
