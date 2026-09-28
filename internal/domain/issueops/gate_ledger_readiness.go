package issueops

import (
	"fmt"
	"strings"
)

type GateLedgerStatus struct {
	ID    string
	State string
	Title string
}

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

func GateLedgerRelativePath(root, file string) string {
	relative := strings.TrimPrefix(file, root)
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		return file
	}
	return relative
}
