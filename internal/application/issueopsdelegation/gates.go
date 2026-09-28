package issueopsdelegation

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

type ChildGates struct {
	Scan func(string) ([]issueops.IssueOpsRecord, error)
}

func (g ChildGates) PRMissing(stateRoot string, parent issueops.IssueOpsRecord) ([]string, []string) {
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

func (g ChildGates) ActiveIDs(stateRoot string, parent issueops.IssueOpsRecord) ([]string, error) {
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

func (g ChildGates) status(stateRoot string, parent issueops.IssueOpsRecord) (issueops.IssueOpsChildStatusResult, error) {
	records, err := g.Scan(stateRoot)
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	return issueopsdomain.BuildChildStatus(parent, issueopsdomain.SelectChildren(parent, records)), nil
}
