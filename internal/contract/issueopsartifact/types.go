// Package issueopsartifact 패키지는 IssueOps artifact 단계 기록의 계약을
// 소유한다. domain/issueopsartifact가 읽는 공용 IssueOps 어휘의 별칭과 봉인
// 오류 DTO를 담는 contract 계층 패키지다.
package issueopsartifact

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsleasecontract "issueops/internal/contract/issueopslease"
)

// Record 타입은 공용 IssueOps 기록의 별칭이다. domain/issueopsartifact는 이 contract 패키지만
// import할 수 있어서(domain은 같은 capability 계약만 import) 그 domain이 읽는
// 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record

// Staged 타입은 execution prepare로 봉인되기 전에 단계별로 저장해 둔 owner
// artifact를 이름과 내용으로 묶은 맵이다.
type Staged = map[string]string

// MaxBytes는 owner artifact 한 건의 최대 크기(바이트)이고, PhaseDone은 완료
// phase 이름이다. 둘 다 공용 계약 값을 이 패키지 이름으로 다시 노출한다.
const (
	MaxBytes  = issueopsleasecontract.OwnerArtifactMaxBytes
	PhaseDone = issueopscontract.PhaseDone
)

// SealedError 타입은 execution prepare 이후 artifact를 더 쓸 수 없다는 오류다.
// ID는 봉인된 cycle ID이며 IssueOpsErrorFields로 다음 명령을 안내한다.
type SealedError struct {
	ID string
}

func (err *SealedError) Error() string {
	return "artifacts are sealed after execution prepare"
}

// IssueOpsErrorFields 메서드는 오류 코드 artifact_stage_sealed와 상태를
// 확인하는 next_command를 JSON 오류 필드로 돌려준다.
func (err *SealedError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":         "artifact_stage_sealed",
		"next_command": "issueops execution status --id " + err.ID + " --json",
	}
}
