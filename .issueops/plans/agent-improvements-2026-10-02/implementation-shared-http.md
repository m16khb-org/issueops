# H 구현 보고: 공용 Streamable HTTP MCP와 요청 단위 권한·scope

범위: contract.md §2(요청 권한, 모든 도구 family의 workspace 경계), §3(transport·수명 중 H 몫), §5 H 행, §6 acceptance 중 H가 검증할 수 있는 부분이다.
supervisor·설치·host 설정(J), inspect 진단(K), 구조화 결과(L), usage 관측 배선(M), golden·inventory·운영 문서(Z)는 구현하지 않았다.
commit, 전역 설치, `$HOME` 설정 변경은 하지 않았다. 검증은 모두 `/tmp` 아래 격리 state·repo·port에서 했다.

## 결과 요약

- `issueops mcp --http [--addr 127.0.0.1:PORT]`가 실제로 동작한다. 기본값은 `127.0.0.1:47831`의 단일 `/mcp`이다.
  SDK v1.8.0 `StreamableHTTPOptions{Stateless:true, JSONResponse:false, PropagateRequestCancellation:true, MaxRequestBodyBytes:4MiB}`를 쓰고 MCPGODEBUG는 건드리지 않는다.
- 모든 HTTP 요청은 `<state>/mcp-http/bearer`(0600, 상위 0700)의 256-bit bearer를 요구한다. 인증 실패는 401과 `WWW-Authenticate`, Host가 정확한 `addr:port`가 아니거나 Origin이 서비스 origin과 다르면 403, 4MiB 초과는 413, 다른 path는 404, GET은 405다.
- workspace 도구는 `RequestScope → CredentialFiles.Read → bindIssueOpsAuthority → ForRequest → dispatch` 순서로 처리하고, dispatch는 반환된 context를 그대로 쓴다.
- 모든 도구를 `mcpToolAuthorities`에 명시적으로 분류했다. 분류되지 않은 도구가 catalog나 dispatch map에 있으면 `NewHTTPHandler`가 실패한다.
- `issueops_execution`은 prepare/status/claim/release/replace(preview·revoke·finalize-preview·finalize·reseed)/resume/reconcile/complete를 HTTP에서도 그대로 dispatch한다. capability가 바인딩된 요청에는 서버 PID ancestry를 붙이지 않는다.
- `issueops mcp service start|stop|status [--json]`은 주입된 `port.MCPService`만 호출한다. 기본 구현은 supervisor가 없다는 사실을 `error_code=supervisor_unavailable`과 non-zero exit로 정직하게 알리고, running·stopped 중 어느 상태도 주장하지 않는다. 실제 supervisor는 J가 `newMCPService()`를 교체해 연결한다.
- I9: HTTP는 이 요청의 `traceparent` header만 `issueopsrecord.WithTraceparent`로 바인딩한다. 서버 `TRACEPARENT` env는 읽지 않고, 잘못된 값이면 원문 없이 `invalid_traceparent_ignored` 경고만 남긴다. CLI는 IssueOps CLI runtime을 조립할 때 `TRACEPARENT`를 한 번 파싱해 observer에 묶는다.

## 변경 파일

| 경로 | 내용 |
|---|---|
| `cmd/issueops/mcpcli/mcp_tool_authority.go` (신규) | 도구별 권한 분류표(server/workspace, root 인자, 요청 상대 경로 인자, record 종류), HTTP 등록용 미분류 검사, workspace 도구 schema에 `authority_file`/`workspace_root`/`cwd` 선택 필드 추가(맵 복사) |
| `cmd/issueops/mcpcli/mcp_request_scope.go` (신규) | `NewRequestScope`: 요청 인자와 record에서만 root를 정하고 충돌을 거부한다. 요청 상대 경로를 scope cwd 기준 절대 경로로 바꾼다 |
| `cmd/issueops/mcpcli/mcp_request_dispatch.go` (신규) | transport별 capability 요구, HTTP의 actor 필드 혼합 거부, 권한 파이프라인, `authority_required`/`authority_invalid` 오류 payload |
| `cmd/issueops/mcpcli/mcp_http.go` (신규) | Host·Origin·bearer·body guard, SDK stateless handler, 허용 목록만 남기는 access log, 요청 header 기반 trace 바인딩, 10초 drain 뒤 취소하는 `ServeHTTP` |
| `cmd/issueops/mcpcli/mcp_http_bearer.go` (신규) | `EnsureHTTPBearer`: temp 파일을 완성한 뒤 no-clobber link로 게시한다. symlink·group/world 권한·형식 오류를 거부한다 |
| `cmd/issueops/mcpcli/mcp_sdk_server.go`, `mcp_tools.go`, `mcp_tool_assistant_worker.go`, `mcp_tool_issueops_execution.go` | `resolveHandlerGroup`이 `func(context.Context, MCPToolCall)`을 반환하고 모든 도구가 같은 dispatch를 탄다. `MCPDependencies`에 `RequestScope/Credentials/BindAuthority/ForRequest/BindTrace/Caller`를 추가했다. web fetch가 요청 ctx를 쓴다. execution은 `Caller`가 있으면 verified identity를 actor로 쓰고 ancestry를 비운다 |
| `cmd/issueops/issueopsapp/mcp_facade.go` | 권한 의존성 조립, `issueOpsMCPHTTPDependencies`(server 기본값을 설치 root로 고정), `issueOpsMCPRequestDependencies`(scope로 service 재조립), record root 조회 |
| `cmd/issueops/issueopsapp/mcp_command_facade.go`, `mcp_http_command.go` (신규), `mcp_service_command.go` (신규) | `--http`, `--addr`, SIGINT/SIGTERM, 포트 충돌 시 임의 포트로 바꾸지 않고 `conflict` 오류로 실패, ready line(`url`, `pid`만 출력), service CLI |
| `cmd/issueops/issueopsapp/{project_docs,assistant,projectbootstrap,loop}_wiring.go`, `app.go` | 상대 root를 process cwd 대신 명시적 base로 해석하는 scoped 생성자 추가(기존 생성자는 그대로 위임) |
| `cmd/issueops/issueopsapp/issueops_record_store_wiring.go`, `issueopscli_runtime_wiring.go` | CLI `TRACEPARENT` 1회 바인딩 observer |
| `internal/adapter/issueops/execution_publication_store.go` | `WithinTransaction`이 `context.Background()` 대신 `context.WithoutCancel(ctx)`를 쓴다. 취소 무시는 유지하면서 record guard와 trace 값은 보존한다 |

테스트: `mcp_http_helpers_test.go`, `mcp_http_test.go`, `mcp_request_scope_test.go`(mcpcli), `mcp_http_command_test.go`, `mcp_service_command_test.go`(issueopsapp), `execution_publication_store_guard_test.go`(adapter/issueops). 기존 테스트 3곳은 새 시그니처에 맞춰 호출만 고쳤다.

## 계약 대응

### 도구 분류 (`mcpToolAuthorities`)

- server(bearer만): `commit_policy`, `skill_manifest`, `docs_index`, `state_*` 6종, `channel_*`, `daemon_status`, `contract_schema/check`, `web_fetch_resilient`, `worker_enqueue/status/list/cancel`, self_verify·self_augment 계열 10종(alias 포함).
- workspace(capability + scope): `atomic_commit_preflight`(path), `project_docs_*` 5종·`api_doc_*`·`commit_suggest`·`lint_diagnose`·`loop_start`(repo), `command_policy_*` 3종·`worker_run_read_only`·`gates_*` 5종(workspace_root), `issueops_execution`(record), `loop_record_attempt/status/stop`(loop record).
- `harness_inspect`는 repo·workspace_root·authority_file 중 하나라도 있으면 workspace, 없으면 설치 root를 쓰는 server 호출이다.
- 요청 상대 경로: `api_doc_review`의 diff_file·prompt_file, gates의 file·files, execution의 claim_token_file·issue_snapshot_file·verification_report_path. `rel_path`와 `result_file`은 기존처럼 repo 기준이라 바꾸지 않았다.

### root 결정

- 명시적 `workspace_root`와 도구의 root 인자는 canonical 경로가 같아야 한다. 다르면 `authority_invalid`다. 상대 root는 절대 cwd가 있을 때만 허용한다.
- record 도구는 record가 기록한 root(Repo, WorktreePath, Execution.Workspace.SourceRoot/Root) 안에서만 고른다. 명시 root는 그중 하나여야 하고, 없으면 cwd를 포함하는 가장 깊은 root, cwd도 없으면 source repo를 쓴다. grant의 common-dir 대조는 G의 `Bind`가 한다.
- cwd를 생략하면 검증된 root로 채우고, 상대 cwd는 root 기준으로 붙인다. 최종 검증은 G의 `ScopeResolver`다. `os.Chdir`, process Getwd/env fallback, 전역 actor·workspace 변수는 없다.
- HTTP 서버 기본 의존성은 `DefaultTarget`, 기본 inspect target, "Project docs route" resource를 설치 root로 고정한다. workspace 도구는 매 요청 `ForRequest`가 commit/lint/project docs/bootstrap/loop/inspect를 scope 값으로 새로 만든다.

### 권한 혼합과 신원

- HTTP: `host/session_id/agent_id/session_pid/session_started_at/session_executable` 중 하나라도 있으면 handler 호출 전에 `authority_invalid`로 거부한다.
- stdio: `authority_file`이 없으면 기존 native ancestry 경로를 그대로 쓴다. 있으면 같은 파이프라인을 타고, actor 필드를 생략하면 verified identity를 쓴다. 필드를 보내면 ancestry 없이 그대로 core에 넘기며, 일치 여부는 G의 `Verify`(`MatchIdentity`)가 판정한다. 읽기 전용 status처럼 verifier를 호출하지 않는 action은 identity를 쓰지 않는다.
- grant는 신원 증명일 뿐이다. claim token, holder, generation, canonical cwd, confirm, fingerprint, CAS는 기존 core 판정 그대로이며, 실제 HTTP e2e에서 다른 holder와 stale generation이 거부되는 것을 확인했다.

### legacy ctx 경로

- MCP의 issueops family는 `issueops_execution` 하나다. G가 남긴 `NewReviewMutationStore`(feedback·domain review·ai-slop·planning·evidence recorder)와 issue-create intent는 CLI 전용 wiring에서만 호출된다. execution 계열 application 패키지를 grep한 결과 MCP에서 도달하지 않으므로 이번에는 바꾸지 않았다. 해당 store를 MCP에 노출하려면 그때 ctx·verifier를 넣어야 한다.
- HTTP에서 도달하는 ctx 손실 경로는 처음 보고에서 publication `WithinTransaction` 하나로 적었지만 독립 검증에서 더 나왔다. persist 헬퍼와 loop·worker·state lock도 같은 문제였으며, 아래 "교정 증분"에서 모두 고쳤다. publication은 `WithoutCancel(ctx)`를 유지한다.
- artifact verification의 `observe` callback은 CLI `remote` 명령에서만 조립된다. MCP에서는 도달하지 않으므로 HTTP에 서버 ancestry가 들어갈 경로가 없다.

### 취소 의미

- 2026-07-28: client가 연결을 끊으면 handler ctx가 취소된다(`PropagateRequestCancellation`).
- 2025-11-25(stateless legacy): 서버가 disconnect를 관측한 뒤에도 handler ctx는 취소되지 않는다. stateless에는 session이 없어 `notifications/cancelled`도 전달할 수 없다. SDK 의미 그대로이며 테스트가 두 동작을 모두 고정한다.
- stdio의 initialize·`notifications/cancelled` 계약은 바뀌지 않았다. 기존 `TestMCPRevisionStdio*`가 그대로 통과한다.
- request ctx는 execution handler와 web fetch에 더해, 교정 증분 이후 execution persist commit, loop·worker·state lock과 write까지 전달된다. gates·policy·project docs·API doc service는 여전히 ctx 인자가 없다. 이 도구들은 handler 진입 전 권한 검사까지만 취소를 따른다(교정 증분의 "전파하지 않는 경계" 참고).

## 검증 증거

### RED → GREEN (overlay 변이)

각 성질을 production 코드에서 한 줄씩 무력화한 사본을 `go test -overlay`로 실행했다. 12건 모두 대상 테스트가 RED였고, 원본은 GREEN이다.

| 변이 | RED 테스트 |
|---|---|
| HTTP workspace capability 요구 제거 | TestHTTPWorkspaceToolsRequireCapabilityAndRejectActorFields |
| HTTP actor 필드 혼합 허용 | 같은 테스트 |
| capability 요청에 서버 ancestry 부착 | TestHTTPExecutionActionsUseVerifiedCallerWithoutServerAncestry |
| `PropagateRequestCancellation=false` | TestHTTPDisconnectCancellationFollowsNegotiatedRevision |
| Host 검사 제거 | TestHTTPGuardEnforcesBearerHostOriginPathAndBodyLimit |
| Origin 검사 제거 | 같은 테스트 |
| 미분류 도구 허용 | TestHTTPRegistrationRejectsUnclassifiedTool |
| trace를 서버 `TRACEPARENT` env에서 읽음 | TestHTTPConcurrentClientsKeepCallerScopeAndTraceIsolated |
| cwd를 scope로 재작성하지 않음 | TestScopedArgumentsRewriteRootsCWDAndRelativeFiles, TestHTTPExecutionActions… |
| publication이 요청 ctx를 버림 | TestRemotePublicationTransactionKeepsRequestGuardButIgnoresCancellation |
| CLI trace가 요청 correlation을 덮어씀 | TestCLIRecordObserverBindsTraceparentOnceAndKeepsRequestCorrelation |
| record root 선택이 cwd를 무시 | TestRequestScopeResolvesOnlyFromRequestAndRecord |

### 결정적 테스트

- 동시성 테스트는 두 요청이 모두 handler에 들어온 사실을 channel로 먼저 받은 뒤에만 release하고, bounded context timeout을 둔다. sleep·polling은 없다. disconnect 테스트는 HTTP request context 종료를 별도 channel로 관측한 뒤 handler ctx를 판정한다.
- 실제 composition e2e(`TestMCPHTTPServesTwoClientsWithRequestScopedAuthority`): 실제 `serveMCPHTTP`, `mcp authorize` 명령, 실제 authority Service·sqlstore, SDK `StreamableClientTransport` 클라이언트 2개를 쓴다. 다음 항목을 확인했다.
  - 두 클라이언트가 서로 다른 repo를 동시에 preflight하면 각자 자기 repo만 보인다.
  - capability 누락은 required, 다른 repo는 invalid, actor 필드 혼합은 invalid, grant scope 밖의 record는 invalid다.
  - 다른 holder의 release는 `holder` 오류, stale generation 2는 거부되고, holder가 generation 1을 실제로 release하면 저장된 lease가 비활성으로 바뀐다(readback).
  - 재발급 뒤 이전 credential은 invalid, 새 credential은 통과한다. 관리 경로가 아닌 파일은 invalid다.
  - access log에 bearer·token·`grants/`·repo 경로가 없다.
- `TestMCPHTTPRejectsExpiredCapability`: 실제 grant에 13시간 뒤 clock을 쓰는 authority Service로 Bind해 HTTP에서 `expired`를 확인했다.
- `TestMCPHTTPForegroundRejectsNonLoopbackAndBusyAddress`: `0.0.0.0`은 거부되고, 사용 중인 포트는 `conflict`로 실패한다.

### 명령 결과

| 명령 | 결과 |
|---|---|
| `go build ./...`, `go vet ./cmd/... ./internal/adapter/issueops/` | 출력 없음, exit 0 |
| `gofmt -l`(변경·신규 Go 29개), `git diff --check` | 출력 없음 |
| `go test ./cmd/issueops/mcpcli/ -count=1` | ok |
| `go test ./... -count=1` | 실패 2건만 있다. `issueopsapp` TestResponseContractsGolden(`$.cli.self_verify_compare…reused_count`, B 변경, G 검증 보고에도 기록됨)과 `internal/architecture` TestDDDResponsibilityInventoryMatchesSource(inventory 미등록, Z 소유). 그 밖의 패키지는 모두 ok이고 `contractgolden`도 ok다 |
| `go test -race -count=1` mcpcli, issueopsapp, adapter/issueops, application/authority, outbound/authority, sqlstore, issueopsrecord | 같은 golden 1건 외 모두 ok, `WARNING: DATA RACE` 0건 |
| `go build -o /tmp/h-bin/issueops ./cmd/issueops` | exit 0 |

### 실제 바이너리 HTTP (격리 `ISSUEOPS_STATE_DIR=/tmp/h-e2e/state`, `127.0.0.1:47931`, 서버 cwd는 빈 디렉터리)

ready line: `issueops mcp http ready url=http://127.0.0.1:47931/mcp pid=14645`.
grant 2개는 실제 `issueops mcp authorize`로 발급했다(session process는 스크립트 셸).

| 요청 | 결과 |
|---|---|
| bearer 없음 / 틀린 bearer | 401 / 401 |
| Host `localhost:47931` / Origin `http://evil.example` | 403 / 403 |
| 4MiB 초과 body / GET | 413 / 405 |
| 인증된 tools/list | 200 |
| client A(2026, curl)·client B(2025, curl) 동시 preflight | 각자 `repo_root`가 repoA, repoB로 섞이지 않음 |
| A의 `gates_init`(상대 file) | repoA에 ledger 생성, 서버 cwd 항목 0개 |
| B의 `loop_start` | `loop-2935b6e5ff22` 생성 |
| capability 누락 / 다른 repo / actor 혼합 / 위조 경로 | authority_required / authority_invalid ×3 |
| 재발급 뒤 이전 / 새 credential | authority_invalid "revoked or never issued" / 통과 |
| 없는 record status | tool error "issueops record io-missing: file does not exist" |
| SIGTERM | drain 뒤 exit 0, 로그 29줄에 bearer·token·`grants/`·repo 경로 없음 |

### 실제 바이너리 stdio 회귀

`issueops mcp`(stdio)에 한 번에 보낸 결과: initialize 2025-11-25 협상, tools/list 51개(workspace 도구에 `authority_file` 노출), 2026 metadata tools/list `resultType=complete`, `docs_index` 성공, capability 없는 native `atomic_commit_preflight` 성공, exit 0.

## 결정·편차

- 분류표와 schema 필드 추가는 mcpcli 서버 생성 시점에 한다(`withAuthorityFields`). inbound catalog `Build()`와 `cmd/issueops/testdata/mcp_tools.golden.json`은 바꾸지 않았으므로, 실제 tools/list에 세 선택 필드가 더 있다. 필드를 catalog assembly로 옮길지, golden을 갱신할지는 L/Z가 정한다.
- record root를 찾으려고 Bind 전에 record를 읽는다. bearer를 가진 같은 OS 사용자에게 record 존재 여부가 드러나며, contract가 정한 신뢰 경계 안의 일이다.
- foreground 단일 인스턴스는 포트 bind로 보장한다. `<state>/mcp-http/` OS file lock과 PID+started_at+executable+build_id 대조 stop은 supervisor(J) 몫으로 남겼다.
- bearer는 foreground 시작 시 없으면 만든다. installer는 같은 `mcpcli.EnsureHTTPBearer`를 재사용해 host 설정에 주입하면 된다.
- trace correlation은 ctx에 바인딩되지만, 현재 SQL span observer는 IssueOps CLI runtime에만 조립되어 있다. MCP/HTTP 경로의 lease·completion store는 observer 없이 sqlstore를 직접 열기 때문에 아직 trace 이벤트가 나오지 않는다.

## 남은 통합 작업

- **J**
  - `newMCPService()`를 LaunchAgent `io.issueops.mcp`·systemd user unit 기반 실제 supervisor adapter로 교체한다. 범위는 OS file lock, stale/conflict 판정, build_id, 인증된 MCP 호출 readiness다.
  - install/update/bootstrap 순서(credential·unit → build → stop → 교체 → start → build_id·MCP 확인 → host config merge)와 codex/claude/omo HTTP entry(bearer 주입, 0600)를 구현한다.
  - `--mcp-transport=stdio` 선택지와 dry-run 무쓰기를 구현한다.
  - host가 세션 시작 시 `issueops mcp authorize`를 실행하고 `authority_file`을 넘기도록 skill·hook 안내를 추가한다.
- **K**: inspect의 HostIntegration에 transport=http와 실제 host receipt(discovered/connected/protocol)를 반영한다. 세 host가 같은 서버 PID에 직접 연결하는지는 K/J 단계의 실제 host 실행으로 확인해야 하며, 이번 보고는 host 연결을 주장하지 않는다.
- **L**: `mcp_sdk_server.go`를 넘긴다. OutputSchema·annotations·structuredContent 작업은 `newSDKServer`/`registerAllTools`에 붙이면 된다. `withAuthorityFields`를 catalog assembly로 옮길지 결정해야 한다.
- **M**: MCP/HTTP execution 경로에 `issueopsrecord` observer를 조립해 요청 traceparent를 실제 span 이벤트로 내보낸다. HTTP 바인딩은 이미 되어 있다. usage/trace 입력도 M 몫이다.
- **Z**
  - `TestDDDResponsibilityInventoryMatchesSource`에 신규 파일·심볼을 등록한다. mcpcli 5개, issueopsapp `mcp_http_command.go`·`mcp_service_command.go`, scoped 생성자들이 대상이다.
  - B의 golden(`reused_count`)을 갱신하고, tools/list schema 변경을 golden에 반영할지 정한다.
  - root usage 문자열에 `mcp --http`·`mcp service`·`mcp authorize`를 추가하고, in-process-only ADR과 운영 문서를 공용 서버 방향으로 갱신한다.
- 후속 정리: CLI 전용 legacy review store와 issue-create intent의 ctx·verifier 마이그레이션은 MCP에 노출할 때 필요하다. loop·worker·state의 취소 전파는 교정 증분에서 끝냈다. gates·policy·project docs·API doc의 명령 실행과 파일 쓰기 취소는 application signature 변경이 필요해 남겨 두었다.
- Z inventory: 교정 증분에서 생긴 신규 production 파일(`internal/adapter/gates/workspace_store.go`, `cmd/issueops/issueopsapp/request_store_fence_wiring.go`)과 바뀐 시그니처도 `TestDDDResponsibilityInventoryMatchesSource`에 등록해야 한다.


## 교정 증분: 독립 검증 결함 5건 대응

`shared-http-verification.md`가 지적한 결함 3건과 parent가 추가로 지시한 context 결함 2건을 고쳤다. 결함마다 먼저 재현 테스트로 RED를 확인했고, 고친 뒤 GREEN과 실제 바이너리 실측으로 다시 확인했다. 기존 HTTP·stdio 동작과 G의 guard, parent의 nil-verifier 수정은 그대로 두었다. parent 소유인 `issueopsrecord` observer 파일과 `mcp_trace_observer_test.go`, golden·inventory·운영 문서, host 설치는 건드리지 않았다. commit은 만들지 않았다.

### 결함별 변경

| # | 결함 | 수정 |
|---|---|---|
| 1 | `gates_init`/`gates_abandon`에서 `file`을 생략하면 기본값이 서버 cwd 기준으로 풀렸다 | `ForRequest`가 gates service를 `newScopedGatesService(root, cwd)`로 다시 만든다. 이 store는 상대 경로를 검증된 요청 cwd 기준으로 해석하므로 `.issueops/gates/<slug>.md`와 `GATES.md` 기본값이 요청 workspace에 생긴다. |
| 2 | gates 파일 경로가 절대 경로, `../`, symlink로 workspace 밖을 읽고 쓸 수 있었다 | 새 `adapter/gates.WorkspaceFileStore`가 모든 읽기·쓰기·생성·discovery를 `os.Root`(저장소가 `projectdoc/catalog.go`에서 이미 쓰는 제한 파일시스템)로 수행한다. 절대 경로는 root 기준 이름으로 바꾸고, 존재하는 가장 깊은 상위 경로만 `EvalSymlinks`로 맞춰 `/tmp`와 `/private/tmp` 같은 별칭을 허용한다. 실제 차단은 `os.Root`가 open마다 한다. discovery는 `fs.FS` 위에서 동작하도록 바꿨으며, native CLI는 기존처럼 `os.DirFS`와 `FileStore`를 쓴다. stdio에서 capability 없이 호출하는 기존 경로도 바뀌지 않았다. |
| 3 | MCP `ExecutionActionDependencies`에 `BaseSync`가 없어 완료된 실행의 replace preview가 실패했다 | `MCPDependencies.BaseSync`를 추가했다. CLI와 같은 `basesyncoutbound.NewInspector(basesyncoutbound.RunGit)`를 `issueOpsMCPDependencies`에 주입하고, `issueOpsExecutionActionDependencies`가 이를 그대로 넘긴다. |
| 4 | `persistExecutionTransition*`와 `issueops_state` 쓰기 헬퍼가 `context.Background()`로 commit했다 | `persistExecutionTransition`, `persistExecutionTransitionWithMutations`, `persistExecutionTransitionWithRawCAS`, `writeIssueOps`, `touchAndWriteIssueOps`, `deleteIssueOps`, `WriteIssueOps`가 모두 ctx를 받는다. 이를 부르는 `ReplacementRecords.Persist`·`WithinLock`, `CycleRecordStore.Save`·`WithinLock`, `RemoteRecordStore`, `BodySyncRepository`, sync-base 기록, publication `Persist`는 span callback이 받은 context로 commit한다. port와 application 인터페이스도 같이 바꿨다. 대상은 `port.ReplacementRecords`, `port.ModeSwitchRecords`, issueopsbranch, issueopscleanup, issueopsdelegation, issueopsmodeswitch, issueopsreplacement이며, `WithinLock` callback은 `func(context.Context) error`, `Save`·`Persist`는 ctx를 첫 인자로 받는다. 호환용 wrapper는 두지 않았다. publication transaction은 의도대로 `WithoutCancel(ctx)`를 유지하므로 값(guard, trace)은 남고 취소만 무시된다. |
| 5 | HTTP에서 도달하는 loop·worker·state 쓰기가 `Background` lock을 썼고, workspace loop 쓰기는 lock 아래에서 grant를 다시 확인하지 않았다 | looprun `Start`/`RecordAttempt`/`Stop`, worker `Enqueue`/`Cancel`/`RunReadOnly`/`DetectStuck`, state `Write`/`WriteRecord`/`Update`/`Delete`/`Prune`/`PrunePrefix`와 `stateport.Store.Mutate`, `sqlstore.DB.Mutate`, loop·worker store의 `Write`(`Put` 대신 `Apply(ctx)`)가 ctx를 받는다. MCP handler는 요청 ctx를 넘기고, CLI는 진입점에서만 `context.Background()`를 쓴다. workspace 요청의 loop store는 `grantFencedLoopStore`로 감싼다. 이 store는 loop span 바깥에서 IssueOps grant root span을 먼저 잡기 때문에, record guard가 rotation과 같은 lock 아래에서 grant를 다시 확인한다. worker job은 고정된 user worker store에 있으므로 `unboundWorkerStore`가 pass-through record guard로 grant 확인 대상에서 명시적으로 뺀다. 취소와 trace 값은 그대로 전달된다. `RunReadOnly`는 명령이 이미 실행된 뒤의 결과 기록에만 `WithoutCancel(ctx)`를 쓴다. 그렇지 않으면 job이 살아 있는 서버 PID로 running에 남는다. |

결함 5를 고치면서 self workflow의 state 저장도 MCP 요청 ctx를 받게 됐다. 대상은 plan, summary, candidate export, lesson, promote, history retention 삭제다. `SelfStateDependencies`와 `SelfPlanningDependencies`의 함수가 ctx를 받고, composition root는 그 ctx로 state service를 부른다. `augmentation/signals.go`는 state write locking과 worker stuck detection을 소스 문자열로 판정하므로, 판정 문자열과 그 fixture를 새 시그니처에 맞췄다.

### 변경 파일

- production: `cmd/issueops/mcpcli/{mcp_tools,mcp_tool_issueops_execution,mcp_sdk_server,mcp_tool_loop,mcp_tool_policy_state,mcp_tool_self,mcp_tool_assistant_worker,state_dependencies,self_state_dependencies,self_planning_dependencies}.go`, `cmd/issueops/issueopsapp/{mcp_facade,gates_wiring,loop_wiring,request_store_fence_wiring(신규),self_workflow_state_wiring,self_workflow_planning_wiring,self_workflow_history_wiring,self_augment_facade,self_verify_facade,probe_state_wiring,quality_wiring}.go`, `cmd/issueops/{statecli,loopcli,workercli}`의 CLI 진입점, `internal/adapter/gates/{check,workspace_store(신규)}.go`, `internal/adapter/issueops/{execution_state,issueops_state,cycle_record_store,execution_replacement_adapter,execution_publication_store,remote_record_store,issueops_body_sync_store,execution_sync_base,review_mutation_store}.go`, `internal/adapter/{looprun,worker}/{store,sqlstore_dependencies}.go`, `internal/adapter/outbound/state/{state_io,state_lock,state_prune}.go`, `internal/adapter/outbound/sqlstore/sqlstore.go`(`Mutate` ctx만), `internal/adapter/augmentation/signals.go`, `internal/port/{issueops_replacement,issueops_mode_switch_service}.go`, `internal/port/state/state.go`, `internal/application/{looprun,worker,state,selfaugment/history,status}`와 위에 적은 issueops application 인터페이스.
- 신규 테스트: `cmd/issueops/issueopsapp/{mcp_http_workspace_boundary_test,request_store_fence_wiring_test}.go`, `internal/adapter/issueops/execution_persist_context_test.go`, `internal/adapter/outbound/state/state_request_context_test.go`, `internal/adapter/{looprun,worker}/request_context_test.go`, `mcpcli/mcp_tool_issueops_execution_test.go`의 `TestMCPExecutionDependenciesMatchEveryCoreActionDependency`.
- 기계적 호출 갱신: 바뀐 시그니처를 쓰는 기존 테스트 약 100곳에 `context.Background()`를 넣었다. 테스트 의미는 바꾸지 않았다.

### RED → GREEN

결함 1·2·3은 수정 전 코드에서 새 테스트가 실패하는 것을 확인했다. 결함 4·5는 수정한 코드의 해당 줄만 이전 동작(`Background`)으로 되돌린 사본을 `go test -overlay`로 돌려 RED를 확인했다.

| 테스트 | 수정 전 또는 변이에서 관측한 RED | GREEN |
|---|---|---|
| `TestMCPHTTPGatesDefaultFilesResolveAgainstRequestCWD` | 기본 원장이 요청 workspace에 없음(서버 cwd에 생성됨) | 통과 |
| `TestMCPHTTPGatesPathsStayInsideAuthorizedWorkspace` | 절대 경로 `file`이 workspace 밖에 생성됨 | 통과 |
| `TestMCPHTTPCompletedReplacePreviewUsesProductionBaseSync` | 처음에는 `BaseSync` 필드가 없었고, 필드만 주입하고 전달하지 않았을 때 `completed replacement preview requires base sync inspector` | no-drift는 `--reseed` next command, drift는 `post_completion_sync_base_required` |
| `TestMCPExecutionDependenciesMatchEveryCoreActionDependency` | 핵심 action 의존성 필드 대조(신규 회귀) | 통과 |
| `TestSpanPersistRefusesCommitAfterRequestCancellation` (replacement, cycle) | `persist after cancellation err = <nil>` | `context.Canceled`, record 변경 없음 |
| `TestSpanPersistKeepsCommittedWriteWhenCancelledAfterCommit` (replacement, cycle) | `CommitCount:0 CommitCoverage:unknown` | commit 후 취소해도 기록이 남고 `CommitCount=1`, coverage complete |
| `TestStateWriteCommitsInsideRequestSpan`, `TestLoopStartCommitsInsideRequestSpan`, `TestWorkerEnqueueCommitsInsideRequestSpan` | 세 테스트 모두 commit이 요청 span에 귀속되지 않음 | `CommitCount=1`, coverage complete |
| `TestStateUpdateCancelledInsideSpanPersistsNothing`, `TestWorkerEnqueueRefusesCancelledRequest` | `err = <nil>`(취소 뒤에도 저장) | `context.Canceled`, 저장 없음 |
| `TestScopedLoopWriteRechecksGrantUnderLockAfterRotation` | Bind 뒤 rotation이 commit됐는데도 loop 쓰기가 성공 | `revoked`로 거부, attempt 0 |
| `TestScopedLoopWriteLosesGrantRevokedWhileFenceIsHeld` | fence를 잡은 동안 grant를 철회해도 쓰기가 성공 | `revoked`로 거부, attempt 0 |
| `TestMCPHTTPBoundLoopAndWorkerWritesSucceed` | `unboundWorkerStore`를 뺀 변이에서 `worker_run_read_only failed` | 실제 HTTP에서 bound loop start, record-attempt, worker 실행 성공 |

동시성 테스트는 sleep 없이 순서를 보장한다. fence 철회 테스트는 grant root span을 테스트가 먼저 잡고, 그 안에서 요청을 시작한 뒤 같은 span에서 grant를 지운다. 요청이 gate에 언제 도착하든 Bind는 철회 전에 끝났고 grant 확인은 lock을 얻은 뒤에 일어나므로 결과는 항상 거부다. 대기 상한은 `time.After(30s)` 하나뿐이다.

### 명령 결과

| 명령 | 결과 |
|---|---|
| `go build ./...`, `go vet ./...` | 출력 없음, exit 0 |
| `gofmt -l`(바뀐 파일과 새 Go 파일 전체), `git diff --check` | 출력 없음 |
| `go test ./... -count=1` | 320개 패키지 ok. 실패는 3건이며 모두 이 증분 범위 밖이다. `TestResponseContractsGolden`의 `$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count`(B 소유, 이전과 같은 경로), parent 소유이자 알려진 RED인 `TestMCPTraceBindingObservesDirectSQLSpansAndClearsCorrelation`(`missing SQL span event: EOF`), `TestDDDResponsibilityInventoryMatchesSource`(Z 소유이며 이번 신규 파일도 등록 대상)다. |
| `go test -race -count=1` mcpcli, issueopsapp, adapter/{issueops,gates,looprun,worker}, outbound/{state,sqlstore,authority}, application/{looprun,worker,state,issueopsreplacement,authority} | `WARNING: DATA RACE` 0건. 위 issueopsapp의 알려진 2건 외에는 모두 ok다(adapter/issueops 244s). |
| `go build -o /tmp/h-corr/issueops ./cmd/issueops` | exit 0 |

### 실제 바이너리 실측

환경: `ISSUEOPS_STATE_DIR=/tmp/h-corr/run/state`, 서버 cwd는 빈 디렉터리 `/tmp/h-corr/run/C`, `--addr 127.0.0.1:47951`, 서버 env에 `TRACEPARENT=00-9999…-01`을 넣었다. ready line은 `issueops mcp http ready url=http://127.0.0.1:47951/mcp pid=47048`이다. grant는 별도 bash 세션(pid 47095)을 session process로 삼아 `issueops mcp authorize --json`으로 발급했다(repoA, repoB, src). completed fixture는 overlay로만 빌드한 보조 프로그램(`/tmp/h-corr/hseed`)이 실제 `Starter`와 `WriteIssueOps`로 기록했으며, repo에는 `cmd/hseed`가 남지 않았다.

| 요청 | 결과 |
|---|---|
| `gates_init`, file 생략, grant A | `.issueops/gates/default-probe.md`가 repoA에 생성됨. 서버 cwd C는 비어 있음 |
| `gates_abandon`, file 생략 | repoA/GATES.md에 `ABANDON: G1 http probe` 기록 |
| `gates_init` file=`/tmp/h-corr/run/outside/abs.md`, `../outside/dotdot.md`, `/tmp/h-corr/run/C/server.md`, `escape/through-symlink.md`, `escape/new/dir/leaf.md` | 모두 `gate file … is outside the authorized workspace`. outside와 C에 파일 없음 |
| `gates_init` file=`new/dir/leaf.md`(존재하지 않는 workspace 내부 leaf) | repoA/new/dir/leaf.md 생성 |
| grant B로 repoA 경로에 `gates_init` | outside 오류로 거부 |
| `gates_status` files=[outside/secret.md], files 생략(symlink 원장 `linked.md` 포함) | 외부 파일 내용 미노출. discovery 결과에 `outside secret` 없음 |
| `gates_abandon` file=outside/secret.md | 거부, secret.md 원문 그대로 |
| `issueops_execution` replace preview, completed record `io-286cd51bb7ca`, no-drift, HTTP와 stdio | 둘 다 `… execution replace --id 'io-286cd51bb7ca' --expected-generation 1 --completion-generation 1 --reseed …` |
| parent에 commit을 하나 추가한 뒤 같은 preview, HTTP와 stdio | 둘 다 `post_completion_sync_base_required`, next command `… execution sync-base --id 'io-286cd51bb7ca' --preview --completion-generation 1 --json …` |
| loop_start, record-attempt(grant A) | `loop-81a213344bfb`, attempt 1 |
| 별도 Python 프로세스가 `issueops_v1/issueops.lock.db`를 `BEGIN IMMEDIATE`로 잡은 상태에서 2026-07-28 `loop_record_attempt`를 보내고 1.5초에 client abort | 서버 로그 `tool=loop_record_attempt outcome=tool_error duration_ms=1501`. fence 대기 중 요청 ctx 취소로 끝났고 기록 없음 |
| 같은 lock 아래에서 2025-11-25 요청을 보내 대기시키고, 그동안 grant row를 지운 뒤 lock 해제 | 1초 뒤와 철회 뒤에도 요청이 대기 중이었고, 해제 뒤 `authority_invalid: authority credential was revoked or never issued` |
| 재발급 grant로 loop_status | attempt 1개(`first attempt`). 취소된 쓰기와 철회된 쓰기는 모두 저장되지 않음 |
| 철회된 이전 grant / 재발급 grant로 record-attempt | Bind에서 `authority_invalid` / 성공(attempt 2) |
| `worker_run_read_only`(git status, 재발급 grant) | `status=succeeded`, exit 0 |
| SIGTERM | 서버 exit 0. 로그 52줄에 bearer, `grants/`, repoA, repoB, src-990, `9999999999`, Authorization, Bearer가 모두 0건이고 `mcp_http_request`와 `mcp_http_tool` 이벤트만 있음 |

### 전파하지 않는 경계(명시)

다음 경로는 요청 취소를 따르지 않는다. 모두 이번에 의도적으로 남긴 것이며, 총체적인 context 전파를 주장하지 않는다.

- gates CHECK 명령, `command_fake_run`, `worker_run_read_only`의 명령 실행은 `policy.Run`이 `context.WithTimeout(context.Background(), timeout)`(`adapter/policy/policy_run.go:54`)으로 돈다. 요청이 끊겨도 policy timeout까지 실행된다. gates·policy·project docs·API doc application에는 ctx 인자가 없으며, 파일 쓰기는 짧고 gates는 `os.Root` 경계 안에서만 일어난다.
- `self_verify` executor는 요청 ctx가 없는 API로 서버 생성 시 한 번 조립된다. 그 안에서 probe가 쓰는 scratch 기록은 `probeStateWriteRecord`가 `Background`로 쓴다. self_verify 실행 자체도 이전부터 취소되지 않는다.
- CLI 전용 경로: legacy `NewReviewMutationStore`(lock과 write 모두 `Background`), `issueopsremote/artifact_verification.go`, `issue_create.go` intent 기록은 MCP에서 조립되지 않는다. quality CLI의 SNR baseline 저장과 status의 읽기 전용 history 조회도 요청이 없는 CLI 진입점에서 `Background`를 쓴다.
- replacement의 `inventory`/`quiescence` 검증은 lock 안에서도 바깥 요청 ctx로 Verify한다. guard 값은 같지만, span reader 재사용(`spanKey`)은 쓰지 않는다. 이번 증분 범위 밖이라 동작을 바꾸지 않았다.

### J/K/L/M/Z에 넘기는 항목(추가분)

- **Z**: `workspace_store.go`, `request_store_fence_wiring.go`와 바뀐 export 시그니처(`WriteIssueOps`, `StateWrite` 계열, `HistoryService.History`/`ApplyRetention`)를 DDD inventory에 반영한다. B의 golden `reused_count`는 그대로 남아 있다.
- **M**: parent가 `mcp_facade.go`에 최종 `BindTrace`/observer를 연결한다. 이번 증분으로 execution persist와 loop·worker·state commit이 요청 span 안에서 일어나므로, observer를 붙이면 요청 traceparent가 해당 SQL span 이벤트에 실린다.
- **J/K/L**: 앞 절의 항목에서 바뀐 것이 없다.
