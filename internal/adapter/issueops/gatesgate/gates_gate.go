// Package gatesgate observes canonical and compatible ledger files for readiness.
package gatesgate

import (
	domain "issueops/internal/domain/issueops"
	"os"
	"path/filepath"
)

type Observer struct{}

func (Observer) DuplicateFiles(root, number string) (bool, []domain.GateLedgerFile) {
	canonical := filepath.Join(root, ".issueops", "issues", number, "gates.md")
	if info, err := os.Stat(canonical); err != nil || info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, ".issueops", "gates"))
	if err != nil {
		return false, nil
	}
	files := make([]domain.GateLedgerFile, 0, len(entries))
	for _, entry := range entries {
		files = append(files, domain.GateLedgerFile{Name: entry.Name(), Directory: entry.IsDir()})
	}
	return true, files
}
