package operationalhealth

import operationalhealthcontract "issueops/internal/contract/operationalhealth"

// 운영 건강 신호는 계약 DTO다. domain은 같은 capability 계약만 import할 수 있어서
// domain/issueopsorphancleanup이 Snapshot·Options와 Authority 상수를 이 패키지
// 이름으로 받는다.
type (
	Snapshot = operationalhealthcontract.Snapshot
	Options  = operationalhealthcontract.Options
)

const (
	AuthorityLive      = operationalhealthcontract.AuthorityLive
	AuthorityPreserved = operationalhealthcontract.AuthorityPreserved
	AuthorityUnknown   = operationalhealthcontract.AuthorityUnknown
)
