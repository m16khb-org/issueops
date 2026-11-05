// Package issueopsdelegation is the application layer for delegated child
// cycles: starting a child under a parent, reporting and repairing the
// parent's child index, recording accept/reject/drop verdicts, and exposing
// the child-derived completion gates.
package issueopsdelegation

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ChildGates derives parent-cycle gate facts from its delegated children,
// scanning records through Scan.
type ChildGates struct {
	Scan func(string) ([]issueops.Record, error)
}

// PRMissing returns the gate names and messages that block the parent's PR
// because children are incomplete. An empty stateRoot yields nothing; a scan
// failure yields a single children_complete gate carrying the error.
func (g ChildGates) PRMissing(stateRoot string, parent issueops.Record) ([]string, []string) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, err := g.status(stateRoot, parent)
	if err != nil {
		return []string{"children_complete"}, []string{"failed to scan delegated children: " + err.Error()}
	}
	return issueopsdomain.ChildPRGateMissing(status.Children), nil
}

// ActiveIDs returns the IDs of the parent's children that are still active.
// An empty stateRoot yields nothing; a scan failure is returned as an error.
func (g ChildGates) ActiveIDs(stateRoot string, parent issueops.Record) ([]string, error) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, err := g.status(stateRoot, parent)
	if err != nil {
		return nil, fmt.Errorf("children_active scan failed: %w", err)
	}
	return issueopsdomain.ActiveChildIDs(status.Children), nil
}

func (g ChildGates) status(stateRoot string, parent issueops.Record) (issueops.ChildStatusResult, error) {
	records, err := g.Scan(stateRoot)
	if err != nil {
		return issueops.ChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	return issueopsdomain.BuildChildStatus(parent, issueopsdomain.SelectChildren(parent, records)), nil
}
