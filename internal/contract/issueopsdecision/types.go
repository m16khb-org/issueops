package issueopsdecision

import issueopscontract "issueops/internal/contract/issueops"

// domain/issueopsdecision은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 어휘를 여기서 이 이름으로 둔다.
type Request = issueopscontract.IssueOpsDecisionRecordRequest
type Decision = issueopscontract.IssueOpsDecision
