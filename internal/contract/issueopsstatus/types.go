package issueopsstatus

import issueopscontract "issueops/internal/contract/issueops"

// domain/issueopsstatus은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.IssueOpsRecord
type Phase = issueopscontract.IssueOpsPhase
type Readiness = issueopscontract.IssueOpsReadiness
type PhaseLedger = issueopscontract.IssueOpsPhaseLedger
type PhaseLedgerEntry = issueopscontract.IssueOpsPhaseLedgerEntry

const (
	PhaseProblem             = issueopscontract.IssueOpsPhaseProblem
	PhaseGrill               = issueopscontract.IssueOpsPhaseGrill
	PhasePlan                = issueopscontract.IssueOpsPhasePlan
	PhaseCompatibilityReview = issueopscontract.IssueOpsPhaseCompatibilityReview
	PhaseImplement           = issueopscontract.IssueOpsPhaseImplement
	PhaseAISlopClean         = issueopscontract.IssueOpsPhaseAISlopClean
	PhaseFeedback            = issueopscontract.IssueOpsPhaseFeedback
	PhasePR                  = issueopscontract.IssueOpsPhasePR
	PhaseDone                = issueopscontract.IssueOpsPhaseDone
)
