// Package issueopsrouting 패키지는 skill routing 추적과 채점에 쓰이는 공용 타입 별칭을 소유하는 contract 계층 패키지다.
package issueopsrouting

import (
	benchmark "issueops/internal/contract/issueopsbenchmark"

	issueopscontract "issueops/internal/contract/issueops"
)

// Record 타입은 domain/issueopsrouting이 읽는 공용 사이클 레코드 어휘다.
// domain은 같은 capability 계약만 import할 수 있어서, 이 contract 패키지가
// 그 어휘를 이 이름으로 둔다.
type Record = issueopscontract.Record

// Entry 타입은 레코드에 기록되는 skill routing 한 건(phase와 skill의 짝)이다.
type Entry = issueopscontract.SkillRoutingEntry

// Expected 타입은 benchmark가 기대하는 phase와 skill의 짝이다.
type Expected = benchmark.SkillRouting

// Result 타입은 기대한 routing이 관측되었는지 채점한 결과다.
type Result = issueopscontract.RoutingFidelityResult
