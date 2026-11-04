// Package state is the outbound adapter that binds the state application service
// to the process state directory and the shared SQLite store, and provides the
// doctor and store-maintenance operations over it.
package state

import (
	"errors"
	"io/fs"
	statedomain "issueops/internal/domain/state"
	statepath "issueops/internal/domain/statepath"
	"os"
	"path/filepath"

	"issueops/internal/adapter/outbound/sqlstore"
	stateapplication "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
)

// Doctor inspects the state directory dir and its SQLite records and returns a
// health report. A missing directory or store is reported as empty; read
// failures return a failed result together with the error.
func Doctor(dir string) (statecontract.DoctorResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failedDoctorResult(dir), err
	}
	entrySnapshots := make([]statedomain.DoctorEntry, 0, len(entries))
	for _, entry := range entries {
		entrySnapshots = append(entrySnapshots, statedomain.DoctorEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(dir, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}
	rows, err := sqlstore.GetAllExisting(dir, stateBucket)
	if errors.Is(err, fs.ErrNotExist) {
		rows = nil
	} else if err != nil {
		return failedDoctorResult(dir), err
	}
	rowSnapshots := make([]stateapplication.DoctorRow, 0, len(rows))
	for _, row := range rows {
		rowSnapshots = append(rowSnapshots, stateapplication.DoctorRow{
			Key:  row.ID,
			Path: statepath.Path(dir, row.ID),
			Data: row.Data,
		})
	}
	return stateapplication.Doctor(dir, entrySnapshots, rowSnapshots), nil
}

func failedDoctorResult(dir string) statecontract.DoctorResult {
	return statecontract.DoctorResult{
		OK:        false,
		Healthy:   false,
		StateDir:  dir,
		ValidKeys: []string{},
		Valid:     []statecontract.ListEntry{},
		Issues:    []statecontract.DoctorIssue{},
	}
}
