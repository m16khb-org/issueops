package issueopsstatus

import issueopscontract "issueops/internal/contract/issueops"

// domain/issueopsstatus은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record
type Phase = issueopscontract.Phase
type Readiness = issueopscontract.Readiness
type PhaseLedger = issueopscontract.PhaseLedger
type PhaseLedgerEntry = issueopscontract.PhaseLedgerEntry

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
