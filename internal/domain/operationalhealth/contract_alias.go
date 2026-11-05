package operationalhealth

import operationalhealthcontract "issueops/internal/contract/operationalhealth"

// domain은 같은 capability 계약만 import할 수 있어서 domain/issueopsorphancleanup이
// Snapshot·Options와 Authority 상수를 이 패키지 이름으로 받는다.
type (
	// Snapshot 타입은 운영 건강 신호를 담는 계약 DTO다.
	Snapshot = operationalhealthcontract.Snapshot
	// Options 타입은 운영 건강 판정의 평가 시각과 보존할 cycle 목록을 담는 계약 DTO다.
	Options = operationalhealthcontract.Options
)

// AuthorityLive, AuthorityPreserved, AuthorityUnknown은 cycle 권한 판정 결과의
// 계약 상수를 이 패키지 이름으로 다시 노출한다.
const (
	AuthorityLive      = operationalhealthcontract.AuthorityLive
	AuthorityPreserved = operationalhealthcontract.AuthorityPreserved
	AuthorityUnknown   = operationalhealthcontract.AuthorityUnknown
)
