# 남은 효율성 후보 14개 구현 계획

## 요청과 완료 기준

사용자가 `docs/repo-efficiency-audit-2026-10-01.md`의 나머지 후보 14개를 모두
구현하도록 승인했다. 기존 출력 캡처 개선과 검증 결과는 보존한다.
각 후보에 실제 변경, 기존 계약 보존, 회귀 검사와 효과 근거가 있어야 완료다.
측정되지 않은 latency 개선을 주장하거나 안전 검사를 생략해서 통과시키지 않는다.

기준 HEAD는 `82d4c647`이고 작업 시작 시 이전 개선의 미커밋 변경 8개가 있었다.
그중 출력 캡처 production/test 5개는 보존 대상이다. 이전 전체 self-verify는
27/27, 최저 점수 100, macOS/Linux lint 통과 상태였다. 현재 작업 완료에는
새 변경 전체에 대한 새 검증 실행이 필요하다.

## 적용되는 결정과 주의사항

- `CONSTITUTION.md`: 안전·정확성 우선, 측정·실행 근거 없는 성능 주장 금지.
- `CONVENTIONS.md`, `conventions/go-and-packages.md`: domain/application/port
  경계를 유지하고 concrete adapter 공통 기능은 composition root에서 주입한다.
- `TESTING.md`, `testing/self-verification.md`: 실패 뒤 전체 배터리를 처음부터
  실행하며, 성공한 같은 실행의 race/vet/build/golden만 재사용한다.
- `SUB_AGENT_PATTERNS.md`: `parallel-independent-research`로 변경 경계를 찾고,
  `task-fan-out-coordination`으로 독립 파일 범위를 구현한다. 이득은 병렬 속도와
  문맥 분리이고, 비용은 통합·부분 컴파일 충돌이다. 공유 파일은 직렬화하고,
  작은 후속 수정과 전체 검증은 메인이 맡는다.
- 요청 모델 배분: 메인 Astra, 조회·독립 검증 GPT-6 Luna, 구현 GPT-6.1 Sol.
- 커밋·push·사용자 홈 설치·원격 쓰기는 하지 않는다.

## 구현 항목

### 1. 명시적 next ID의 직접 조회

`application/issueopsnext`와 inventory outbound/wiring에서 명시 ID와 환경 ID는
strict read로 고른다. 저장소 정규화 경계를 확인하여 다른 repo ID를 받아들이지 않는다.
실행이 있는 record는 `ListCycles`나 sibling scan 없이 판정한다.
실행 전 record의 branch/root가 있을 때만 2번의 strict stream으로 충돌을 조회한다.
이 안전 검사까지 O(1)이라고 주장하지 않는다. raw JSON SQL 필터는 duplicate key와
정규화 전 경로 때문에 사용하지 않는다.

검증: 명시/env 우선순위, 없는/손상/외부 repo ID, 실제 root conflict를 보존한다.
일반 명시 ID에서 list 호출 0회, strict read 1회를 검증한다.

### 2. repo 필터 조회의 streaming과 정규화 재사용

`outbound/sqlstore`, `outbound/issueopsrecord`, inventory application/adapter에
row → strict Decode → filter/project 흐름을 둔다. 전체 raw/decoded slice 대신
일치하는 projection과 모든 진단만 보관한다. 모든 row의 strict 검증은 계속 수행한다.
요청 repo의 정규화 결과를 요청 내 cache에 넣어 같은 path를 다시 Git으로 조회하지 않는다.
기존 `ScannedRecords`, `ReadErrors`, `UnreadableIDs`, `Diagnostics`는 유지한다.

검증: 100/1,000/10,000 row의 filtered inventory에서 메모리·호출 수를 비교하고,
관련 없는 손상 row도 진단되는지 확인한다. schema/index/migration은 추가하지 않는다.

### 3. readiness와 cleanup의 요청 내 Git 관측 공유

각 public readiness operation 안에서 exact root/argv별 exit/stdout/stderr를 공유한다.
`newCycleReadiness`는 MCP 요청보다 오래 살 수 있으므로 constructor cache는 금지한다.
standalone PR과 observePR의 내부 PR이 같은 operation의 관측을 공유하도록 구성한다.
fetch 이후 반드시 다시 읽어야 하는 사실과 다음 요청의 관측은 재사용하지 않는다.

검증: branch/status 중복 호출 감소, 실패 tuple 보존, 같은 Readiness 인스턴스의
연속 호출에서 변경된 Git 상태 반영, prefetch/post-fetch 순서를 확인한다.

### 4. lsof 경로 정규화 중복 제거

`adapter/issueops/execution_process.go`의 parser 안에서 eligible raw absolute
path별 containment 결과를 재사용한다. FD/access, symlink, deleted suffix, root
경계와 상대 경로 거부는 유지한다. lsof selector·timeout은 바꾸지 않는다.

검증: resolution 호출 수가 unique eligible path 수와 같고 PID/점유 결과가 같다.

### 5. cleanup ancestry 요청 내 재사용

`cleanup_workspace_processes.go`의 한 ps snapshot에서 PID별 ancestry와 오류를
재사용한다. requester·occupant는 오류 시 실패, descendant 집계는 기존처럼 skip한다.
전역 PID cache는 금지한다.

검증: PID당 최대 1회 탐색, missing/cyclic chain, 128-hop 제한, collateral 결과 보존.

### 6. sync-base 확정 blocker 이후 원격 조회 생략

`execution_sync_base.go`에서 record/authority의 확정 blocker가 있으면 필요한 local
진단까지 수집하고 `ls-remote`·fetch·원격 파생 판정 전에 반환한다. 관측하지 않은
원격 부재나 fetch 실패를 만들어 넣지 않는다. 기존 execution/worktree 부재의
부분 진단 조기 반환 원칙을 확장하며 거부 결과가 원격 진단 전부를 보장하지 않음을 기록한다.

검증: blocker에서 network 호출 0회, 알려진 gate 유지, 성공 경로 fetch/merge 순서 보존.

### 7. 상태 읽기의 validation 책임 통합

`issueops_state.go`의 strict decoder 검증은 유지하고 두 reader의 성공 후 재검증만
제거한다. write·mutation fence는 건드리지 않는다.

검증: 유효 record 결과 동일, schema/ID/unknown/trailing/invariant 거부 matrix 동일.

### 8. operational-health 요청별 인덱스

한 Classify 안에서 count+unique value 인덱스를 만든다. worktree, terminal handle/PTY,
task의 run/id와 legacy task ID, dispatch 및 전체 lease-holder tuple을 구분한다.
중복을 overwrite하지 않고 양방향 역색인 검사를 유지한다.

검증: 기존 전체 정렬 Finding과 동일, duplicate/missing/mismatch/legacy case 보존,
큰 snapshot에서 반복 선형 lookup이 사라짐을 benchmark와 구조로 확인한다.

### 9. architecture source/AST inventory 공유

test-process 내 root별 불변 snapshot을 공유하고 declaration과 lease-codec 확인에서
같은 AST를 재사용한다. mutable slice는 복사하고 fixture source와 오류를 숨기지 않는다.
의도적으로 새로운 `go list`를 실행하는 byte-stability 검사는 그대로 둔다.

검증: 여러 consumer에서 walk 1회·source당 parse 1회, root 분리·결과 안정성·오류 전파.

### 10. 기존 스킬 Python 테스트 자동 실행

현재 6개 스킬의 `scripts/`와 `tests/`에 테스트 파일 10개가 존재한다.
`test_*.py`와 `*_test.py`를 중복 없이 수집하고 스킬별 subprocess/CWD/import를 격리한다.
기존 root scripts suite는 정확히 한 번 실행한다. Python 3.10+ 및 출력·timeout 제한,
import/discovery/test 실패 전파를 유지한다. 빈 스킬과 discovery가 빠진 실제 테스트를
구분한다. 기존 Python script tests 단계가 이 책임을 소유한다.

검증: 실제 10개 파일 모두 실행, passing/failing/import-error/duplicate-name fixture,
CI에 별도 중복 Python 검사를 추가하지 않음.

### 11. Go test match self-test 자동 실행

Python 검사 뒤에 `bash scripts/verify-go-test-match-test.sh`를 실행하는 독립 단계를 둔다.
step order, recovery command, goal/coverage, contract version/hash의 영향을 반영한다.
전체 Go 검사의 대체 증거로 취급하지 않는다.

검증: 실제 self-test 통과, 실패 코드·출력 보존, 정확히 한 번 실행, rerun 가능.

### 12. MCP 단일 SDK dispatch

구형 exported test dispatcher와 그 facade를 제거하고 모든 in-repo 테스트를 실제
SDK CallTool 경로로 옮긴다. CLI/MCP wire가 지원 표면이며 cmd test dispatcher의
호환 shim은 두지 않는다. direct text 결과의 배열 표현 차이와 IsError 누락을
회귀 테스트로 확인하고 SDK 결과 변환을 바로잡는다.

검증: 구형 호출 참조 0개, 정확한 commit-policy Markdown, tool/protocol error 구분,
인자 검사 선행, per-server catalog·dependency 격리, resource·context 보존.

### 13. host JSON merge/readback 공통화

agy/Omo/Claude의 user MCP JSON map mechanics와 named-entry semantic readback을
installutil에 두고 dependency injection으로 호출한다. host DTO·path·mode·진단
context·evidence 순서, Omo extension/Claude hooks 및 Codex TOML은 보존한다.

검증: sibling/top-level 보존, malformed fail-closed, dry-run no-write, 권한,
각 host readback 및 기존 adapter matrix 결과 동일. 실제 설치는 하지 않는다.

### 14. SQLite cache hit의 중복 디렉터리 검사 제거

매 Open의 removed-root 전체 정리와 fresh permission/sidecar 검사 계약은 유지한다.
existing real directory는 fresh Lstat 하나로 type/mode를 확인해 불필요한
MkdirAll의 재검사를 없앤다. missing/non-directory는 기존 생성·거부 경로를 유지한다.
삭제된 requested root는 재생성 전에 기존 handle을 축출한다. TTL·watcher는 추가하지 않는다.

검증: 1/10/100 cached roots benchmark, requested/other removed root 정리,
cached permission repair, symlink/non-regular 거부와 cross-root race.
여전히 필요한 O(N) global sweep까지 제거했다고 주장하지 않는다.

## DAG와 쓰기 경계

설계: 8개 영역 조회 → 정확성/검증 적정성 검토. 메인이 계약 위험을 소스로 대조했다.

구현 A: 14·7·8·9·10·12·13의 독립 파일 범위 → focused 검증.
구현 B: 2→1, 3→4→5→6, 3→1, 10 이후 11 → focused 검증.
14→2는 SQLite 파일 충돌, 2→1은 streaming seam 공유,
10→11은 step contract 충돌 때문에 필요하다.
3·4·5·6은 같은 큰 adapter package를 검증하므로 직렬화한다.

각 worker는 담당 코드와 인접 테스트 및 간단한 결과 보고만 작성한다.
메인은 공용 DDD inventory JSON, golden, canonical 문서와 전체 검증을 소유한다.
기존 capture source/test는 worker 쓰기 범위에서 제외한다.

## 최종 검증

14개 각각의 before/after 근거, 실패를 검출하는 회귀 검사, 변경 범위 대조가 필요하다.
그 뒤 정규 collector로 책임 인벤토리를 갱신하고 관련 contract/golden을 검증한다.
최신 binary로 self-verify를 실행하여 race/vet·Python·build·CLI/MCP·QA의 실제
성공 증거를 확인하고 macOS와 Linux lint를 실행한다.
검사 사이에 소스가 바뀌거나 실패하면 관련 원인을 고친 뒤 전체 배터리를 새로 실행한다.
