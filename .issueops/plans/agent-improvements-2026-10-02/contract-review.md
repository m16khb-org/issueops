# contract.md 독립 검토

기준 HEAD `92eaa00143841f964c285dacdd41aedbeeecdf2e`. 검토 모델 export:
`PI_MODEL=claude-fable-5-1`, `PI_REASONING_LEVEL=medium`(요청은 high였으나
export 값은 medium). 입력은 `brief.md`, `contract.md`, 조사 노트 6개,
`execution-ledger.md`와 아래에 적은 현재 소스다. production 변경·설치·테스트는
하지 않았다.

## 판정: REVISE

설계 방향(native CLI 발급 capability, 단일 stateless HTTP, 세션별 proxy 없음,
lease 판정은 core 유지)은 소스와 맞고 I1-I10 모두 수용 기준과 producer/검증
owner가 있다. 다만 아래 두 항목은 그대로 구현하면 architecture fitness 테스트
실패 또는 소유권 충돌로 이어지므로 구현 전에 고쳐야 한다. 나머지는 명확화나
후속 항목이다.

## 필수 수정 (blocker)

### F1. `ValidateHolder`의 `*model.VerifiedActor` 인자는 domain import 규칙 위반

contract §2 "일반 mutation"은
`domain/issueopsauthorization.ValidateHolder(record,actor,verified *model.VerifiedActor)`
로 두고 `model`을 `contract/issueops`로 정한다. 그러나
`internal/architecture/dependency_test.go:1220-1233`의 `isAllowedDomainContract`는
domain→contract edge를 같은 capability 이름(`domain/X`→`contract/X`)과
publication/completion/preparation 예외만 허용한다.
`domain/issueopsauthorization`→`contract/issueops`는 거부된다. 현재 이 domain은
`contract/issueopsauthorization/types.go`의 alias(`Record`, `Actor`)만 import한다
(`authorization.go:1-8`).

**수정:** `contract/issueopsauthorization/types.go`에
`type VerifiedActor = issueopscontract.VerifiedActor`를 추가하고 domain signature를
`ValidateHolder(record Record, actor *Actor, verified *VerifiedActor) (bool, error)`
로 적는다. Z-foundation 범위에 이 alias 추가를 명시한다. 대안으로 dependency
테스트에 예외를 추가하는 것은 기존 guard 약화이므로 수용하지 않는다.

### F2. `MutationAuthority.Validate` signature 변경의 호출자 소유권이 비어 있다

contract는 `Validate(ctx,record,actor)`로 바꾸고 G에 "lease/completion/cycle/
reconcile/replacement/publication/sync-base/preparation"의 guard를 맡긴다.
실제 `Validate`/`NewMutationAuthority` 호출자는 다음에도 있다(grep, 테스트 제외):
`internal/application/issueopsbranch/{link.go:65,prepare.go:33,retarget.go:27,workspace_link.go:85}`,
`internal/application/issueopsdelegation/{start.go:46,status.go:55,verdict.go:61}`,
`issueopscycle/plan_link_authority.go:11`, `issueopsexecution/execution_reconcile.go:45`,
그리고 `cmd/issueops/issueopsapp/issueops_{artifact_verification,body_sync,branch_prepare,branch_retarget,child_create,child_start,link,publication,review_reflection}_wiring.go` 9개.
branch/delegation application과 9개 wiring은 G 목록에 없고, "나열하지 않은 공유
파일은 Z만" 규칙상 Z-integration(마지막)으로 가므로 G/H 단계에서 빌드가 깨진다.

**수정(둘 중 하나, 전자를 권장):**
(a) 기존 `Validate(record, actor)`와 `NewMutationAuthority(pathsMatch)`는 그대로
두고, `NewMutationAuthorityWithVerifier(pathsMatch, verifier)`와
`ValidateContext(ctx, record, actor)`를 추가한다. 기존 호출자는 native 경로의
현재 동작을 유지하고, HTTP가 지나는 경로만 G가 새 메서드로 바꾼다.
(b) signature를 바꾼다면 위 application 두 패키지와 wiring 9개를 G 범위에
명시적으로 추가하고 §5 표를 갱신한다.

## 명확화 (구현 전 한 줄씩 확정)

### F3. actor 인자와 capability 동시 전달 규칙이 두 문장에서 어긋난다

§2는 HTTP에서 "actor 관련 입력과 capability를 함께 보내면 거부"라 하고, Bind/Verify
문단은 "supplied actor가 비어 있지 않으면 검증한 identity와 일치해야 한다"고 한다.
HTTP와 stdio+capability 두 경우로 나눠 적는다: HTTP는 `authority_file`이 있을 때
`host/session_id/agent_id/session_pid/session_started_at/session_executable` 중
하나라도 있으면 `authority_invalid`; stdio는 capability와 actor를 함께 받되 일치
검사를 한다. 테스트 목록(§6 "actor 인자 혼합")에 두 경우를 각각 넣는다.

### F4. grant 저장 DB와 span 규칙을 명시한다

`sqlstore.Open(stateRoot)`는 state root 단위다(`issueopsrecord/store.go:317`,
`sqlstore.go:116`). repo-scope key의 `issueops_authority_v1` bucket을 같은 DB에
두는 것은 가능하다. 다만 `WithSpan`은 같은 dir의 중첩 span을 거부하므로
(`sqlstore.go:377-383`) `Issue`의 `Within`은 lease span 밖에서만 열고, `Bind`/
`Verify`/`BindSpan`은 절대 자체 span을 열지 않는다는 문장을 §2 "원자성"에 추가한다.
`Record.Actor`는 `model.NativeActor`이므로 `ProcessAncestry`가 저장되지 않도록
encode 시 비우고, 읽기 테스트에서 nil을 단언한다.

## 소스 대조 결과 (contract 주장 확인)

- ancestry 결합: `mcpcli/mcp_tool_issueops_execution.go:54-58`은 `os.Getpid()`
  계보를 caller actor에 붙인다. 대체 대상이 맞다.
- claim 판정: `issueopslease/claim_transaction.go:24-73`은 generation, canonical
  cwd, token SHA, holder 전이를 core에서 검사하고 token을 지운다(`:65,:72`).
  grant가 lease 소유권이 아니라는 contract 원칙과 일치한다. 두 caller의 동일 repo
  grant가 lease 권한으로 승격되지 않는다는 §6 테스트는 이 경로가 보장한다.
- action parity: `issueopsexecution/execution_api.go:31-108`의 dispatch는
  prepare/status/claim/release/replace(reseed 포함)/resume/reconcile/complete다.
  §2 "필수 action parity" 목록과 같다.
- stale generation/workspace: `claim_transaction.go:30-36`, `native_actor.go:45-50`
  (live PID+started_at+executable)로 host 재시작·PID 재사용이 거부된다.
  server 재시작은 SQLite grant가 남아 유지된다는 주장은 저장 설계와 일치한다.
- 관측 결합: `sqlstore.go:340-411` wait/hold 정의와 `:700-722,738-781`의 data
  commit 위치, `issueopsrecord/store.go:152,188-190`의 context 없는 Put/Delete가
  contract I8 서술과 같다.
- I1 `trace/decode.go:26-37` scanner.Err 미검사, I6 `preflight.go:51-54` history
  4회, I2 `selfverify/contract.go:13` v6 모두 확인했다.
- SDK v1.8.0: upstream `mcp/streamable.go`에서 `Stateless`(129-146, GET/DELETE 405는
  138과 384, legacy DELETE 처리는 445-449), `JSONResponse`(150-154),
  `PropagateRequestCancellation`(210-219; 431에서 2026 subscription listen은
  옵션과 무관하게 전파), version header 검사(347-358)를 확인했다. 로컬 module
  cache에는 v1.6.1만 있으므로(`go.mod:33`) F의 checksum 검증이 선행돼야 한다.
- host HTTP 지원: Omo 세 runtime 모두 `config-schema.js:21-27,64-69`에
  `type:"http"/url/headers`가 있다. Claude는 `claude mcp add --help`에
  `--transport http`, `--header`가 있다. **Codex는 로컬 native 바이너리가 없다**
  (`codex --version` → `codex-darwin-arm64/.../codex ENOENT`). Codex `http_headers`
  확인은 공식 문서에 의존하며, §6 real-surface 검증과 `codex mcp get issueops`는
  정상 Codex 설치가 전제다. 이를 execution-ledger에 환경 전제조건으로 적는다.
- `Observer.Inspect(root,target,home,version,skillName)`(`inspect.go:14`),
  `Tool{Name,Description,InputSchema}`(`catalog_types.go:4-8`),
  `NativeInstallRequest`(`port/install.go:5-14`),
  `resolveHandlerGroup(deps,name) func(MCPToolCall) MCPToolOutcome`
  (`mcp_sdk_server.go:160`)는 contract가 전제한 현재 모양과 같다.
- `daemon/lock.go:12`, `instance.go:31` 재사용 가능. `update/runtime.go:55`
  `StopDaemon` 옆에 service stop을 두는 위치도 맞다.

## I1-I10 완결성 점검

| ID | acceptance | producer | 검증 owner | 비고 |
|---|---|---|---|---|
| I1 | §4 complete/warnings sentinel | A | A+Z 통합 | 충족 |
| I2 | 137/112ms fixture, v7 | B | B | clock 주입 대신 runner duration 사용; note와 다르지만 결정적이고 sleep 없음 |
| I3 | 임시 HOME/receipt stale | K | K+real-surface | J 뒤 직렬, 충족 |
| I4 | SDK round-trip parity | L | L | H→L 순서 명시, 충족 |
| I5 | revision matrix | F | F+H | 충족 |
| I6 | 4→1, 2→1 recording runner | C | C | 충족 |
| I7 | 200/1200 결정성, 64×8KiB | D | D | 1024 raw cap 폐기는 결정적 선택을 만족 |
| I8 | fake clock 단계, benchmark | E | E | WithRecordGuard/Apply 경계 확정 |
| I9 | 세 host fixture, traceparent 격리 | M | M | A,E,H 뒤 직렬 |
| I10 | §6 권한·workspace·service·real-surface | G/H/J/Z | Z | F1/F2 해결 전제 |

누락된 항목은 없다. 다만 I2의 Python suite 검증은 `execution-ledger.md`의
`mon_G5X30VS3R28RVHTF` exit 0 기준선을 변경 후 재실행해야 한다는 §6 문장이
그대로 유효하다.

## 선택 후속 (blocker 아님)

- Omo `issueops_project`(project-local)와 user entry의 중복 생성 금지는 §3에 있으나
  Claude `.mcp.json`의 dogfood 설정과 user-scope HTTP entry가 공존할 때의 우선순위를
  운영 문서에 한 줄 적으면 좋다.
- `Observation.Features []string`의 null/empty 구분을 I3 schema(I4 docs_index
  output schema와 동일 규칙)에 명시한다.
- `authority_file` 경로를 host transcript에 남기는 것은 "같은 OS 사용자 신뢰" 가정
  안이므로 허용되지만, README 수준 문서에 이 경계를 한 줄로 밝힌다.

## 결론

F1, F2를 수정하고 F3, F4를 한 문장씩 확정하면 PASS로 전환할 수 있다. 다른 변경이나
추가 인프라는 요구하지 않는다.
