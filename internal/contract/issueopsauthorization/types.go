// Package issueopsauthorization 패키지는 IssueOps lease 권한 판정에 쓰는 계약
// 어휘를 소유한다. domain/issueopsauthorization이 읽는 공용 기록, 행위자,
// lease 상태 값을 별칭으로 노출하는 contract 계층 패키지다.
package issueopsauthorization

import issueopscontract "issueops/internal/contract/issueops"

// Record 타입은 공용 IssueOps 기록의 별칭이다. domain/issueopsauthorization는 이 contract 패키지만
// import할 수 있어서(domain은 같은 capability 계약만 import) 그 domain이 읽는
// 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record

// Actor 타입은 명령을 실행하는 행위자(host, session 등)를 가리키는 공용 IssueOps Actor의 별칭이다.
type Actor = issueopscontract.Actor

// VerifiedActor 타입은 신원 검증을 마친 행위자를 가리키는 공용 IssueOps VerifiedActor의 별칭이다.
type VerifiedActor = issueopscontract.VerifiedActor

// LeaseStatusActive 상수는 현재 보유자가 있는 활성 lease 상태 값이다.
const LeaseStatusActive = issueopscontract.LeaseStatusActive

// LeaseStatusReleased 상수는 보유자가 해제한 lease 상태 값이다.
const LeaseStatusReleased = issueopscontract.LeaseStatusReleased
