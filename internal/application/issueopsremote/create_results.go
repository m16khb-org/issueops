package issueopsremote

import (
	reportcontract "issueops/internal/contract/artifactreadability"
	executionissue "issueops/internal/contract/executionissue"
	"issueops/internal/port"
)

// IssueCreateResult 타입은 이슈 생성 결과에 본문 가독성 보고서를 더한 값이다.
type IssueCreateResult struct {
	port.IssueProviderCreateIssueResult
	Readability reportcontract.Report `json:"readability"`
}

// ChildCreateResult 타입은 자식 이슈 생성 결과에 가독성 보고서, 작업 ID, 복구 명령을 더한 값이다.
type ChildCreateResult struct {
	port.IssueProviderCreateChildResult
	Readability     reportcontract.Report `json:"readability"`
	OperationID     string                `json:"operation_id,omitempty"`
	RecoveryCommand string                `json:"recovery_command,omitempty"`
}

// PublicationResult 타입은 PR/MR 생성 결과에 가독성 보고서와 경고를 더한 값이다.
type PublicationResult struct {
	executionissue.IssueProviderCreatePullRequestResult
	Readability reportcontract.Report `json:"readability"`
	Warnings    []string              `json:"warnings,omitempty"`
}
