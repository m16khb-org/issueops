package issueops

import (
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func scanIssueOpsChildrenForParent(stateRoot string, parent issueops.IssueOpsRecord) (map[string]issueops.IssueOpsRecord, error) {
	records, err := ScanReadableIssueOps(stateRoot)
	if err != nil {
		return nil, err
	}
	return issueopsdomain.SelectChildren(parent, records), nil
}
