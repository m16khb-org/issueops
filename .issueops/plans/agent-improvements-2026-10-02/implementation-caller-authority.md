# G 구현 보고: native caller grant와 권한 호출자 마이그레이션

범위: contract.md §2 발급·전달, 정확한 공용 인터페이스, 원자성·철회와 §5 G 행.
HTTP listener, 서비스 수명, 설치(H/J)는 구현하지 않았고 완료를 주장하지 않는다.
commit·전역 설치·HOME 설정 변경은 하지 않았다.

## 결과 요약

- `issueops mcp authorize --workspace-root PATH ACTOR_FLAGS [--cwd PATH] --json`을 추가했다.
  CLI가 직접 관측한 ancestry와 live PID receipt를 기존 native 규칙으로 검증하고,
  cycle·lease 없이 caller capability를 발급한다. stdout에는 `{ok,authority_file,expires_at}`만 나간다.
- `application/authority.Service`의 `Issue/Bind/Verify/BindSpan/Check`, request-local context,
  `sqlstore.WithRecordGuard` 연결용 composition `bindIssueOpsAuthority`를 구현했다.
- lease claim/release/resume/reseed, completion, reconcile, replacement, sync-base,
  preparation, publication, branch, delegation, decision, routing과 일반 mutation guard가
  모두 `ActorVerifier`를 거친다. 기존 native/stdio 동작은 바인딩이 없으면 그대로 유지된다.

## 새 파일

| 경로 | 내용 |
|---|---|
| `internal/domain/authority/authority.go` | key/token/digest, record 생성·codec(schema v1 fail-closed), Check(hash·TTL·scope), MatchIdentity, `Error{Code,Reason}` |
| `internal/application/authority/service.go` | Service, private binding/span context, native·capability 검증 |
| `internal/adapter/outbound/authority/repository.go` | 고정 user-state DB `issueops_v1`의 `issueops_authority_v1` bucket, `Within`(자체 span), 무생성 `Get` |
| `internal/adapter/outbound/authority/credential_files{,_unix,_other}.go` | `<state>/mcp-http/grants/<key>/<sha256(token)>` 불변 0600 파일, openat+O_NOFOLLOW 읽기 |
| `internal/adapter/outbound/authority/scope.go` | canonical root, Git common-dir(전 worktree 공유), 비Git bounded root, `GIT_*` env 차단 |
| `cmd/issueops/issueopsapp/authority_wiring.go` | 불변 서비스 조립, `issueOpsActorVerifier`, `bindIssueOpsAuthority`(Bind 후 record guard 부착) |
| `cmd/issueops/issueopsapp/mcp_authorize_command.go` | CLI 어댑터, `mcp_command_facade.go`에 `authorize` dispatch 추가 |

인계받은 공유 파일 변경: `contract/authority/types.go`(SchemaVersion, Bucket, 오류 코드,
`NativeActor/ProcessReceipt/VerifiedActor` capability-local alias, `ErrInvalidState`),
`contract/issueops/verified_actor.go`(Method 상수), `port/authority/authority.go`(`ProcessInspectorFunc` adapter).
`contract/issueopsauthorization/types.go`의 `VerifiedActor` alias는 parent가 이미 추가한 그대로 쓴다.

## 계약 대응

- **키·토큰**: key = SHA-256(scope-kind NUL anchor NUL host NUL session NUL agent), anchor는 Git
  common-dir 또는 비Git source root. secret은 `crypto/rand` 32 bytes base64url, credential은 `key.secret`.
- **재발급**: 같은 key의 grant를 덮어써 이전 hash를 철회한다. 새 경로를 반환하며 이전 파일은 남지만 검증에서 거부된다.
- **TTL 12시간**: `now >= expires_at`이면 거부한다. span 안에서는 BindSpan 진입 시각을 기준으로 한다.
- **저장**: schema v1, key·source_root·git_common_dir·actor(host/session/agent/session_process)·token_sha256·issued/expires만.
  encode 전 ancestry를 비우고, decode는 unknown field·missing/zero/future schema·trailing data를 `invalid state`로 거부한다.
  재발급도 손상·미래 schema grant를 덮어쓰지 않고 실패한다.
- **발급은 lease가 아니다**: Issue는 lease를 읽거나 바꾸지 않고, lease span 안에서 열면 sqlstore가 nested span으로 거부한다.
- **Bind/Verify/BindSpan/Check**: Bind·Verify·BindSpan은 span을 열지 않는다. Verify는 바인딩이 있으면
  매번 grant를 재검증하고, span 안에서는 BindSpan이 넘긴 locked reader와 진입 시각을 재사용한다.
  바인딩이 없으면 기존 native ancestry+live 검사를 한다. capability 결과의 ancestry는 항상 nil이다.
  supplied actor가 비어 있지 않으면 grant identity와 일치해야 한다(stdio 혼합 규칙).
  HTTP의 혼합 입력 거부는 transport 판단이라 H가 구현한다.
- **record guard**: `bindIssueOpsAuthority`는 `sqlstore.WithRecordGuard(ctx, service.BindSpan)`를 붙인다.
  guard는 span마다 소비되고 호출자 ctx에는 남아 이후 top-level span이 각각 재검증된다.
  grant가 없는 다른 root의 span은 authority_invalid로 실패한다(조용한 foreign-root 허용 없음).
- **오류**: authority_required/authority_invalid는 `domain/authority.Error`다.
  contract에 함수를 두면 DDD contract-role 검사를 깨므로 domain에 두었다.
  schema 오류는 공용 `invalid state`다. reason에는 credential과 그 경로를 넣지 않는다.

## 호출자 마이그레이션

- 시그니처: `ValidateHolder(record, actor, verified)`, `ValidatePlanCoordinator(actor, verified, canonical)`,
  `NewMutationAuthority(pathsMatch, verifier)`, `Validate(ctx, record, actor)`, `ValidatePlanLink(ctx, …)`,
  `AuthorizeHolder(ctx, record, actor, pathsMatch, verifier)`. 호환 래퍼는 추가하지 않았다.
- 일반 mutation: holder가 있으면 actor의 관측 ancestry와 holder의 기록된 receipt로 verifier 입력을 만든다.
  domain은 verified identity와 그 process receipt를 holder와 대조한다. 실패는 기존
  "current write lease holder" 오류에 원인을 `%w`로 감싼다. plan link는 ancestry가 없을 때만 verifier를 호출한다.
- lease 4종은 `ProcessInspector` 자리를 `authorityport.ActorVerifier`로 바꿨다. completion은 아키텍처 규칙상
  port를 import할 수 없어 completion-local `ActorVerifier` func을 두고, composition이 authority Service로 어댑트한다.
  holder·generation·token·CAS·canonical cwd 판정은 각 기존 transaction에 그대로 있다.
- execution reconcile, replacement(`Verifier` 필드, 인벤토리용 `InspectProcess`는 유지),
  publication normalize(ctx 추가), create preparation, preparation `ValidateActor(ctx, …)`,
  sync-base(`VerifiedSyncExecutionBase(verifier)`, 기존 `SyncExecutionBase`는 native 기본값), decision, routing.
- contract §2의 9개 wiring에 더해 claim/release/reseed/resume/completion/decision/routing/execution/
  replacement/preparation/publication/remote_commands/child_create wiring을 같은 단계에서 갱신했다.
- 요청 권한 값을 버리던 receipt write는 `context.WithoutCancel(ctx)`로 바꿨다. 대상은 artifact verification
  Record, review reflection stamp, child intent Outcome/Complete/RecoveryFailure다. 취소 무시라는 원래 의도는 유지한다.
- legacy `NewReviewMutationStore`는 ctx 없는 port라 `NativeActorVerifier()`(grant store 없음)를 쓴다.
  capability 요청은 이 경로에서 fail-closed다. 아래 H 경계에 적었다.

## 검증 증거

**Mutation RED → 복원 GREEN** (각 성질을 production에서 한 줄씩 무력화하고 대상 테스트 실행, 이후 원복):

| 성질 | 무력화 | RED 테스트 |
|---|---|---|
| 재발급 철회 | digest 비교 생략 | TestReissueRevokesPreviousCredential |
| 만료 경계 | `!now.Before` → `now.After` | TestCredentialExpiresAtTTL |
| host 재시작·PID 재사용 | live 검사 생략 | TestServerRestartKeepsGrantButHostRestartAndPIDReuseRevoke |
| 다른 repo/root | ScopeAllows 생략 | TestBindRejectsMissingMalformedForeignAndWrongCredential |
| missing/future schema | schema 검사 생략 | TestUnsupportedStoredSchemaFailsClosed |
| 다른 actor 혼합 입력 | MatchIdentity 생략 | TestBindVerifiesCapabilityWithNilAncestry |
| credential symlink | leaf O_NOFOLLOW 제거 | TestCredentialReadRejectsSymlinksModesAndForeignContent |
| group/world 권한 | 0o077 검사 제거 | 같은 테스트 |
| 다른 holder 프로세스 | domain receipt 대조 제거 | TestAuthorizeHolderUsesCapabilityIdentityWithoutAncestry |
| locked 재검증·rotation | BindSpan check 제거 | TestBindSpanRechecks…, TestMCPAuthorizeIssuesPreLeaseCapabilityAndGuardRechecksEachSpan |
| 발급 시 관측 ancestry | verifyNative 생략 | TestIssueRequiresObservedAncestryAndLiveSession, TestMCPAuthorizeRejectsSessionOutsideObservedAncestry |
| lease가 verified identity 사용 | 요청 actor로 대체 | TestResolveActorUsesVerifiedIdentityNotRequestFields |

12건 모두 RED였고 원복 뒤 `go build ./...` 성공, 같은 패키지 `-race` GREEN.

**추가 회귀 테스트**: 발급 전 lease 불필요·저장 내용(토큰·경로·ancestry 부재), 32 goroutine 요청 격리
(`-race`), unbound 요청의 capability 비상속, 취소, credential 경로 변형 7종·FIFO·취소 시 무쓰기,
nested span 거부, worktree 간 common-dir 공유와 `GIT_DIR` 무시, capability가 다른 holder의
release/complete를 못 하고 stale generation·foreign cwd를 우회하지 못함(lease·completion).
실제 sqlstore로 연속 span 재검증, 재발급 뒤 다음 span의 callback 미실행, foreign root 거부도 확인했다.

**실행 결과**
- `gofmt -l`(변경·신규 Go 파일) 출력 없음, `go vet ./...` 출력 없음, `git diff --check` 출력 없음.
- `go test -race`: domain/application/outbound authority, issueopsauthorization, issueopscycle, lease,
  completion, execution, replacement, remote, branch, delegation, decision, routing, inbound/outbound lease,
  sqlstore, issueopsapp 모두 ok. data race 경고 없음.
- `go test ./internal/... ./cmd/...` 전체: 실패는 아래 두 건뿐이다(G 범위 밖).
  - `internal/architecture` TestDDDResponsibilityInventoryMatchesSource: 신규 파일·심볼 inventory 미등록.
    parent 소유 inventory 갱신 대상.
  - `cmd/issueops/issueopsapp` TestResponseContractsGolden: `self_verify_compare…reused_count`.
    I2(B)의 selfaugment 변경에서 나온 것이고 이 변경과 무관하다.

**격리 실제 CLI 시나리오** (`go build`한 임시 바이너리, `ISSUEOPS_STATE_DIR=$W/state`, 임시 Git repo+worktree,
session process는 명령을 실행한 셸 자신):
- authorize exit 0, stdout `{ok:true, authority_file:$W/state/mcp-http/grants/<key>/<digest>, expires_at:+12h}`, stdout의 토큰 출현 0회.
- 파일 0600, key/grants/mcp-http 디렉터리 0700.
- worktree root로 재발급: exit 0, 같은 key 디렉터리, 새 경로.
- `issueops_v1/issueops.db`: authority bucket 1행, 다른 record 0행(cycle/lease 미생성). 키는
  `actor, expires_at, git_common_dir, issued_at, key, schema_version, source_root, token_sha256`,
  actor 키는 `host, session_id, session_process`. hash는 재발급 토큰과만 일치, raw 토큰·경로·ancestry 문자열 없음.
- ancestry 밖 live 프로세스(자식 `cat`)와 틀린 started_at: exit 1, `{ok:false}`,
  "native session process receipt is not in the local process ancestry", credential 미생성.
- native fallback(바인딩 없음): `execution release`는 올바른 ACTOR_FLAGS면 verifier를 통과해
  "record io-missing not found", 위조 actor면 ancestry 오류로 거부된다.

## 결정·편차

- `Error`를 contract가 아닌 domain에 두었다(contract 함수 role 검사). H는 `errors.AsType[*authoritydomain.Error]`로 코드를 얻는다.
- port에 `ProcessInspectorFunc` adapter를 추가했다(테스트·composition 어댑터용).
- `ValidatePlanCoordinator`에 verified 인자를 추가했다. 기존 native 규칙(관측 ancestry 존재)은 그대로 두었다.
- native 일반 mutation은 이제 holder receipt의 live 검사도 거친다(계약의 "Verify 없으면 native ancestry+live").
  이 변화로 가짜 PID 1/4242 fixture를 쓰던 테스트는 실제 live receipt(PID 1의 관측값, 자식 프로세스에서도 동일)로 바꿨다.
  live 검사를 위해 `ps`가 필요한 remotecmd 테스트 1건은 PATH에서 시스템 경로를 지우지 않게 했다.
- completion domain의 `NormalizeActor/ValidateLiveActor`는 production에서 더 쓰이지 않는다(테스트 helper만 사용).
  completion domain은 G 쓰기 범위 밖이라 남겼다. 후속 정리 대상이다.
- `cycleapp.NormalizeNativeActor`는 계약대로 native helper로 남겼고 production 호출자는 없다.

## H/J에 넘기는 경계

- H: MCP handler에서 `RequestScope → CredentialFiles.Read → bindIssueOpsAuthority → ForRequest → dispatch`.
  `bindIssueOpsAuthority`가 Bind와 record guard 부착을 함께 한다. HTTP에서 `authority_file`과 actor 필드
  혼합 거부, `authority_file?` schema, 오류 코드의 JSON 매핑은 H 몫이다.
- H: `mcpcli/mcp_tool_issueops_execution.go`의 서버 `os.Getpid()` ancestry 부착을 HTTP 경로에서 제거해야 한다.
  stdio는 지금처럼 native ancestry로 동작한다. artifact verification `Verify`도 `s.observe()`로
  서버 ancestry를 붙인다. capability가 바인딩되면 무시되지만 HTTP 경로에서는 observe 주입을 끊어야 한다.
- H: ctx를 받지 않는 legacy 경로는 capability 요청에서 fail-closed다. 대상은 `NewReviewMutationStore`
  (feedback/domain review/ai-slop/planning recorder/evidence recorder)와 `context.Background()`로 span을 여는
  issue-create intent(권한 검사는 없음)다. HTTP parity를 주장하려면 이 port에 ctx·verifier를 넣어야 한다.
- H: request ctx를 모든 mutation closure까지 전달해야 guard가 span에 걸린다. ctx를 버리는 store 호출은 guard 없이
  unspanned 재검증만 받는다.
- J/Z: 서비스·설치·host config, `issueops mcp` usage 문서, DDD inventory와 response golden 갱신, 운영 문서.
