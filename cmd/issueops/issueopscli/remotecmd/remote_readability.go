package remotecmd

import (
	reportcontract "issueops/internal/contract/artifactreadability"
	port "issueops/internal/port"
)

type reflectCompletionResponse struct {
	port.IssueProviderUpdateIssueBodySectionResult
	Readability reportcontract.Report `json:"readability"`
}
