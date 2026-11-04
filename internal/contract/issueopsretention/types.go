// Package issueopsretention 패키지는 오래된 IssueOps cycle 기록을 정리하는
// retention 작업의 결과 DTO를 소유한다. 삭제 대상, 유지 대상, 읽기·삭제 실패
// 진단을 담는 contract 계층 패키지다.
package issueopsretention

import issueopscontract "issueops/internal/contract/issueops"

// Record 타입은 공용 IssueOps 기록의 별칭이다. domain/issueopsretention는 이 contract 패키지만
// import할 수 있어서(domain은 같은 capability 계약만 import) 그 domain이 읽는
// 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record

// UnreadableRecordDiagnostic 타입은 읽지 못해 판단에서 빠진 기록 한 건의 ID와 원인 코드다.
type UnreadableRecordDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

// DeleteFailureDiagnostic 타입은 삭제하지 못한 기록 한 건의 ID와 원인 코드다.
type DeleteFailureDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

// Result 타입은 retention 실행 결과다. Pruned는 정리 대상으로 판정한 기록 ID이고
// Kept는 남긴 기록 ID다. Cutoff는 정리 여부를 가르는 기준 시각이며, DryRun이면
// 실제 삭제 없이 판정만 담는다.
type Result struct {
	OK                    bool                         `json:"ok"`
	StateRoot             string                       `json:"state_root"`
	MaxAge                string                       `json:"max_age"`
	Cutoff                string                       `json:"cutoff"`
	DryRun                bool                         `json:"dry_run"`
	Pruned                []string                     `json:"pruned"`
	Kept                  []string                     `json:"kept"`
	ReadErrors            int                          `json:"read_errors"`
	Unreadable            []string                     `json:"unreadable"`
	UnreadableDiagnostics []UnreadableRecordDiagnostic `json:"unreadable_diagnostics"`
	DeleteErrors          int                          `json:"delete_errors"`
	Failed                []string                     `json:"failed"`
	DeleteDiagnostics     []DeleteFailureDiagnostic    `json:"delete_diagnostics"`
	Error                 string                       `json:"error,omitempty"`
}

// LeaseStatusReleased, PhaseDone, IssueCreateCompleted는 정리 가능 여부 판정에
// 쓰는 공용 계약 값을 이 패키지 이름으로 다시 노출한다.
const (
	LeaseStatusReleased  = issueopscontract.LeaseStatusReleased
	PhaseDone            = issueopscontract.PhaseDone
	IssueCreateCompleted = issueopscontract.IssueCreateIntentCompleted
)
