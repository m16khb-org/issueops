// Package issueopsstatus 패키지는 issueopsstatus capability의 계약을 소유한다.
// domain은 같은 capability의 계약만 import할 수 있어서, 공용 IssueOps 어휘(레코드,
// 단계, 준비 상태, 단계 원장)를 여기서 별칭으로 다시 노출한다.
package issueopsstatus

import issueopscontract "issueops/internal/contract/issueops"

// Record 타입은 domain/issueopsstatus가 읽는 공용 IssueOps 레코드의 별칭이다.
type Record = issueopscontract.Record

// Phase 타입은 IssueOps 주기 단계의 별칭이다.
type Phase = issueopscontract.Phase

// Readiness 타입은 PR 준비 상태 판정의 별칭이다.
type Readiness = issueopscontract.Readiness

// PhaseLedger 타입은 단계 진입 기록 원장의 별칭이다.
type PhaseLedger = issueopscontract.PhaseLedger

// PhaseLedgerEntry 타입은 PhaseLedger의 항목 하나의 별칭이다.
type PhaseLedgerEntry = issueopscontract.PhaseLedgerEntry

// Phase 상수들은 IssueOps 주기의 단계 이름이며, 공용 IssueOps 계약의 값을 그대로
// 가리킨다. 문제 정의(PhaseProblem)에서 시작해 PhaseDone으로 끝난다.
const (
	PhaseProblem             = issueopscontract.PhaseProblem
	PhaseGrill               = issueopscontract.PhaseGrill
	PhasePlan                = issueopscontract.PhasePlan
	PhaseCompatibilityReview = issueopscontract.PhaseCompatibilityReview
	PhaseImplement           = issueopscontract.PhaseImplement
	PhaseAISlopClean         = issueopscontract.PhaseAISlopClean
	PhaseFeedback            = issueopscontract.PhaseFeedback
	PhasePR                  = issueopscontract.PhasePR
	PhaseDone                = issueopscontract.PhaseDone
)
