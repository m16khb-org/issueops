package issueopsartifact

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsleasecontract "issueops/internal/contract/issueopslease"
)

// domain/issueopsartifact은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.IssueOpsRecord
type Staged = map[string]string

const (
	MaxBytes  = issueopsleasecontract.OwnerArtifactMaxBytes
	PhaseDone = issueopscontract.IssueOpsPhaseDone
)

type SealedError struct {
	ID string
}

func (err *SealedError) Error() string {
	return "artifacts are sealed after execution prepare"
}

func (err *SealedError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":         "artifact_stage_sealed",
		"next_command": "issueops execution status --id " + err.ID + " --json",
	}
}
