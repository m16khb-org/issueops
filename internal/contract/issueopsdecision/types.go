// Package issueopsdecision 패키지는 결정 기록 domain이 읽는 요청·결정 어휘를
// issueops 공용 계약의 별칭으로 노출한다.
package issueopsdecision

import issueopscontract "issueops/internal/contract/issueops"

// Request 타입은 결정 기록 요청(DecisionRecordRequest)의 별칭이다.
// domain/issueopsdecision은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 어휘를 여기서 이 이름으로 둔다.
type Request = issueopscontract.DecisionRecordRequest

// Decision 타입은 기록된 결정(issueopscontract.Decision)의 별칭이며 Request와 같은 이유로 여기서 노출한다.
type Decision = issueopscontract.Decision
