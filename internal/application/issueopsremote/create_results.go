package issueopsremote

import (
	reportcontract "issueops/internal/contract/artifactreadability"
	executionissue "issueops/internal/contract/executionissue"
	"issueops/internal/port"
)

type IssueCreateResult struct {
	port.IssueProviderCreateIssueResult
	Readability reportcontract.Report `json:"readability"`
}
type ChildCreateResult struct {
	port.IssueProviderCreateChildResult
	Readability     reportcontract.Report `json:"readability"`
	OperationID     string                `json:"operation_id,omitempty"`
	RecoveryCommand string                `json:"recovery_command,omitempty"`
}
type PublicationResult struct {
	executionissue.IssueProviderCreatePullRequestResult
	Readability reportcontract.Report `json:"readability"`
}
