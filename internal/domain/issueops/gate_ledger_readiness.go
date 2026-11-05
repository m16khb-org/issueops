package issueops

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GateLedgerStatus is the parsed state of one gate in a gate ledger file: its
// ID, state (for example unchecked or evidence_pending), and title.
type GateLedgerStatus struct {
	ID    string
	State string
	Title string
}

// GateLedgerFileReadiness turns one ledger file into readiness output. A
// complete ledger yields nothing; otherwise it returns a gates_incomplete
// gate naming relativePath and one warning per gate that is unchecked or
// still waiting for evidence.
func GateLedgerFileReadiness(relativePath string, complete bool, gates []GateLedgerStatus) ([]string, []string) {
	if complete {
		return nil, nil
	}
	warnings := []string{}
	for _, gate := range gates {
		if gate.State == "unchecked" || gate.State == "evidence_pending" {
			warnings = append(warnings, fmt.Sprintf("gate %s (%s): %s", gate.ID, gate.State, gate.Title))
		}
	}
	return []string{"gates_incomplete:" + relativePath}, warnings
}

// GateLedgerRelativePath returns file relative to root with forward slashes.
// It returns file unchanged when nothing remains after trimming the root.
func GateLedgerRelativePath(root, file string) string {
	relative := strings.TrimPrefix(file, root)
	relative = strings.TrimPrefix(relative, string(filepath.Separator))
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		return file
	}
	return filepath.ToSlash(relative)
}
