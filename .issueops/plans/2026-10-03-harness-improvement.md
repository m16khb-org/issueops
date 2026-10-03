# 하네스 동시성·요청 경로 개선 계획

## 목표와 범위

하네스의 성능, 중복, 품질, 동시성을 조사하고 검증 가능한 개선을 구현한다.
공개 CLI/MCP DTO와 상태 schema는 유지한다. 커밋·푸시·설치는 하지 않는다.
모델은 GPT-6 Astra, GPT-6.1 Sol, GPT-6 Luna로 한정하며 effort는 난도에 맞춘다.

Repo grounding: `mcpcli/mcp_sdk_server.go`, `mcpcli/mcp_tools.go`,
`internal/adapter/outbound/sqlstore/sqlstore.go`, `internal/application/state/`,
`internal/domain/projectdoc/route.go`를 읽었다. 조사 원문은
`../research/2026-10-03-harness-improvement/`에 있다.

Decision-complete plan: 아래 네 파일 소유권으로 구현을 분리한다. 구현 노드가
회귀 테스트와 변경 전후 측정을 함께 소유하고, 모든 변경 뒤 독립 검증을 수행한다.

Assumptions/defaults: 외부 서비스나 사용자 상태가 아닌 임시 디렉터리와 loopback
서버로 재현한다. 이미 동작하는 root-wide 직렬화와 workspace별 policy reload는
유지한다. 조사 교차 검증에서 반례가 발견되면 구현 전에 해당 항목을 수정한다.

Unresolved questions: 사용자 결정이 필요한 차단 질문은 없다.

Acceptance criteria: 네 항목의 focused 검증, 실제 CLI/MCP 사용 증거, 전체
self-verify와 누락된 vet/race 검증, 독립 리뷰를 모두 만족한다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md`: 안전·정확성을 성능보다 우선한다.
- `.issueops/architecture/runtime.md`: span은 root 단위 cross-process 배제이며,
  callback 전체를 하나의 data transaction으로 바꾸지 않는다.
- `.issueops/testing/concurrency-and-race.md`: sleep으로 순서를 추정하지 않는다.
- `.issueops/testing/self-verification.md`: 최종 증거는 동일 상태의 완전한
  battery로 만든다. self-verify가 이미 수행한 성공 검사를 중복하지 않는다.
- 기존 사용자 변경은 없었으며 이후 다른 작업의 변경을 덮어쓰지 않는다.

## 태스크와 의존성

| ID | 작업 | 파일 소유권 | 선행 |
|---|---|---|---|
| T1 | 취소 뒤 callback 종료까지 span 잠금 유지 | `internal/adapter/outbound/sqlstore/` | 조사 검증 |
| T2 | State 삭제·보존정책 직렬화와 취소 진입 차단 | `internal/application/state/`, `internal/adapter/outbound/state/` 테스트 | 조사 검증 |
| T3 | MCP handler별 input schema 준비 | `cmd/issueops/mcpcli/` | 조사 검증 |
| T4 | 문서 라우팅의 반복 토큰화·중복 집합 생성 제거 | `internal/domain/projectdoc/` | 조사 검증 |
| V1 | 변경 통합 검증과 실제 사용 | 테스트 실행·증거 파일만 | T1–T4 |
| V2 | 독립 코드·계획·증거 리뷰 | 리뷰 보고서만 | V1 |

T1–T4는 수정 경로가 겹치지 않는다. 조사·구현·검증은 각각 별도 DAG로 실행한다.
코드 수정 뒤 실패가 발생하면 해당 소유권 안에서 수정하고 관련 검사를 다시 수행한다.

## T1. 취소와 잠금 수명 분리

문제: `BeginTx(ctx)`는 요청 취소 시 SQL transaction을 자동 rollback한다.
callback이 아직 실행 중이어도 다른 프로세스가 span 잠금을 획득할 수 있다.

선택: 잠금 transaction의 수명만 callback 소유권에 묶는다. 대기·재시도와
callback·data mutation에는 원래 요청의 취소를 적용한다. 획득 직전·직후 취소를
확인하고 모든 성공·오류·panic 경로에서 잠금을 해제한다.

- Astra가 담당한다. transaction 수명과 cross-process 직렬화 불변식을 함께
  검토해야 하므로 high를 사용한다.
- 서로 다른 실제 DB handle을 사용하는 테스트에서 holder 취소 뒤 callback이
  명시적으로 종료할 때까지 contender가 typed busy를 관측해야 한다.
- callback 해제 뒤 contender가 성공하고, 취소된 대기자는 callback을 실행하지 않는다.
- 기존 pre-commit 취소, post-commit 보존, panic 및 process-kill 복구를 유지한다.
- `go test -race ./internal/adapter/outbound/sqlstore -count=1`
- 기존 contention benchmark를 실행하되 이 수정은 속도 향상이 아닌 정확성 개선으로 보고한다.

## T2. State 삭제·보존정책과 취소

문제: `Delete`가 writer span을 우회한다. prune의 목록 선택과 삭제 사이에
writer가 key를 갱신하면 새 checkpoint를 오래된 선택 결과로 지울 수 있다.
이미 취소된 요청도 `OpenStore`를 호출해 파일을 만들거나 scan할 수 있다.

선택:

1. standalone Delete도 root span을 획득한다.
2. confirmed prune은 목록 선택 전부터 삭제 종료까지 같은 span을 유지한다.
   내부 삭제는 이미 열린 store를 사용하며 Delete를 재호출해 중첩 잠금을 만들지 않는다.
3. context를 받는 진입점은 기존 pure validation 뒤 첫 저장소 접근 전에
   `ctx.Err()`를 확인한다. dry-run prune도 포함한다.
4. prefix·age·count·dry-run 결과와 per-delete transaction 의미를 보존한다.

- Luna가 담당한다. 기존 패턴이 있고 변경 범위가 두 계층 안에 있으므로 high를 사용한다.
- channel barrier로 갱신 전후 두 순서를 고정하고 실제 sqlstore 통합 테스트를 둔다.
- 사전 취소 시 OpenStore·transform·callback 호출 수가 0이고 임시 root가 생성되지 않는다.
- `go test -race ./internal/application/state ./internal/adapter/outbound/state -count=1`

## T3. MCP input schema 준비

문제: handler가 이미 특정 도구에 고정됐는데 요청마다 catalog를 선형 탐색하고
`ClosedProjection`으로 같은 중첩 map을 복제한다.

선택: handler 생성 시 해당 schema의 closed projection을 준비한다. 요청마다
입력 검증은 계속 수행하고 protocol error 코드와 effect 이전 거부를 유지한다.
package-global cache와 authority cache는 만들지 않는다.

- Astra 또는 Luna가 담당한다. prepared schema는 handler-local read-only 값이다.
- 동일 handler benchmark를 먼저 추가하고 변경 전후 각각 5회 측정한다.
- nested unknown field, invalid schema, unknown tool, 서버별 catalog 격리와
  병렬 호출을 검증한다. 기존 schema·tools/list 결과는 바꾸지 않는다.
- 실제 loopback HTTP 및 stdio에서 정상·오류 호출을 확인한다.
- `go test ./cmd/issueops/mcpcli -run '^$' -bench BenchmarkSDKToolHandler -benchmem -count=5`
- `go test -race ./cmd/issueops/mcpcli -count=1`

## T4. 문서 라우팅의 반복 계산 제거

문제: `hasTaskToken`이 동일 task를 반복해서 FieldsFunc로 분해하고,
`appendRouteDocsUnique`가 add마다 중복 집합을 다시 만든다.

선택: 호출 안에서 토큰 집합과 문서 중복 집합을 각각 한 번만 만든다.
기존 순서, 첫 reason 우선, Unicode 단어 경계와 fallback 라우팅은 유지한다.
전역 mutable cache나 새로운 외부 dependency는 추가하지 않는다.

- Luna가 담당한다. 순수 함수의 측정 가능한 국소 변경으로 제한한다.
- 기존 테스트와 Unicode·구두점·복합 task·중복 제거·순서 사례를 대조한다.
- 동일 benchmark를 변경 전후 각각 5회 실행해 ns/op, B/op, allocs/op를 비교한다.
- `go test ./internal/domain/projectdoc -run '^$' -bench BenchmarkRouteDocsForTask -benchmem -count=5`
- `go test -race ./internal/domain/projectdoc ./internal/application/projectdocs -count=1`

## 검증과 종료

사용자는 공급자 인증 실패 뒤 추가 에이전트 대신 현재 세션에서 직접 완료하도록
선택했다. 남은 구현과 최종 리뷰는 메인 에이전트가 수행하며, 독립 모델 리뷰는
완료로 표시하지 않는다. 아래의 원래 모델 배정은 계획 이력으로 남긴다.

1. 메인 에이전트가 실제 diff와 각 노드의 증거를 읽고 수정 범위를 대조한다.
2. 변경 파일 LSP 진단과 `git diff --check`, 새 파일을 포함한 gofmt 검사를 수행한다.
3. 최신 binary로 self-verify를 실행한다. Python은 준비한 격리 환경을 사용한다.
4. 같은 bundle에 `go vet ./...`, `go test -race ./... -count=1`의 실제 실행 증거가
   없으면 별도로 수행한다. 설치된 golangci-lint의 저장소 규칙도 확인한다.
5. Sol 6.1이 독립 리뷰로 correctness·scope·검증 누락을 확인한다.
6. 최종 보고서에 측정 범위, 개선 수치, 실패·미검증·보류 항목을 구분한다.
7. 임시 `.omo/omo.jsonc`와 작업 초안을 제거한다. 커밋·푸시하지 않는다.

## 채택하지 않는 후보

- root lock을 key lock으로 바꾸는 재설계: cross-record·cross-process 계약을 변경한다.
- pprof·runtime 통계 API: 현재 병목을 고치는 데 필요한 변경이 아니다.
- policy override cache: workspace별 최신 파일을 매번 읽는 계약과 충돌한다.
- SQLite 버전·DSN 문자열을 고정하는 테스트: 실제 동작 회귀를 증명하지 못한다.
- built-in policy catalog cache: mutable 반환값의 복제 비용까지 측정한 뒤 별도 판단한다.
