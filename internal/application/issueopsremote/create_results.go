package issueopsremote

import (
	reportcontract "issueops/internal/contract/artifactreadability"
	"issueops/internal/port"
)

type IssueCreateResult struct {
	port.IssueProviderCreateIssueResult
	Readability reportcontract.Report `json:"readability"`
}
type ChildCreateResult struct {
	port.IssueProviderCreateChildResult
	Readability reportcontract.Report `json:"readability"`
}
type PublicationResult struct {
	port.IssueProviderCreatePullRequestResult
	Readability reportcontract.Report `json:"readability"`
}
