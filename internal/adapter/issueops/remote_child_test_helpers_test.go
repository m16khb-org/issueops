package issueops

import (
	"fmt"
	"issueops/internal/port"
)

// CreateRemoteChild는 provider가 구성되어 있을 때만 원격 child work item을 만든다.
func CreateRemoteChild(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error) {
	if prov == nil {
		return port.IssueProviderCreateChildResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateChild(req)
}
