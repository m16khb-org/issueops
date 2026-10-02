# G 독립 검증: native caller grant와 권한 호출자 마이그레이션

대상: `implementation-caller-authority.md`가 보고한 G 구현을 `contract.md` §2(발급·전달, 공용 인터페이스, 원자성·철회)와 §5 G 행, 기존 lifecycle 계약과 대조했다.
방법: G 신규 파일 전부와 마이그레이션된 호출자 diff를 직접 읽고, 보고서의 명령을 다시 실행하고, 임시 state와 임시 Git repo에서 실제 CLI 바이너리로 발급 흐름을 돌렸다. 실험은 `/tmp` 아래 사본과 `go test -overlay`로만 했고, 이 보고서 외에 repo 파일은 바꾸지 않았다.
기준 작업 트리: HEAD `92eaa001`, G 변경이 uncommitted 상태로 올라와 있는 트리.

## 판정

**PASS** (G 범위). 계약이 G에 요구한 발급·검증·철회·호출자 마이그레이션은 실제 코드와 실행 결과로 확인됐다.
다만 아래 결함 1건은 보고서의 주장과 어긋나므로 H가 legacy 경로에 손대기 전에 고쳐야 한다. 그 외 후속 3건은 동작 결함이 아니다.

| 구분 | 내용 | 위치 |
|---|---|---|
| 결함 D1 | `NativeActorVerifier()`는 `now`를 nil로 조립한다. capability가 바인딩된 context가 이 verifier에 도달하면 `Verify`가 `s.now()`에서 nil pointer panic을 낸다. 보고서는 "이 경로에서 fail-closed"라고 썼지만 실제로는 오류 반환이 아니라 panic이다. 오늘 production에서 바인딩된 context를 이 verifier로 보내는 호출자는 없다(`SyncExecutionBase` legacy export는 호출자 없음, `NewReviewMutationStore`는 `context.Background()`). | `internal/adapter/issueops/execution_lease_identity.go:27-29`, `internal/application/authority/service.go:141` |
| 후속 F1 | 재발급은 이전 credential 파일을 지우지 않는다. 같은 key 디렉터리에 거부되는 파일이 누적된다(실측 9개). 계약은 "이전 파일은 남지만 검증에서 거부된다"고 허용하므로 결함은 아니지만 GC가 없다. | `credential_files_unix.go:18-54`, `service.go:75-85` |
| 후속 F2 | `--cwd`는 CLI가 scope 검증에만 쓰고 `Issue`로 넘기지 않는다. `Issue`는 항상 cwd를 비워 resolve한다. 계약이 cwd를 grant에 요구하지 않으므로 허용 범위다. | `mcp_authorize_command.go:67-71`, `service.go:63` |
| 후속 F3 | `ValidatePlanCoordinator`는 관측 ancestry가 하나라도 있으면 verifier 없이 통과시킨다. G 이전과 같은 규칙이며 보고서가 편차로 밝혔다. capability 경로는 verifier 결과와 identity를 대조한다. | `internal/domain/issueopsauthorization/plan_link.go:17-18, 23-27` |

## 실행한 명령과 결과

임시 바이너리는 `/tmp/g-verify-*/issueops`로 빌드했다. `bin/`은 gitignore 대상이지만 repo 안은 건드리지 않기 위해서다.

| 명령 | 결과 |
|---|---|
| `gofmt -l` (변경·신규 Go 파일 전체) | 출력 없음, exit 0 |
| `go vet ./...` | 출력 없음, exit 0 |
| `go build -o /tmp/.../issueops ./cmd/issueops` | exit 0 |
| `go test -race -count=1` 22개 패키지 (domain/application/outbound authority, issueopsauthorization, issueopscycle, lease, completion, execution, replacement, remote, branch, delegation, decision, routing, inbound lease/decision/routing, outbound lease, sqlstore, adapter/issueops, contract/authority, contract/issueops) | 전부 `ok`. `adapter/issueops`는 203.8s. `WARNING: DATA RACE` 0건 |
| `go test -race -count=1 ./cmd/issueops/issueopsapp` | `TestResponseContractsGolden`만 실패: `$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count unexpected`. I2(B)의 selfaugment 변경이 원인이며 G 범위 밖. authority wiring 테스트 2건은 통과. DATA RACE 0건 |
| `go test -count=1 ./internal/architecture` | `TestDDDResponsibilityInventoryMatchesSource`만 실패(신규 파일·심볼 inventory 미등록). layer dependency·contract role 검사는 통과했고 `internal/architecture`는 수정되지 않았다 |
| `go test -overlay` 외부 probe 2건 | 아래 "추가 probe" 참조 |

보고서가 적은 범위 밖 실패 2건(golden, inventory)은 그대로 재현됐고 다른 실패는 없었다.

## 계약 항목별 대조

| 계약 항목 | 판정 | 근거 |
|---|---|---|
| `issueops mcp authorize --workspace-root PATH ACTOR_FLAGS [--cwd] --json` | PASS | `mcp_command_facade.go:11-18` dispatch, `mcp_authorize_command.go:40-63` 필수 플래그 검사. 실측: 누락 플래그·추가 인자·상대 경로 모두 exit 1 |
| CLI가 직접 관측한 ancestry만 사용, caller ancestry 불수용 | PASS | `mcp_authorize_command.go:72` `command.observe(command.pid())`, 즉 CLI 자신의 PID 조상. Issue는 `verifyNative`(`service.go:59, 207-216`)로 receipt가 조상에 있고 live인지 검사 |
| cycle/holder 불필요, lease 불변 | PASS | `Issue`(`service.go:52-90`)는 lease를 읽지 않는다. 실측 DB에는 `issueops_authority_v1` bucket 행 3개뿐, 다른 bucket 0개 |
| scope = canonical root + Git common-dir, worktree 공유, 비Git은 root 한정 | PASS | `scope.go:24-57` (`rev-parse --path-format=absolute --git-common-dir --show-toplevel`, `GIT_*` env 제거 `:84-92`), `authority.go:171-177 ScopeAllows`. 실측: worktree 재발급이 같은 key 디렉터리, symlink root도 같은 key, 다른 repo는 다른 key, 비Git root는 `git_common_dir=""` |
| key = SHA-256(scope-kind NUL anchor NUL host NUL session NUL agent) | PASS | `authority.go:60-68` |
| secret 32 bytes base64url, credential `key.secret` | PASS | `service.go:18, 68-72`, `authority.go:72-80` |
| 재발급은 이전 secret 철회, TTL 12h | PASS | `authority.go:18`, `:87-99`, `:153-162`. `TestReissueRevokesPreviousCredential`, `TestCredentialExpiresAtTTL`(경계 `TTL-1ns` 허용, `TTL` 거부). 실측: 동시 6회 발급 뒤 유효 토큰 1개, 최초 토큰 무효 |
| bucket 저장 내용: identity/receipt, scope, hash, 시각만. ancestry 미저장 | PASS | `authority.go:102-108` encode 전 `NormalizeIdentity`, `:125-144` `len(ProcessAncestry) != 0`이면 invalid state. 실측 저장 JSON 키: `actor{host,session_id,session_process}, expires_at, git_common_dir, issued_at, key, schema_version, source_root, token_sha256`. raw 토큰·secret·`mcp-http` 경로·ancestry 문자열 0회 |
| raw token stdout 미출력, `{ok,authority_file,expires_at}`만 | PASS | `mcp_authorize_command.go:84-96`. 실측 stdout에 토큰·secret 0회 |
| `<state>/mcp-http/grants/<key>/<sha256(token)>` 불변 0600 | PASS | `credential_files_unix.go:35` `O_CREAT|O_EXCL|O_NOFOLLOW` 0600, `:49` dir fsync. 실측 파일 0600, key/grants/mcp-http 디렉터리 0700 |
| 서버는 managed dir의 정확한 key/digest만 root-relative open, symlink·비정규·다른 uid·group/world 거부 | PASS | `credential_files.go:26-39` 경로 형식, `credential_files_unix.go:106-136` `openat` + `O_NOFOLLOW` + `privateEntry`(S_IFMT, `0o077`, euid). 외부 probe: 0640 파일, symlink leaf, 0750 key dir, FIFO, digest 불일치, state 밖 경로 전부 거부 |
| 공용 DTO (`Record/IssueRequest/Receipt/Use/Scope`), `Use.Token json:"-"` | PASS | `contract/authority/types.go:26-63`, `TestUseDoesNotSerializeCredential`, `TestUseDoesNotAcceptCredentialFromJSON` |
| `VerifiedActor{Identity,Method}` JSON 입력 불가, capability 결과 ancestry nil | PASS | `contract/issueops/verified_actor.go:10-13` 두 필드 `json:"-"`, `service.go:240-243` `NormalizeIdentity`로 ancestry 제거. `TestVerifiedActorDoesNotAcceptProofFromJSON`, `TestBindVerifiesCapabilityWithNilAncestry`, wiring 테스트 `:71` |
| port 인터페이스 5종 + `ScopeResolver` | PASS | `port/authority/authority.go:10-39`. `ProcessInspectorFunc` adapter 추가는 보고서가 밝힌 편차 |
| `Service` 생성자·메서드 시그니처 | PASS | `service.go:31`, `:52`, `:94`, `:115`, `:133`, `:155` 계약과 동일 |
| Issue는 native 분기만 허용 | PASS | `service.go:56-58` 바인딩된 context면 `authority_invalid` |
| Within callback에서 파일 생성 + Apply, 고아 파일은 검증에서 거부 | PASS | `service.go:75-85`, `repository.go:24-55`(non-nil 반환만 Apply). 고아 파일 digest는 저장 hash와 달라 `Check`에서 거부 |
| 손상·미래 schema grant를 재발급으로 덮어쓰지 않음 | PASS | `repository.go:38-44` decode 실패 시 callback 미실행. `TestUnsupportedStoredSchemaFailsClosed`, `TestRepositoryRejectsUnsupportedSchemaAndNestedLeaseSpan` |
| 공개 오류 `authority_required/authority_invalid`, schema는 `invalid state` | PASS | `authority.go:26-39`, `contract/authority/types.go:12-14, 24`. `Error`를 domain에 둔 이유(contract role 검사)는 보고서 기재 |
| Bind: hash/TTL/scope/live receipt 검사 뒤 private context key에 보관, span 안 열지 않음 | PASS | `service.go:94-110`, `:183-205`. `application/authority`에 `WithSpan`/`sqlstore` import 없음(grep) |
| Verify: 바인딩 있으면 매번 grant 재검증, 없으면 native ancestry+live | PASS | `service.go:133-153`, `TestUnboundVerifyUsesNativeAncestryAndLiveProcess`. 실측 CLI native fallback: 올바른 ACTOR_FLAGS로 `execution release`는 "record io-missing not found"까지 진행, 위조 started_at·live 비조상 프로세스는 "not in the local process ancestry" |
| stdio 혼합 입력은 supplied actor가 grant identity와 일치해야 함 | PASS | `authority.go:181-192 MatchIdentity`, `service.go:149`. 테스트 `:286-291` |
| `WithRecordGuard`, BindSpan 재검증·locked reader 재사용·진입 시각 기준 TTL | PASS | `sqlstore/record_guard.go:23-44`(E 소유), `service.go:115-128, 141-144`. `TestBindSpanRechecksThroughLockedReaderAndSerializesRotation`(unspanned reader 재호출 0, locked reader 2회, 만료 후에도 span 진입 시각 유지). wiring 테스트: 실제 sqlstore에서 연속 top-level span 2회 재검증, 재발급 뒤 다음 span은 callback 실행 전 `authority_invalid`, 다른 root span도 `authority_invalid` |
| Issue의 Within은 lease span 밖에서만 | PASS | `TestRepositoryRejectsUnsupportedSchemaAndNestedLeaseSpan:93-99` `NestedSpanError` |
| 서버 restart는 grant 보존, host restart/PID 재사용은 거부 | PASS | `TestServerRestartKeepsGrantButHostRestartAndPIDReuseRevoke` |
| 동시 caller 격리, unbound 요청의 capability 비상속 | PASS | `TestConcurrentRequestsKeepCapabilitiesIsolated`(32 goroutine, `-race`). 외부 probe: 다른 요청에 바인딩이 살아 있어도 `context.Background()` Verify는 native 오류 |
| 전역 current actor/workspace 금지, `os.Chdir/Getwd/env` fallback 금지 | PASS | G 7개 경로 grep: package-level var는 interface assertion과 sentinel error뿐, `Chdir/Getwd/Getenv` 0건. request identity는 `bindingKey/spanKey` context value |
| grant는 신원 증명일 뿐, holder/generation/token/cwd/CAS는 기존 core | PASS | 아래 "호출자 마이그레이션" |

## 실제 CLI 시나리오 (격리 state, 임시 Git repo + worktree, session = 커널 프로세스 자신)

- 발급: exit 0, stdout `{"ok":true,"authority_file":"$STATE/mcp-http/grants/<key>/<digest>","expires_at":"+12h"}`. 토큰 문자열과 secret은 stdout에 없다. 파일 이름이 토큰의 SHA-256과 일치하고 디렉터리 이름이 토큰의 key 부분과 일치한다.
- 권한: 파일 0600, `<key>`·`grants`·`mcp-http` 0700.
- worktree root로 재발급: exit 0, 같은 key 디렉터리, 새 경로, 이전 파일은 남아 있음.
- symlink로 가리킨 root: 같은 key(canonical 경로 사용). 다른 repo: 다른 key.
- DB `issueops_v1/issueops.db`: `records` 테이블에 `issueops_authority_v1` 행 3개(repo, other, plain), 다른 bucket 없음. 각 행의 `token_sha256`은 마지막으로 발급된 토큰과만 일치한다.
- 위조: 자식 `cat` 프로세스 receipt → exit 1, `{"ok":false,"authority_file":"","expires_at":""}`, stderr "native session process receipt is not in the local process ancestry". 올바른 PID에 틀린 started_at → 같은 거부. 두 경우 모두 credential 디렉터리가 생기지 않았다.
- 비Git root + `--cwd` 내부: exit 0. `--cwd` 외부: exit 1 "cwd is outside workspace_root". 상대 root: "workspace_root must be absolute".
- 동시 발급 6회(같은 key): 전부 exit 0, 경로 6개 모두 다름, DB 행 1개, 유효 토큰 1개. span lock이 rotation을 직렬화한다.
- native fallback: `execution release --id io-missing ...`에 올바른 actor는 verifier를 통과해 "issueops record io-missing not found", 위조 actor 2종은 ancestry 오류. lifecycle CLI에는 `authority_file` 플래그가 없어 grant가 CLI 경로에 섞이지 않는다.

## 추가 probe (`go test -overlay`, repo 파일 미변경)

- `TestProbeLegacyNativeVerifierUnderBoundContext`(package `issueops`): 실제 Repository/CredentialFiles/ScopeResolver/`NativeProcessInspector`로 발급·Bind한 context를 `NativeActorVerifier().Verify`에 넘기면 `panic=runtime error: invalid memory address or nil pointer dereference`, `err=<nil>`. 결함 D1의 근거다.
- `TestProbeUnboundVerifyIgnoresOtherRequestsBinding`: 통과.
- `TestProbeCredentialBoundary`(package outbound `authority`): 같은 토큰 2회 Write는 두 번째가 실패(O_EXCL). 0640 파일, symlink leaf, 0750 key dir, FIFO, digest 불일치 파일, state 밖 경로 모두 `authority credential path is not a managed credential file`.

## 호출자 마이그레이션 점검

LSP/grep으로 production 호출자를 전수 확인했다. `cycleapp.NormalizeNativeActor`의 production 호출자는 0건이고, `NewMutationAuthority(`·`AuthorizeHolder(`·lease/completion 생성자는 모두 `issueOpsActorVerifier()`를 받는다(`authority_wiring.go:31`, 요청마다 immutable 의존성으로 새 Service 조립).

| 호출자 | 확인 내용 |
|---|---|
| 일반 mutation `MutationAuthority.Validate/Authorize/AuthorizeHolder` | `mutation_authority.go:36-48`. `verifyHolderCaller`(`:53-72`)는 holder receipt + 요청 ancestry로 verifier 입력을 만들고 실패를 "current write lease holder" 오류에 `%w`로 감싼다. domain `ValidateHolder`(`authorization.go:11-39`)는 verified identity·receipt를 holder와 대조한다. verifier nil이면 거부(`TestAuthorizeHolderObservesPathsOnlyAfterIdentityMatches:64`). capability 입력에 ancestry 없음·다른 세션 프로세스 거부·typed 오류 보존(`TestAuthorizeHolderUsesCapabilityIdentityWithoutAncestry`) |
| plan link | `plan_link_authority.go:10-26`. ancestry 없을 때만 verifier 호출, 오류 메시지가 verifier 원인을 `%w`로 덧붙이는 점만 변경(기존 문구 유지) |
| lease claim/release/resume/reseed | `release.go:72-96 resolveActor`가 verified identity로 domain actor를 만든다. holder·generation·cwd는 각 transaction 그대로(`TestCapabilityIdentityNeverReleasesAnotherHoldersLease`, `TestCapabilityHolderStillObeysGenerationAndCanonicalCWD`) |
| completion | `complete.go:36-48` completion-local `ActorVerifier` func, 조립은 `issueops_completion_wiring.go:21-44`. 다른 holder·stale generation 거부, holder capability는 ancestry 없이 완료(`authority_capability_test.go`) |
| execution reconcile / replacement | `execution_reconcile.go:20-27, 46-49`, `replacement/service.go:22-35`. verifier nil은 명시 오류. replacement의 `InspectProcess`는 인벤토리용으로 남음 |
| sync-base | `execution_sync_base.go:60-72, 93-101`. holder 대조는 identity+receipt(`sameNativeActor`)이며 ancestry를 요구하지 않는다. legacy `SyncExecutionBase`는 `NativeActorVerifier()` 사용(production 호출자 없음, 결함 D1 영향 범위) |
| preparation / publication / create preparation / remote command | `issueops_preparation_wiring.go:131-134`, `publication_actor.go:29-33`, `create_preparation.go:26-31`, `issueops_remote_commands.go:22-25` 모두 ctx 전달 |
| branch link/prepare/retarget/workspace link, delegation start/status/verdict | `Validate(ctx, …)`로 ctx 전달. delegation `Status`는 `repair=true`일 때만 Validate를 부르고 읽기 경로는 변경 없음(`status.go:46-57`) |
| decision / routing | `NewService(…, verifier)` + `AuthorizeHolder(ctx, …)` |
| 9개 wiring 파일 | 전부 `issueOpsActorVerifier()` 주입 확인 |
| ctx를 버리던 receipt write | artifact verification `Record`, review reflection, child create `Outcome/Complete`, child reconcile `RecoveryFailure`가 `context.WithoutCancel(ctx)`로 바뀌어 취소 무시는 유지하고 권한 값은 보존한다. `withIssueOpsLock`이 `db.WithSpan(ctx)`를 열므로 record guard가 걸린다 |
| 변경 전부터 ctx 없는 경로 | `ArtifactVerificationService.Validate`(읽기 전용, 권한 검사 없음)와 `issue_create.go`의 intent 호출은 기존처럼 `context.Background()`. 보고서가 H 경계로 명시한 대로이며 G 회귀가 아니다 |

읽기 전용·advisory 동작이 바뀐 호출자는 없었다. 요청 context가 새로 손실된 호출자도 없었다.

## 보고서 주장 대조

- RED→GREEN 12건과 추가 회귀 테스트는 모두 실제 테스트 함수로 존재하고(`service_test.go` 11건, `authority_wiring_test.go` 2건, lease/completion/cycle capability 테스트 8건, credential/repository/scope 7건) `-race`로 통과했다. 변경·신규 테스트에 추가된 sleep/polling은 없다(`time.Sleep`·`time.After` 히트 4건은 모두 diff 밖의 기존 줄).
- "legacy `NewReviewMutationStore`·`SyncExecutionBase`에서 capability 요청은 fail-closed" 주장은 틀렸다. 실제로는 panic이다(결함 D1). `NewReviewMutationStore`는 `context.Background()`를 쓰므로 오늘은 도달하지 않는다.
- 나머지 결정·편차(domain `Error`, `ProcessInspectorFunc`, `ValidatePlanCoordinator` verified 인자, native 일반 mutation의 live 검사 추가, completion domain `NormalizeActor/ValidateLiveActor` 미사용)는 코드와 일치한다.

## 이 검증이 하지 않은 것

- HTTP transport, 혼합 입력 거부, `authority_file` schema, 서비스·설치는 H/J 범위라 검증하지 않았다.
- 같은 OS 사용자가 credential 경로를 알면 행사할 수 있다는 경계는 계약대로 방어 대상이 아니므로 테스트하지 않았다.

## 작업 트리 확인

이 보고서 작성 전후 `git status`에서 새로 추가되거나 바뀐 repo 파일은 이 파일뿐이다. overlay probe 파일, 임시 state, 임시 바이너리는 모두 `/tmp/g-verify-*` 아래에 있다.
