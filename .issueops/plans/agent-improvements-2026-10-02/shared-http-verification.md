# H 공용 HTTP 독립 검증 보고 (교정 증분 재감사)

대상: `implementation-shared-http.md`가 열거한 H 변경 전부와 교정 증분(결함 5건 대응). 기준: `contract.md` §2·§3·§5·§6과 기존 stdio(native) 동작.
기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e` + 작업 트리(uncommitted 460건), Go 1.26.4 darwin/arm64, SDK v1.8.0.
저장소는 이 문서 외에 바꾸지 않았다. 보조 프로그램(`hreceipt`, `hseed`)은 `go build -overlay`로 `/tmp/h-verify2/bin`에만 만들었고, 변이 실험도 `go test -overlay`로만 했다. 빌드 뒤 `cmd/hreceipt`·`cmd/hseed`가 repo에 없음을 확인했다(`test -e` 결과 "aux absent"). 실험 state·repo·서버 cwd는 모두 `/tmp/h-verify2` 아래다.

작업 트리는 검증 중에도 parent가 수정하고 있었다. 처음 읽을 때 `mcp_facade.go:86`은 `BindTrace: issueopsrecord.WithTraceparent`였고, 빌드 직전(13:44:50Z)에 `issueOpsMCPTraceBinding(os.Stderr)`로 바뀌었다. 아래 모든 테스트·실측은 바뀐 뒤 상태로 했으며, 그 상태의 파일 해시(sha256 앞 12자)는 문서 끝에 적었다.

## 판정

**조건부 통과.** 이전 보고의 결함 3건(gates 기본 cwd, gates 경로 이탈, MCP `BaseSync` 누락)과 parent가 추가한 context 결함 2건(execution persist의 `Background` commit, loop·worker·state의 `Background` lock과 lock 아래 grant 재확인)은 모두 소스·테스트·변이 RED·실제 바이너리로 고쳐졌음을 확인했다.
새로 확인한 결함은 1건이다. `worker_run_read_only`는 Bind 뒤 grant가 철회돼도 lock 아래에서 재확인하지 않고 명령을 실행·기록한다. loop와 달리 guard를 pass-through로 바꿔 끼웠기 때문이며, contract §2 "rotation과 write는 같은 lock으로 직렬화한다"와 parent 지시(loop/worker/state의 locked grant revocation)를 글자대로 만족하지 못한다. 아래 "결함 1"에 재현 테스트 결과를 적었다.

| # | 결함 | 어긴 조항 | 증거 |
|---|---|---|---|
| 1 | `unboundWorkerStore.WithLock`이 요청 ctx의 record guard를 `passThroughRecordGuard`로 **덮어써** worker span에서 grant를 재확인하지 않는다. Bind 뒤 grant row를 지워도 `worker_run_read_only`가 `status=succeeded`로 끝난다. 같은 조건에서 loop 쓰기는 `revoked`로 거부된다 | §2 "BindSpan은 Check로 재검증", "rotation과 write는 같은 lock으로 직렬화한다" | `request_store_fence_wiring.go:34-42`, overlay 프로브 `TestProbeWorkerWriteAfterGrantRevokedPostBind` 출력 `loop_start after revoke: err=authority_invalid … revoked`, `worker_run_read_only after revoke: err=<nil> status=succeeded` |

## 읽은 파일

신규·수정 production: `mcpcli/{mcp_tool_authority,mcp_request_scope,mcp_request_dispatch,mcp_http,mcp_http_bearer,mcp_sdk_server,mcp_tools,mcp_tool_issueops_execution,mcp_tool_gates,mcp_tool_loop,mcp_tool_policy_state,mcp_tool_assistant_worker,generated_command_provenance}.go`, `issueopsapp/{mcp_facade,mcp_http_command,mcp_service_command,request_store_fence_wiring,gates_wiring,loop_wiring,mcp_trace_wiring,cycle_readiness_wiring,planning_recorder_wiring,gate_readiness_wiring}.go`, `adapter/gates/{workspace_store,check}.go`, `adapter/issueops/{execution_state,issueops_state,issueops_lock,execution_publication_store}.go`, `adapter/looprun/{store,lifecycle}.go`, `adapter/worker/store.go`, `adapter/outbound/state/{state_io,state_lock}.go`, `adapter/outbound/sqlstore/{sqlstore(360-500,762-872),record_guard}.go`, `application/{looprun,worker,state,authority}/service.go`, `adapter/outbound/authority/scope.go`, `adapter/outbound/issueopslease/{sqlite,reconcile_repository,resume_repository}.go`, `adapter/outbound/issueopscompletion/repository.go`.
테스트 14개 파일의 Test 목록과 교정 증분 테스트 3개(`mcp_http_workspace_boundary_test`, `request_store_fence_wiring_test`, `execution_persist_context_test`) 본문, parent 소유 `mcp_trace_observer_test.go` 본문. `time.After`는 `mcp_http_helpers_test.go:83`, `mcp_http_command_test.go:88`(15s), `request_store_fence_wiring_test.go:97`(30s) 세 곳이며 모두 종료 대기 상한이다. sleep·polling은 없다.

## 명령 결과

| 명령 | 결과 |
|---|---|
| `go build -o /tmp/h-verify2/bin/issueops ./cmd/issueops` | exit 0 (`go version -m`: go1.26.4, `issueops v0.1.1-…-92eaa0014384+dirty`) |
| `go vet ./...` | exit 0, 출력 없음 |
| `gofmt -l` (H production 33개), `git diff --check` (같은 목록) | 출력 없음 |
| `go test ./... -count=1` | ok 320개. FAIL 2개뿐: `issueopsapp` `TestResponseContractsGolden`(`$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count`, B 소유), `internal/architecture` `TestDDDResponsibilityInventoryMatchesSource`(Z 소유). parent가 RED로 두었던 `TestMCPTraceBindingObservesDirectSQLSpansAndClearsCorrelation`은 **현재 트리에서 PASS**다(`mcp_trace_wiring.go`가 `mcp_facade.go:86`에 조립됨, `-run` 단독 실행 0.01s PASS). H 회귀가 아니라 M 조립이 트리에 들어온 결과다 |
| `go test -race -count=1` mcpcli, issueopsapp, adapter/{issueops,gates,looprun,worker}, outbound/{state,sqlstore,authority,issueopslease,issueopscompletion,issueopsrecord}, application/{looprun,worker,state,issueopsreplacement,authority} | `WARNING: DATA RACE` 0건. 위 golden 1건 외 모두 ok (adapter/issueops 220.7s) |

## 교정 5건 재감사

각 항목은 (a) 소스, (b) production 테스트, (c) 고친 줄만 되돌린 overlay 변이에서 RED, (d) 실제 바이너리 실측 네 가지로 확인했다.

| # | 결함 | 소스 | 테스트 GREEN | 변이 RED | 실제 바이너리 |
|---|---|---|---|---|---|
| 1 | gates `file` 생략 시 서버 cwd | `ForRequest`가 `newScopedGatesService(root, cwd)`(`mcp_facade.go:121`) → `WorkspaceFileStore{Root,CWD}`(`gates_wiring.go:13-15`). `name()`이 상대 경로를 `s.CWD` 기준으로 붙인다(`workspace_store.go:88-92`) | `TestMCPHTTPGatesDefaultFilesResolveAgainstRequestCWD` | `newScopedGatesService`를 `FileStore{}`로 되돌리면 "default ledger is not in the request workspace" | `gates_init` file 생략 → `repoA/.issueops/gates/default-probe.md` 생성, 서버 cwd `C` 항목 0. `gates_abandon` 생략 → `repoA/GATES.md`에 ABANDON 기록. `cwd=repoA/.issueops`로 생략하면 `openat .issueops/GATES.md`를 요청 cwd 기준으로 찾는다 |
| 2 | gates 경로의 workspace 이탈 | 모든 읽기·쓰기·discovery가 `os.OpenRoot(s.Root)` 안에서 일어난다(`workspace_store.go:19-83`). `name()`은 존재하는 가장 깊은 상위만 `EvalSymlinks`로 맞춰 `/tmp`↔`/private/tmp` 별칭을 허용하고(`:104-120`), `filepath.IsLocal` 실패면 "outside" 오류(`:97-100`) | `TestMCPHTTPGatesPathsStayInsideAuthorizedWorkspace` | 같은 변이에서 "absolute.md escaped the workspace" | 절대 경로, `../outside`, 서버 cwd, symlink 디렉터리 경유, symlink 아래 새 경로, repoB 경로, `/private` 별칭 외부, symlink 파일(`linked.md`) 8건 모두 `is outside the authorized workspace`. outside·C·repoB에 새 파일 없음. 내부 새 leaf(`new/dir/leaf.md`), 절대 내부, `/private` 별칭 내부는 생성. grant B로 repoA 경로 거부. `gates_status files=[outside/secret.md]`와 discovery(symlink 원장 포함) 모두 "outside secret" 미노출. `gates_abandon` outside → secret.md 원문 그대로 |
| 3 | MCP `BaseSync` 누락 | `MCPDependencies.BaseSync`(`mcp_tools.go:85`), `issueOpsMCPDependencies`가 `basesyncoutbound.NewInspector(basesyncoutbound.RunGit)` 주입(`mcp_facade.go:75`), `issueOpsExecutionActionDependencies`가 전달(`mcp_tool_issueops_execution.go:36`) | `TestMCPHTTPCompletedReplacePreviewUsesProductionBaseSync`(no-drift/drift), `TestMCPExecutionDependenciesMatchEveryCoreActionDependency` | H 보고의 RED 기록 인용. 이번에는 production 테스트 PASS와 소스 대조로 확인 | completed fixture는 만들지 않았다. 대신 active lease fixture에서 status/claim/release 전 action이 HTTP·stdio 모두 dispatch됨을 확인(아래 수명주기) |
| 4 | execution persist의 `Background` commit | `persistExecutionTransition*`·`writeIssueOps`·`deleteIssueOps`·`WriteIssueOps`가 ctx를 받고(`execution_state.go:55-59,127`, `issueops_state.go:141,184,189,244`) `db.CompareAndApply(ctx, …)`로 commit한다(`execution_state.go:117,144`, `issueops_state.go:159,210,213`). 호출자 전수(`rg`): replacement adapter, remote/body-sync/cycle store, sync-base 3곳, publication 모두 span ctx. `context.Background()`로 부르는 production 호출자는 CLI 전용 `review_mutation_store.go:21` 하나 | `TestSpanPersistRefusesCommitAfterRequestCancellation`, `TestSpanPersistKeepsCommittedWriteWhenCancelledAfterCommit`(replacement·cycle) | `execution_state.go:117`을 `Background`로 되돌리면 `…/cycle: cancelled request still committed its record`, `…/replacement: commit was not attributed to the request span (CommitCount:0 CommitCoverage:unknown)` | lease release/claim 실측이 요청 ctx로 commit됐다(`issueops execution status --json` readback `released`). lease·completion 저장소는 `WithSpan(ctx, func(spanCtx…))` 안에서 `store.Apply(spanCtx…)`(`outbound/issueopslease/sqlite.go:44-47,138,199`, `issueopscompletion/repository.go:34-37,106`) |
| 5 | loop·worker·state의 `Background` lock, lock 아래 grant 미확인 | `looprun.Service.Start/RecordAttempt/Stop(ctx…)`→`Store.WithLock(ctx…)`→`Write(spanCtx…)`→`db.Apply(ctx…)`(`application/looprun/service.go:33-109`, `adapter/looprun/store.go:78-103`). worker·state도 같다(`application/worker/service.go:29-162`, `application/state/service.go:38-233`). MCP handler는 요청 ctx를 넘긴다(`mcp_tool_loop.go`, `mcp_tool_assistant_worker.go:53,59,88`, `mcp_tool_policy_state.go:40,62`). workspace loop는 `grantFencedLoopStore`가 grant root span을 먼저 열어 guard를 그 lock 아래서 소비한다(`request_store_fence_wiring.go:16-29`) | `TestLoopStartCommitsInsideRequestSpan`, `TestWorkerEnqueueCommitsInsideRequestSpan`, `TestWorkerEnqueueRefusesCancelledRequest`, `TestStateWriteCommitsInsideRequestSpan`, `TestStateUpdateCancelledInsideSpanPersistsNothing`, `TestScopedLoopWriteRechecksGrantUnderLockAfterRotation`, `TestScopedLoopWriteLosesGrantRevokedWhileFenceIsHeld`, `TestMCPHTTPBoundLoopAndWorkerWritesSucceed` | `loop_wiring.go`에서 fence 줄을 지우면 bound loop_start 자체가 `revoked`로 실패(guard가 loop DB에서 grant를 찾지 못함). fence가 실제로 하중을 받는 코드임을 보여 준다 | 아래 "lock 아래 철회·취소" 실측. **단 worker는 결함 1** |

## 도구 분류와 root 결정 (소스·실측)

- `mcpToolAuthorities`(`mcp_tool_authority.go:63-126`)는 51개 전부를 명시한다. 실제 2026 `tools/list` 51개를 `authority_file` 노출 여부로 나누면 workspace 25개(`api_doc_*`, `atomic_commit_preflight`, `command_*` 3, `commit_suggest`, `gates_*` 5, `harness_inspect`, `issueops_execution`, `lint_diagnose`, `loop_*` 4, `project_docs_*` 5, `worker_run_read_only`), server 26개(`channel_*`, `commit_policy`, `contract_*`, `daemon_status`, `docs_index`, `self_*` 10, `skill_manifest`, `state_*` 6, `web_fetch_resilient`, `worker_enqueue/status/list/cancel`)로 contract §2 표와 일치한다. `validateHTTPToolClassification`(`:128-148`)은 catalog Tools와 Dispatch 양쪽 이름을 모으므로 annotation에 의존하지 않는다.
- `harness_inspect` 실측: 인자 없음 → `issueops_root=/Users/habin/workspace/issueops`(server), `repo`만 → `authority_required`, `repo`+grant → `target_repo=/private/tmp/h-verify2/run/repoA`.
- root 결정(`mcp_request_scope.go:32-61,63-86,91-136`)과 실측: `path=repoA`+`workspace_root=repoB` → "conflicts with another workspace root input"; 상대 `path` cwd 없음 → "must be absolute unless cwd is absolute"; `path=.`+`cwd=repoA` → 통과; `cwd=repoB`로 repoA → "cwd is outside workspace_root"; `/tmp`와 `/private/tmp` 혼용 → 통과(canonical 동일); record 도구에 `workspace_root=repoB` → "workspace root conflicts with record io-…"; grant B로 repoA record → "outside the authorized repository scope".
- `scopedArguments`(`:149-177`)는 `authority_file`을 지우고 root 인자·`workspace_root`·`cwd`를 scope 값으로 덮어쓴다. gates handler는 그 `workspace_root`/`cwd`/`files`만 읽으므로(`mcp_tool_gates.go:45-52`) 요청 인자가 서비스에 그대로 새지 않는다.
- `issueopsapp`·`mcpcli` production에 `os.Chdir` 0건, 전역 current actor/workspace 변수 없음. `os.Getwd`는 `apidoc_wiring.go:24`, `basic_wiring.go:16`, `project_docs_wiring.go:30`, `loop_wiring.go:21` 네 곳이며 HTTP 요청 경로에서는 `issueOpsMCPRecordRoots`의 loop 분기가 `newLoopService()`(→`newLoopIdentity()`→`os.Getwd`)를 만드는 것 하나다. `Status`는 `Store.Read`만 쓰므로 결과에 영향이 없지만, 요청 경로에서 Getwd를 호출하는 코드가 남아 있다(아래 후속).
- HTTP 서버 기본 의존성(`issueOpsMCPHTTPDependencies`, `mcp_facade.go:95-106`)은 DefaultTarget·Inspect·"Project docs route"를 `issueOpsRoot()`로 고정하고, `ForRequest`(`:110-125`)가 Commit·Lint·ProjectDocs·Bootstrap·Loop·Gates·Worker를 scope로 다시 만든다. per-session proxy는 없다. 테스트 클라이언트 2개와 실측 클라이언트 전부가 같은 PID(95492) 한 서버를 썼다.

## 권한·신원·context 경로 (소스)

- HTTP는 actor 필드 6종 중 하나라도 있으면 handler 전에 `authority_invalid`(`mcp_request_dispatch.go:54-60`). 실측: `session_id`, `host` 동봉 → 거부. server 도구(`docs_index`)에 `session_id` → closed schema `-32602 unknown_key`. gates 같은 workspace 도구에 `session_pid` → schema가 먼저 `unknown_key`로 막는다(혼합 검사 전에 schema 단계에서 거부).
- 파이프라인 순서는 `RequestScope → Credentials.Read → BindAuthority → ForRequest → dispatch`(`:68-88`)이고 dispatch는 반환된 `requestCtx`를 쓴다(`:30`). `ForRequest`는 `Caller`를 세팅하고(`:87`), execution handler는 `Caller`가 있으면 ancestry를 붙이지 않는다(`mcp_tool_issueops_execution.go:55-61,67-88`). `ObserveNativeProcessAncestry(os.Getpid())`는 stdio의 capability 없는 호출에서만 실행된다.
- 관리 credential 읽기 실측: bearer 파일, `grants/../grants/`, managed dir 밖 복사본, grants 안 symlink, 다른 digest 이름의 복사본, group-readable(0640)로 바꾼 정상 파일 모두 "not a readable managed credential". 0600 복구 후 통과.
- legacy port: `NewReviewMutationStore`는 `issueopscli_runtime_wiring.go:82,94,119`, `issueops_cleanup_wiring.go:110`, `planning_recorder_wiring.go:13`, `cycle_readiness_wiring.go:39`에서만 조립되고, 그 상위(`newPlanningRecorder`, `newCyclePhaseService`, `newGateReadiness`→`issueops_orphan_loopgate_wiring.go:16`)는 전부 CLI wiring이다. `mcpcli` production에서 `Publication.Create`·review store·artifact verification·issue create를 참조하는 곳은 0건(`rg`). `NativeActorVerifier()`의 production 사용은 `review_mutation_store.go:18`과 `execution_sync_base.go:61`(테스트 전용 진입)뿐이다. `mcp_tool_issueops.go:5-7`의 `Background` 핸들러는 호출자 0건인 죽은 코드다.
- publication은 `WithoutCancel(ctx)`를 유지한다(`execution_publication_store.go:19-21`, `TestRemotePublicationTransactionKeepsRequestGuardButIgnoresCancellation` PASS).
- 취소 의미: `PropagateRequestCancellation: true`(`mcp_http.go:67-72`). `commitData`는 commit 직전 `ctx.Err()`를 확인하고(`sqlstore.go:465-471`) span 밖 commit은 unattributed로 센다(`:472-475`).
- reconcile 저장소(G 소유)는 `Canonicalize`만 span을 열고 `ApplyReceipt`/`ClearIntent`/`RecordFailure`는 span 없이 raw CAS로 commit한다(`reconcile_repository.go:51,216,301`). 요청 ctx가 그대로 들어가므로 취소는 확인되지만 span 귀속은 unattributed다. HEAD 이전 설계이고 H 변경이 아니라 기록만 한다.

## 실제 바이너리 HTTP 실측

환경: `ISSUEOPS_STATE_DIR=/tmp/h-verify2/run/state`, `ISSUEOPS_ROOT=/Users/habin/workspace/issueops`, 서버 env `TRACEPARENT=00-9999…-01`, 서버 cwd `/tmp/h-verify2/run/C`(빈 디렉터리), `--addr 127.0.0.1:47961`. ready line `issueops mcp http ready url=http://127.0.0.1:47961/mcp pid=95492`. bearer `0600` 43바이트, `mcp-http` `0700`.
grant는 대화형 bash(pid 98327, `started_at 2026-10-02T13:48:41Z`, executable `bash`)에서 `issueops mcp authorize --json`으로 발급했다: A(codex/session-a→repoA), B(claude/session-b→repoB), BA(claude/session-b→repoA). 철회 실험 뒤 A는 같은 세션에서 재발급했다(A2).

### Guard

| 요청 | 결과 |
|---|---|
| bearer 없음 / 틀린 bearer | 401 + `WWW-Authenticate: Bearer realm="issueops"` |
| Host `localhost:47961` / `127.0.0.1` | 403 `forbidden host` |
| Origin `http://evil.example` / `http://127.0.0.1:47961/` / `https://127.0.0.1:47961` / `null` | 403 `forbidden origin` |
| Origin `http://127.0.0.1:47961` | 200 |
| path `/other`, `/mcp/` | 404 |
| GET / DELETE | 405 |
| 인증 없이 bad path / bad host | 404 / 403 (순서 path→Host→Origin→bearer, `mcp_http.go:89-98`) |
| body 정확히 4,194,304바이트 / 4,194,305바이트 | 200 / 413 `request body exceeds 4194304 bytes` |
| `Mcp-Protocol-Version: 1999-01-01` | 400 `Unsupported protocol version (supported versions: 2026-07-28,2025-11-25,2025-06-18,2025-03-26,2024-11-05)` |
| 2026: `_meta` 누락 / `Mcp-Method` 누락 / 불일치 | -32602 `missing or invalid _meta field` / -32020 `missing required Mcp-Method header` / -32020 `header mismatch` |
| 2026 정상(`_meta` protocolVersion+clientInfo+clientCapabilities, `Mcp-Method`) | 200, `resultType=complete`, tools 51 |
| 2025 legacy(header 없음) | 200 SSE |

### Workspace·신원

| 요청 | 결과 |
|---|---|
| A(2026)·B(2025) 동시 `atomic_commit_preflight` | `repo_root` 각각 repoA / repoB |
| capability 없음 | `authority_required` |
| 위 "root 결정" 표의 충돌·상대·cwd 6건 | 표대로 |
| actor 혼합 2건, 위조 credential 6건 | 전부 `authority_invalid` |
| 없는 record / 없는 loop | `issueops record io-none: file does not exist` / `loop loop-deadbeef: file does not exist` |
| `loop_start` A → `loop_record_attempt` A / B / `loop_status` BA | 성공 / `outside the authorized repository scope` / 성공(같은 repo의 다른 세션) |
| `worker_run_read_only` A, cwd=repoA / cwd=C / workspace_root=C | `succeeded` / `cwd is outside workspace_root` / `outside the authorized repository scope` |
| `command_fake_run` cwd 생략 | schema `missing_required /cwd`(기존 catalog 필수 필드) |

실측 전후 서버 cwd `C`는 항목 0개였다.

### lock 아래 철회·취소 (결정적 순서)

python이 `issueops_v1/issueops.lock.db`에 `BEGIN IMMEDIATE`를 잡은 뒤 요청을 보냈고, 독립된 server 도구 호출(`docs_index`)이 완료된 시점을 "요청이 서버에 도착함"의 기준으로 썼다. sleep은 쓰지 않았다.

| 순서 | 결과 |
|---|---|
| 2026 `loop_record_attempt`(A) 대기 중 client abort | `tool=loop_record_attempt outcome=tool_error duration_ms=311`, lock 해제 전 종료, 기록 없음 |
| 2025 `loop_record_attempt`(A) 대기 → grant A row 삭제(`REVOKED 1`) → 해제 전 미완료 확인 → lock 해제 | `authority_invalid: authority credential was revoked or never issued`, attempts 없음 |
| 이후 이전 A 파일 | Bind에서 `revoked or never issued` |
| 별도 loop에서 2026 abort / 2025 abort(모두 대기 중) → lock 해제 → `loop_stop`으로 직렬화 | 2026은 해제 전 `tool_error`·기록 없음, 2025는 해제 뒤 `outcome=ok`로 끝까지 처리돼 attempt `["abort 2025-11-25"]` 기록 |

2026은 disconnect가 handler ctx 취소로 전파되고, 2025는 전파되지 않는다는 SDK 의미가 실제 바이너리에서 그대로 보인다. `web_fetch_resilient`는 loopback을 `safety_rejected`로 막아 로컬 hold 서버로는 재현할 수 없었고, 외부 지연 HTTP는 쓰지 않았다.

### 수명주기 (record `io-bce2bdadca53`, repoA, worktree `repoA-300-http-lease`, gen 1 active, holder codex/session-a/pid 98327)

| 요청 | 결과 |
|---|---|
| status, grant B / A2+`workspace_root=repoB` / A2 생략 / A2 cwd=worktree | scope 거부 / "conflicts with record" / `active gen 1` / 같음 |
| release gen1 grant BA / gen2 grant A2 / gen1 A2 cwd=repoA / gen1 A2 + `session_id` | `only the current holder may release generation 1` / `… generation 2` / `release cwd must be the canonical worktree` / `authority_invalid`(혼합) |
| claim gen1 grant A2(현재 holder, `claim_token_file=../outside/token`) | 멱등 성공(active gen 1 유지), token 파일을 읽지 않았고 outside에 파일 없음 |
| claim gen1 grant BA | core 거부: `lease is not claimable at generation 1 … must be reseeded` |
| release gen1 grant A2 cwd=worktree | `ok`, HTTP status와 `issueops execution status --json` readback 모두 `generation 1 released` |

### trace

| 요청 | 서버 stderr `issueops_record_span` 이벤트 |
|---|---|
| `loop_stop`(이미 종료된 loop → span 안 오류) traceparent `00-aaaa…-01`(2026)와 `00-bbbb…-01`(2025) 동시 | 각 요청당 2건(grant fence span + loop span)이 자기 trace_id만 가진다: `trace_id="bbbb…" parent="bbbb…"` ×2, `trace_id="aaaa…" parent="aaaa…"` ×2 |
| header 없음 | `trace_id`/`parent_id` 필드 없음(서버 env `9999…` 미사용) |
| `traceparent: SECRET-TRACE-VALUE-123` | `trace_id` 없음 + `mcp_http_trace … invalid_traceparent_ignored` 1건, 원문 0건 |

M 조립이 트리에 있으므로 HTTP 요청의 traceparent가 실제 SQL span 이벤트에 실린다. 이전 보고의 "관측 불가" 보류는 해소됐다.

### 명령·수명

| 명령 | 결과 |
|---|---|
| `mcp service status --json` / `start` / `stop --json` | `{"ok":false,"status":"","pid":0,"build_id":"","url":"http://127.0.0.1:47831/mcp","error_code":"supervisor_unavailable"}`, exit 1. running·stopped 어느 쪽도 주장하지 않는다 |
| 같은 포트 두 번째 `mcp --http` | `127.0.0.1:47961 is unavailable (conflict): … address already in use`, exit 1 |
| `--addr 0.0.0.0:47999` | `must use a loopback IP literal`, exit 1 |
| `mcp --addr …`(`--http` 없음) | `use --http to serve HTTP, or no flags for stdio`, exit 1 |
| SIGTERM: lock에 막힌 in-flight `loop_record_attempt`가 있는 상태에서 `kill -TERM 95492` | 새 연결 즉시 refused, lock 해제 뒤 in-flight 요청은 200으로 완료(attempt `survived drain` 기록), 서버 exit 0 |

### 로그 위생

서버 stderr 235줄: bearer 0, `grants/` 0, `repoA|repoB` 0, `Authorization|Bearer` 0, 64자 hex 0, env trace `9999999999` 0, `SECRET` 0, record id 0, `/tmp/h-verify2` 0. 이벤트 종류는 ready 1, `mcp_http_request` 121, `mcp_http_tool` 94, `mcp_http_trace` 2, `issueops_record_span` 17뿐이다.

## 실제 바이너리 stdio 회귀

cwd `/tmp/h-verify2/run/stdio`, `issueops mcp`에 11개 메시지를 한 번에 보냈다. exit 0. initialize 2025-11-25 협상, tools/list 51개(`atomic_commit_preflight`에 `authority_file` 노출), `docs_index` 성공, capability 없는 native `atomic_commit_preflight`·`issueops_execution status` 성공, capability+status 성공, capability+불일치 actor(`host=claude session_id=session-zzz`) release → `authority_invalid: supplied actor does not match the authorized caller`, grant B로 repoA preflight → scope 거부, capability+일치 actor(`codex/session-a`) release → released. `gates_init` file 생략은 host 프로세스 cwd(`stdio/.issueops/gates/stdio-native.md`)에 생성됐다. stdio는 host의 cwd라 기존 동작이며 HTTP 결함 1의 수정이 stdio native 경로를 바꾸지 않았음을 보여 준다.

## 결함 상세

### 결함 1: worker span의 guard pass-through

- `issueOpsMCPRequestDependencies`는 workspace 요청의 worker를 `workerapp.Service{Effects: unboundWorkerStore{…}}`로 바꾼다(`mcp_facade.go:122`). `unboundWorkerStore.WithLock`은 `sqlstore.WithRecordGuard(ctx, passThroughRecordGuard)`로 **같은 context key를 다시 써** Bind가 건 guard를 대체한다(`request_store_fence_wiring.go:36-42`, `record_guard.go:23-28`). worker DB(`<state>/worker`)에는 grant bucket이 없어 BindSpan이 실패하므로 producer가 의도적으로 뺐다는 주석이 있다.
- 재현(overlay로 `issueopsapp`에 추가한 `TestProbeWorkerWriteAfterGrantRevokedPostBind`, repo에 남기지 않음): grant 발급 → `bindIssueOpsAuthority`로 Bind 성공 → `ForRequest`로 request deps 생성 → grant row 삭제 → 같은 ctx로 `deps.Loop.Start` 와 `deps.Worker.RunReadOnly` 호출. 출력: `loop_start after revoke: err=authority_invalid: authority credential was revoked or never issued`, `worker_run_read_only after revoke: err=<nil> status=succeeded`. 명령이 실행되고 job이 `succeeded`로 기록됐다.
- 어긴 조항: §2 "BindSpan은 Check로 재검증하고 … rotation과 write는 같은 lock으로 직렬화한다", "Verify는 이 context가 있으면 매번 grant를 검증". parent 지시의 "loop/worker/state request context plus locked grant revocation" 중 worker가 빠졌다.
- 수정 방향: loop와 같은 패턴으로 `grantFencedWorkerStore`가 IssueOps grant root span을 먼저 잡고 그 안에서 worker `WithLock`을 열면 guard가 grant root reader로 소비된다. `RunReadOnly`의 마지막 finish 쓰기는 `WithoutCancel` 의미를 유지하되 fence는 같은 방식으로 적용할 수 있다. 명령 실행(`Effects.Run`) 자체는 lock 밖이므로 fence는 "job 기록" 시점의 재확인까지만 보장한다. gates CHECK·`command_fake_run`도 store lock이 없어 Bind 뒤 철회를 재확인하지 않지만, contract가 이들에 lock을 요구하지 않으므로 결함으로 세지 않고 아래 후속에 둔다.

## 보류·후속 (통과로 기록하지 않음)

- supervisor(LaunchAgent/systemd), `<state>/mcp-http/` OS file lock, PID+started_at+executable+build_id 대조 stop, install/update 순서, host config merge, `--mcp-transport=stdio`, dry-run 무쓰기: 구현 없음(J). `mcp service`는 주입된 port만 부른다는 사실만 확인했다.
- 세 host(codex/claude/omo)가 같은 서버 PID에 직접 연결하는지: 코드상 proxy는 없고 실측 클라이언트 전부가 pid 95492 하나를 썼지만, 실제 host 연결은 K/J의 receipt로만 확인할 수 있다.
- `ISSUEOPS_ROOT` 미설정 시 `issueOpsRoot()`(`misc_facade.go:27`)가 cwd로 fallback하는 점은 unit(J)이 env를 명시해야 한다. 이번 실측은 env를 명시했다.
- `issueOpsMCPRecordRoots`의 loop 분기가 `newLoopService()`를 통해 `os.Getwd()`를 호출한다(`loop_wiring.go:21`). 결과에는 쓰이지 않지만 contract의 "process Getwd 금지"를 글자대로 지키려면 `app.Reader{Store: newLoopStore()}` 같은 cwd 없는 reader로 바꾸는 편이 맞다.
- `claim_token_file`·`issue_snapshot_file`·`verification_report_path`·`api_doc_review`의 `diff_file`/`prompt_file`은 절대 경로를 그대로 통과시킨다(`mcp_request_scope.go:179-184`). claim token은 core가 저장된 digest와 대조하므로 임의 파일 읽기의 효과는 token 불일치뿐이고, 이번 실측에서 외부에 쓰인 파일은 없었다. gates처럼 `os.Root`로 묶지는 않았으므로 family 간 일관성은 L/Z 결정 사항이다.
- `worker.RunReadOnly`의 finish 쓰기(`application/worker/service.go:105`)와 gates CHECK/`policy.Run`(`adapter/policy/policy_run.go:54`)의 `Background` timeout은 H 보고가 명시한 비전파 경계이며 그대로다.
- `generated_command_provenance.go:14,65`의 `provenanceapp.Bind(context.Background()…)`는 next command 서명 바인딩이며 DB·lock을 열지 않는다. 기록만 한다.
- `withAuthorityFields`가 실제 tools/list에 더한 3개 필드와 `mcp_tools.golden.json` 불일치, `TestResponseContractsGolden`(B), `TestDDDResponsibilityInventoryMatchesSource`(Z)는 그대로 남아 있다.
- stdio 응답은 요청 순서와 다르게 돌아온다(id 10이 먼저). SDK가 요청을 동시 처리하는 기존 동작이며 H 변경이 아니다.

## 검증 시점 파일 해시 (sha256 앞 12자)

`mcp_tool_authority.go 75c0c8596b40`, `mcp_request_scope.go f2d1b207766d`, `mcp_request_dispatch.go a166aa7ea3df`, `mcp_http.go f862a46b25f7`, `mcp_http_bearer.go b78d0f92322f`, `mcp_sdk_server.go 8231ef375d03`, `mcp_tools.go a8d943bf02c6`, `mcp_tool_issueops_execution.go 26b14a92c40b`, `mcp_facade.go 890ada599b13`, `mcp_http_command.go 964b4f1b9487`, `mcp_service_command.go 4c0368ceda2e`, `request_store_fence_wiring.go d2685507a248`, `gates_wiring.go b5604c0f5468`, `loop_wiring.go 9ff92fb84f0d`, `mcp_trace_wiring.go fe41f3a0185a`, `adapter/gates/workspace_store.go ff9bcca5de7d`, `execution_state.go 5762a3e3b281`, `issueops_state.go df4ae00d9a22`, `execution_publication_store.go 5faa58e120b2`, `sqlstore.go b9cb124f0113`, `record_guard.go db28520ab881`, `application/looprun/service.go 5f324f1c6b44`, `application/worker/service.go 9e5a3557f8eb`, `application/state/service.go 9649c16128b9`.

## 정리

실험 산출물은 `/tmp/h-verify2`에만 있다. 서버는 SIGTERM으로 exit 0 종료했고, grant용 셸 세션과 lock 보유 python은 종료했다. `git status`에 이 문서 외 내 변경은 없고 `cmd/hseed`·`cmd/hreceipt`는 repo에 없다.
