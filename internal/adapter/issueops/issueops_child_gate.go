package issueops

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func issueOpsChildPRGateMissing(stateRoot string, parent issueops.IssueOpsRecord) ([]string, []string) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, _, err := issueOpsChildStatusWithoutParentLock(stateRoot, parent)
	if err != nil {
		return []string{"children_complete"}, []string{"failed to scan delegated children: " + err.Error()}
	}
	return issueopsdomain.ChildPRGateMissing(status.Children), nil
}

func issueOpsActiveChildIDs(stateRoot string, parent issueops.IssueOpsRecord) ([]string, error) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return nil, nil
	}
	status, _, err := issueOpsChildStatusWithoutParentLock(stateRoot, parent)
	if err != nil {
		return nil, fmt.Errorf("children_active scan failed: %w", err)
	}
	return issueopsdomain.ActiveChildIDs(status.Children), nil
}

func issueOpsChildStatusWithoutParentLock(stateRoot string, parent issueops.IssueOpsRecord) (issueops.IssueOpsChildStatusResult, map[string]issueops.IssueOpsRecord, error) {
	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, nil, err
	}
	return buildIssueOpsChildStatus(parent, scanned), scanned, nil
}
