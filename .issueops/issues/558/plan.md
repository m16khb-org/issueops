# #558 계획: CI lint 복구와 채널 조회·로그·state 잔재·host adapter 정비

- Lifecycle ID: `io-7dff0c24f1e9`
- Issue: https://github.com/m16khb-org/issueops/issues/558
- Branch: `558-ci-lint-channel-perf-log-adapter-cleanup` (base `main`)
- 요청 범위: 직전 점검 결과(1~9, 11번)를 이슈 하나로 정리해 사이클 전체를 진행한다. 종료점은 draft PR 발행과 `execution complete`다. merge와 cleanup은 범위 밖이다.
- 세션: direct mode로 worktree를 준비한 뒤 환경별 자동 세션 인계를 적용한다. 인계는 실행 위치만 바꾸고 위 범위와 종료점은 그대로다.

## 적용되는 결정과 주의사항

| 문서 | 항목 | 이 계획에 미치는 제약 |
|---|---|---|
| `.issueops/adr/2026-08-22-cross-session-channel.md` | 결정 2 | 사라진 `since` ID는 처음부터 반환한다. SQL 범위 조회로 바꿔도 이 의미를 유지해야 한다. |
| `.issueops/adr/2026-08-22-cross-session-channel.md` | 결정 4 | 채널 필터는 읽기 시점 스캔이고 인덱스는 요구가 확인된 뒤에만 둔다. 새 인덱스를 만들지 않고 기존 `(bucket, id)` 기본 키 범위만 쓴다. |
| `.issueops/cautions/2026-07-07-sqlite-sqlstore-span-discipline.md` | 저장 규율 | 새 저장 동작은 sqlstore를 거친다. 채널 retention 삭제도 파일 조작이 아니라 sqlstore 연산으로 한다. |
| `.issueops/CAUTIONS.md` | 로컬 battery와 CI 게이트 일치 | 로컬 최종 battery에 CI와 같은 golangci-lint 단계를 넣는다(2번 항목의 근거). |
| `.issueops/testing/unit-and-contract.md:29-37` | 로컬 lint 명령 | `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')"`와 `GOOS=linux` 변형을 그대로 쓴다. |
| `.issueops/testing/concurrency-and-race.md:47` | `IsolateGitConfig`는 `t.Setenv`를 쓴다 | 병렬화 대상 테스트는 `t.Setenv`·`t.Chdir`·이를 부르는 헬퍼를 쓰지 않는 것만 고른다. |
| `.issueops/CAUTIONS.md:41-44` | Omo·omp MCP catalog digest | host adapter 통합 뒤에도 catalog SHA env와 config 형식이 바이트 단위로 같아야 한다. |
| `.issueops/cautions/runtime.md:46` | legacy `daemon/`은 실행 중인 프로세스가 없음을 확인한 뒤 지운다 | 자동 정리도 socket dial과 PID 생존 확인 뒤에만 `daemon/`을 지운다. |
| `AGENTS.md` §10 | 장기 실행 프로세스의 로그 rotation | MCP HTTP 서비스 로그에 크기 상한을 둔다. |
| `.issueops/adr/2026-07-08-sqlite-store-maintenance-policy.md`, `.issueops/operations/guides/cli-and-state.md:52-53` | `state maintain`은 비파괴 | 은퇴 경로 삭제를 maintain이 아니라 install/update의 commit 뒤 단계에 둔다. |
| `internal/architecture/testdata/ddd_responsibility_inventory.json` `channel-cursor-order` | 커서 규칙은 도메인 소유 | since 커서 판정을 도메인 함수에 남기고 adapter는 범위 실행만 한다. |
| go-sdk v1.8.0 `PropagateRequestCancellation` (`mcp/streamable.go:210-219`) | 2026-07-28 이상 프로토콜만 HTTP 취소 전파 | 이전 프로토콜의 대기 상한은 그대로라는 한계를 문서화한다. |
| `AGENTS.md` §7, CONSTITUTION | host adapter는 얇게, 4 host 결과 동일 | 통합은 spec 값으로만 host 차이를 표현한다. |

`ARCHITECTURE.md`, `CONVENTIONS.md`, `TESTING.md`, `ADR.md` 색인도 대조했다. 위 표 밖에서 이 변경을 막는 항목은 없다.

## 변경 내용

### A. CI lint 복구와 battery 정렬 (항목 1, 2)

- `internal/adapter/hostprobe/omp.go:337`의 `defer tx.Rollback()`을 `defer func() { _ = tx.Rollback() }()`로 바꾼다.
- `.issueops/testing/self-verification.md:73`의 최종 battery 명령 목록에 `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`와 `GOOS=linux` 변형을 추가한다.

### B. read-only 조회 경로 (항목 3)

현재 상태(확인함):
- `sqlstore.GetExisting`·`ListExisting`은 호출마다 `openExistingData`로 `sql.Open`을 하고 닫는다(`sqlstore.go:554-593`, `:640-653`).
- 채널 recv는 `ListIDs` 뒤 id마다 `Get`을 부른다(`application/channel/service.go:90-119`, `adapter/channel/store.go:51-62`).
- loop gate는 모든 loop id마다 `ReadExisting`을 부른다(`application/looprun/read.go:31-39`, `adapter/looprun/store.go:48-53`). 호출부는 `gate_readiness_wiring.go:16,19`와 `doctor_wiring.go:30`이다.
- operational health는 IssueOps id마다 `ReadIssueOpsExisting`을 부른다(`operationalhealth/collector.go:256-274`, 배선은 `issueops_readers_wiring.go:10`).

변경:
1. `sqlstore`에 `WalkExistingAfter(ctx, dir, bucket, afterID, visit)`를 추가하고 기존 `WalkExisting`은 `afterID=""`로 위임한다. 쿼리는 `WHERE bucket = ? AND id > ? ORDER BY id`이고 연결은 한 번만 연다.
2. 채널: `Effects`의 `ListIDs`/`Get`을 `MessagesAfter(ctx, startAfter)` 하나로 바꾼다. 커서 규칙(정확한 since 뒤부터, 사라진 since는 처음부터: ADR 결정 2)은 DDD 원장의 `channel-cursor-order` 정책 target인 도메인에 남긴다(`internal/architecture/testdata/ddd_responsibility_inventory.json:20679-20701`). 도메인 함수 `RangeStart(since string, sinceExists bool) string`이 범위 시작 ID를 정하고, adapter는 같은 연결에서 since row 존재를 확인한 뒤 그 값으로 `id > ?` 범위만 실행한다. `Service.read`는 orchestration으로 남는다. `IDsAfter`는 호출부가 사라지므로 지우고, 정책의 `target_symbol`·evidence를 `RangeStart`로 갱신한다. `TestDDDResponsibilityInventoryMatchesSource`로 확인한다. observed 집합과 `SelectReceived`의 channel·limit 판정은 지금처럼 Go에서 한다.
3. loop: `Store`에 한 번의 walk로 모든 loop를 decode하는 `ReadAllExisting()`을 두고, `ReadExisting`과 같은 decode 함수를 공유한다. row별 decode 오류는 지금처럼 `LoopObservation.Error`로 남긴다.
4. health: `IssueOpsReader`의 `ListIDs`/`Read`를 row 순서를 보존하는 scan 함수 하나로 바꾼다. `adapter/issueops`의 기존 `decodeIssueOpsRecord`와 `GetAllExisting`을 재사용하고, decode 실패 row는 지금과 같은 `issueops_read_failed` 문제로 기록한다.

### C. 채널 recv 취소와 retention (항목 4, 5)

- `channelapp.Service.Recv`가 `ctx`를 받고, 대기는 `Effects.Wait(ctx, d)`에서 `ctx.Done()`과 경쟁한다. 취소되면 `ctx.Err()`를 반환한다. CLI는 signal context를 넘긴다. MCP 디스패치는 지금 ctx를 버리므로(`cmd/issueops/mcpcli/mcp_sdk_server.go:216-219`) 이를 고쳐 handler(`mcp_tool_channel.go:43-51`)까지 요청 context를 넘긴다.
- HTTP 취소 전파의 한계: 공용 서비스는 `PropagateRequestCancellation: true`로 설정되어 있지만(`mcp_http.go:67-71`), go-sdk v1.8.0은 2026-07-28 이상 프로토콜 요청에만 handler context를 HTTP 요청에 묶는다(`go-sdk@v1.8.0/mcp/streamable.go:210-219`). 이전 프로토콜 client가 끊어지면 대기는 지금처럼 `timeout_seconds` 상한(기본 300초)까지 이어진다. 이 한계는 PR 본문과 `.issueops/operations/guides/cli-and-state.md`의 channel 절에 적는다. 검증은 application 테스트(취소 즉시 반환)와 mcpcli HTTP 테스트(2026-07-28 프로토콜 client가 요청을 취소하면 handler가 즉시 반환) 두 층으로 한다.
- retention: 이슈 본문은 `state prune`을 경로로 적었지만 계획에서 바꾼다. `state prune`은 `state` bucket의 key 목록을 `kept`/`pruned`로 모두 보고하는 계약이고(`application/state/prune.go:24-67`, `contract/state/results.go:26-37`), 채널은 별도 root(`<state>/channel`, `channel_wiring.go:15`)에 있다. 채널 메시지를 넣으면 출력에 모든 메시지가 섞이고, 사람이 실행하지 않으면 정리도 일어나지 않는다. 그래서 `channel send`가 쓰기 직후 같은 store에서 보존 기간(7일)이 지난 메시지를 지운다. 메시지 ID가 `msg-%016x-` 나노초 hex라서(`adapter/channel/store.go:65-74`) `id < "msg-<cutoff hex>"` 범위 삭제 한 번으로 끝난다. sqlstore에 `DeleteBefore(ctx, bucket, id)`를 추가하고 span 규율을 따른다.
- 이 변경은 이슈 완료 기준과 intent 성공 기준을 바꾼다. `feedback add --classification contract_change`로 기록하고, `issueops intent record`로 성공 기준의 retention 문구를 "`channel send`가 보존 기간(7일)이 지난 채널 메시지를 지운다"로 다시 기록하며, `sync-issue`로 본문을 고친 뒤 `feedback mark-issue-updated`를 기록한다.

### D. 테스트 병렬화 (항목 6)

- 기준선(2026-10-08 측정): 전체 suite 안에서 89초, 같은 HEAD에서 단독 `go test ./internal/adapter/issueops -count=1` 74.1초(user 27.4초, sys 33.2초로 대부분 대기 시간이다). 성공 기준은 단독 `go test ./internal/adapter/issueops -count=1` wall time이 44.5초 이하다.
- 대상 선정은 AST 스크립트(워크트리 밖 임시 파일)로 기계적으로 한다. 패키지 안 함수 호출 그래프를 만들고, 다음 중 하나를 직접 하거나 그런 함수를 호출하는 테스트는 제외한다: `t.Setenv`, `t.Chdir`, `os.Setenv`, `os.Chdir`, `os.Unsetenv`, `issueOpsStateRootForTest()`/`statestore.StateDir()`처럼 전역 state root를 쓰는 호출, **패키지 수준 변수에 대한 대입**(테스트 파일 변수와 production 변수 모두. 예: `GitCmd`·`GitOut`·`GitCmdRaw`(`policy_preflight_test_helpers_test.go:8-11`), `stubIssueOpsGit`, `executionOwnerPromptTemplate`(`execution_owner_context.go:17`)). 남은 테스트에만 `t.Parallel()`을 첫 문장으로 넣는다.
- 이미 확인한 제외 파일: `cleanup_*_inherited_command_test.go` 3개, `execution_namespace_test.go`, `span_network_guard_test.go`(TestMain fake git과 PATH), `issueops_start_lockkey_test.go`, `issueops_start_repo_test.go`, `cycle_identity_test.go`, `execution_snapshot_instance_test.go`, `issueops_pr_readiness_local_test.go`, `execution_owner_instance_test.go`.
- `go test -race -shuffle=on -count=3 ./internal/adapter/issueops`로 공유 상태 경합을 확인한다. 실패한 테스트는 원인을 고치거나 제외로 돌리고 이유를 남긴다. 목표에 못 미치면 남은 시간을 차지하는 테스트를 측정해 보고하고 임의로 범위를 넓히지 않는다.

### E. MCP HTTP 서비스 로그 상한 (항목 7)

현재 상태(확인함): launchd plist와 systemd unit이 stdout/stderr를 `<state>/mcp-http/server.log`로 append한다(`mcpservice/supervisor.go:107-110,201`). 프로세스는 `diagnostics` writer로 그 fd에 쓴다(`mcp_http_command.go:41,60,77`).

변경: `runMCPHTTP`가 stderr가 `<state>/mcp-http/server.log`와 같은 파일(`os.SameFile`)일 때만 `mcpservice` adapter의 capped writer로 감싼다. writer는 시작 시 fd 1·2에 `O_APPEND`를 보장하고(unix `fcntl`), 쓰기 뒤 크기가 8MiB를 넘으면 내용을 `server.log.1`로 복사하고 원본을 truncate한다(copytruncate). 최대 사용량은 약 16MiB다. unit 파일 형식은 바꾸지 않는다.

### F. 은퇴한 state 경로 정리 (항목 8)

현재 상태(확인함):
- `state doctor`는 현재 쓰이는 `channel`(`channel_wiring.go:15`)과 `upstream`(`upstream_wiring.go:35`) 디렉터리를 unexpected로 보고한다(`domain/state/doctor.go:76-83`, 실측 경고 4건).
- 반대로 writer가 없는 `hook-failures.jsonl`, `hook-metrics.jsonl`, `.last-store-maintain`는 harness 소유로 허용한다(`doctor.go:92`). `daemon/`과 `issueops-migration-receipt.json`도 writer가 없다.
- `state maintain`은 비파괴로 문서화되어 있다: MCP 도구 설명(`internal/contract/mcp/state_catalog.go:39-41`), 운영 가이드(`.issueops/operations/guides/cli-and-state.md:52-53`), ADR `.issueops/adr/2026-07-08-sqlite-store-maintenance-policy.md`. 그래서 삭제를 `state maintain`에 붙이지 않는다. 설명을 바꾸면 Omo·omp·agy의 advertised catalog SHA도 바뀐다.

변경:
1. doctor 허용 목록에 `channel`, `upstream`을 넣고 은퇴한 세 파일 이름을 뺀다. 은퇴 경로 allowlist(`daemon/`, `hook-failures.jsonl`, `hook-metrics.jsonl`, `.last-store-maintain`, `issueops-migration-receipt.json`)에 있는 항목은 `retired_path` code(severity warning)로 보고하고 message에 `issueops update`가 지운다고 적는다. 소비자가 없어지는 `statecontract.HookFailureLogFile`은 지운다.
2. 삭제는 `issueops install`/`update`가 맡는다. 제거된 subsystem은 update로 사라졌으므로 그 잔재도 update가 정리하는 것이 자연스럽다. `application/install.RunTransaction`의 `TransactionEffects`에 `RemoveRetiredState(*port.NativeInstallResult, dryRun bool)`을 추가하고, activation seal과 `Finalize` 뒤(commit이 확정된 뒤)에만 실행한다. 삭제는 되돌릴 수 없으므로 rollback 경계 안에 두지 않는다. dry-run은 지울 경로를 `messages`에 "would remove retired state path ..."로 나열만 한다. 실제 실행은 state root 바로 아래의 allowlist 이름만 `Lstat`로 확인해 지우고(symlink는 건드리지 않음) 결과를 `messages`에 남긴다. 삭제 실패는 install을 실패로 바꾸지 않고 수동 정리 안내 message로 남긴다(`Finalize` 실패 처리 `transaction.go:126-131`과 같은 방식). `daemon/`은 legacy daemon이 아직 실행 중일 수 있으므로(33915907이 update의 daemon stop 단계도 지웠다) 지우기 전에 `daemon/issueops.sock`에 unix dial을 시도하고 `daemon/issueops.pid`의 PID 생존을 확인한다. 둘 중 하나라도 살아 있으면 `daemon/`을 건너뛰고 수동 정지 안내 message를 남기며, doctor는 계속 `retired_path`로 보고한다. 실행 중인 daemon이 unlink된 log fd를 잡고 있으면 용량이 회수되지 않기 때문이다. 이 분기는 G10 테스트에 포함한다.
3. 운영 가이드와 install 운영 문서에 update가 은퇴 경로를 지운다는 사실을 적고, `.issueops/cautions/runtime.md:46`의 수동 삭제 안내를 `retired_path` 보고와 update 정리(실행 중인 daemon이면 건너뜀)에 맞게 고친다.

### G. host adapter 통합과 작은 정리 (항목 9, 11)

현재 상태(확인함): `adapter/omo`와 `adapter/omp`의 production 파일 차이는 설정 루트(`~/.omo` 대 `~/.omp/agent`)와 omp stdio entry의 `"type": "stdio"`, 오류 문구 하나뿐이다. 소비자는 `host_installers.go:11-12`, `install_wiring.go:20-21`, `internal/adapter/host_installers_test.go:11-12`다.

변경:
1. `internal/adapter/extensionhost` 패키지 하나에 `Installer`, `Dependencies`, `Spec{Host, DisplayName, ConfigRoot, SkillsRoot, StdioType}`을 두고 `Omo`(`DisplayName: "Omo"`), `Omp`(`DisplayName: "omp"`) spec 값을 정의한다. 사람이 읽는 문구(install 메시지 `omo/install.go:57,67`, readback label `activation.go:14`, catalog digest 오류 `mcp.go:75,79,82`, extension 불일치 오류 `activation.go:28`)는 `DisplayName`으로 만들어 지금 문구를 그대로 유지한다. 두 패키지는 지우고 소비자를 옮기며, `.issueops/architecture/hexagonal-core.md:55-56,86`의 패키지 설명도 고친다. 테스트는 두 spec을 표로 돌리고, install 결과의 `messages`·`error`와 생성된 `mcp.json`·extension 바이트를 변경 전 값과 비교한다. `internal/architecture/testdata/ddd_responsibility_inventory.json`은 `-update-ddd-inventory`로 재생성한다.
2. codex와 claude의 `shellQuote`를 `internal/adapter/installutil`의 helper 하나로 합친다. 빈 입력은 POSIX 의미대로 `''`를 반환한다. `DefaultNativeInstallRequest`가 `BinPath`를 항상 채우므로(`adapter/install/install.go:30-33`) codex의 `"issueops"` 대체값은 도달하지 않는 분기다.
3. `issueops_v1`, `lease_holder_v1`, `artifact_stage_v1` bucket 이름을 한 곳에서 정의하고 6개 선언부(`issueopscompletion/repository.go:20-21`, `issueopslease/sqlite.go:22-23`, `issueopspreparation/repository.go:24-25`, `issueops/issueops_artifact_stage.go:22`, `issueopsartifact/store_adapter.go:11`, `issueopsretention/store_adapter.go:10`)가 참조하게 한다. 위치는 architecture 테스트가 허용하는 공유 패키지로 정한다.

## 재사용하는 기존 구현

| 기존 구현 | 재사용 방식 |
|---|---|
| `sqlstore.WalkExisting` / `GetAllExisting` / `openExistingData` | 범위 조회 변형의 기반. read-only 의미와 busy timeout을 그대로 쓴다. |
| `sqlstore.DB.Apply`와 span 규율 | 채널 retention 삭제를 같은 트랜잭션 규칙으로 실행한다. |
| `adapter/issueops.decodeIssueOpsRecord`, `scanIssueOpsRows` | health scan의 decode 경로. 새 decoder를 만들지 않는다. |
| `issueopsrecord.Store.ScanEach` | row별 진단을 남기는 scan 패턴의 선례. |
| `channeldomain.SelectReceived`, `NormalizeRecv`, `PollDelay` | recv 판정 규칙은 그대로 둔다. |
| `application/install.RunTransaction`과 `Finalize` 실패 처리 | 은퇴 경로 정리를 commit 뒤 단계로 붙이고 실패를 같은 message 방식으로 남긴다. |
| `omo`/`omp` 기존 install 테스트 | spec 표 테스트로 옮겨 동작이 같음을 확인한다. |

새로 만드는 것은 `WalkExistingAfter`, `DeleteBefore`, capped log writer, `extensionhost` 패키지다. 앞의 둘은 기존 함수에 범위 조건이 없어서 필요하다. log writer는 지금 크기를 제한하는 코드가 없다. `extensionhost`는 두 복제본을 하나로 합친 결과다.

## 성능 영향

| 경로 | 지금 | 변경 뒤 |
|---|---|---|
| `channel recv` (다른 채널 메시지 N개) | 연결 N+1회, 실측 N=500에서 520ms | 연결 1회, 쿼리 2회, 목표 50ms 이하 |
| `channel recv --wait` polling 한 번 | 연결 1회 + 새 id마다 1회 | 연결 1회 |
| loop gate, health 수집 | 연결 N+1회 | 연결 1회 |
| `channel send` | Put 1회 | Put 1회 + 범위 DELETE 1회(기본 키 범위) |
| MCP 로그 한 줄 | write 1회 | write 1회 + fstat 1회, 8MiB마다 복사 1회 |
| `go test ./internal/adapter/issueops` | 89초 직렬 | 목표 44초 이하 |

hook 경로(실측 p50 7.9ms)는 건드리지 않는다. 측정은 임시 `ISSUEOPS_STATE_DIR`에 메시지 500개를 넣은 뒤 `channel recv` 실행 시간을 재고, 테스트 시간은 `go test -json`의 패키지 elapsed로 잰다.

## 하위 호환성과 side effect

- CLI/MCP: `channel` 명령과 `channel_recv`, `state maintain`, `state_maintain`의 입력·출력 필드와 도구 설명은 바뀌지 않는다. 따라서 advertised MCP catalog SHA도 바뀌지 않는다. `state doctor`에는 새 issue code 값 `retired_path`가 생긴다(필드 추가 없음). response-contract golden에 차이가 생기면 갱신하고 그 이유를 기록한다.
- 동작 변화 1: 7일이 지난 채널 메시지는 다음 `send` 때 지워진다. 오래된 `since` ID가 지워진 경우 ADR 결정 2에 따라 처음부터 반환하는 동작은 그대로다.
- 동작 변화 2: `issueops install`/`update`가 commit 뒤 state root 바로 아래의 은퇴 경로 allowlist를 지운다. 되돌릴 수 없는 사용자 파일 삭제이므로 정확한 이름만 대상으로 하고 symlink는 건드리지 않으며, dry-run이 미리 보여 준다. 롤백은 코드 revert이며 지운 파일은 복구하지 않는다(모두 제거된 subsystem의 산출물이다). 삭제 실패는 install 결과를 실패로 바꾸지 않는다.
- 동작 변화 3: `state doctor`가 `channel`, `upstream`을 더 이상 경고하지 않고, 은퇴 경로가 남아 있으면 `retired_path`로 경고한다.
- 동작 변화 4: 2026-07-28 이상 프로토콜의 MCP client가 `channel_recv` 대기 중 요청을 취소하면 즉시 반환한다. 이전 프로토콜은 지금처럼 `timeout_seconds`까지 기다린다(go-sdk 한계, 문서화).
- record schema, provider body 계약, DB 스키마·인덱스 변경은 없다. 데이터베이스 변경은 row 삭제(채널 retention)뿐이다. 현재 실측 row 수는 `channel_v1` 24건이다.
- host 설정: Omo·omp의 `mcp.json`, extension, skill 링크 경로와 내용, install `messages`·`error` 문구는 바이트 단위로 같아야 한다. install dry-run JSON을 변경 전후로 비교한다.
- LLM 프롬프트 본문 변경은 없다.

## 게이트

CHECK는 셸 없이 argv로 실행되므로(`internal/adapter/policy/policy_run.go:34`) 환경 변수·파이프·시간 측정은 `python3 -c`로 감싸고 고유한 EXPECT 토큰을 출력한다(선례: `.issueops/issues/552/gates.md`). 아래는 gates init에 넘길 spec의 요지이며, 실제 CHECK 문자열은 4단계에서 이 형식으로 작성한다.

| Gate | 결과 | CHECK 요지 | EXPECT |
|---|---|---|---|
| G1 | 모든 Go 파일이 gofmt 형식이다 | `gofmt -l $(git ls-files '*.go')` 출력이 비었는지 | `GOFMT_CLEAN` |
| G2 | go vet 통과 | `go vet ./...` rc=0 | `ALL_PASS` |
| G3 | golangci-lint 0건 (darwin, linux) | go.mod toolchain(`GOTOOLCHAIN=go<go.mod>`)으로 `golangci-lint run ./...`를 기본과 `GOOS=linux`에서 각각 rc=0 | `ALL_PASS` |
| G4 | battery 문서에 lint 단계 | `grep -c 'golangci-lint run' .issueops/testing/self-verification.md` | `/^[1-9][0-9]*$/` |
| G5 | 채널 범위 조회·retention·커서 규칙 | `go test -v` sqlstore·adapter/channel·application/channel·domain/channel에서 새 테스트 이름별 `--- PASS` 개수 확인 | `ALL_PASS` |
| G6 | recv 취소 (application + MCP HTTP) | `go test -v`로 `TestRecvWaitReturnsWhenContextCancelled`와 mcpcli의 HTTP 취소 테스트가 모두 `--- PASS` | `ALL_PASS` |
| G7 | channel recv 50ms 이하 | 임시 `ISSUEOPS_STATE_DIR`에서 다른 채널 메시지 500개와 대상 메시지 1개를 send한 뒤 `bin/issueops channel recv --channel target --json` 5회 중앙값 측정 | `RECV_OK` (출력에 ms 포함) |
| G8 | loop·health 단일 scan | looprun·operationalhealth 패키지 `go test -v`에서 새 테스트 `--- PASS` | `ALL_PASS` |
| G9 | 로그 상한 | mcpservice capped writer 테스트 `--- PASS` | `ALL_PASS` |
| G10 | state doctor 분류와 update의 은퇴 경로 정리 | domain/state·application/install·install adapter 테스트 `--- PASS` | `ALL_PASS` |
| G11 | host adapter 통합 | `go test ./internal/adapter/extensionhost ./internal/adapter` rc=0, 그리고 Omo·omp install dry-run JSON이 변경 전 저장본과 같다 | `ALL_PASS` |
| G12 | DDD 원장 | `go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -v` | `ALL_PASS` |
| G13 | 테스트 병렬화 경합 없음 | `go test -race -shuffle=on -count=3 ./internal/adapter/issueops` rc=0 | `ALL_PASS` |
| G14 | 테스트 시간 44.5초 이하 | 단독 `go test ./internal/adapter/issueops -count=1` wall time 측정 | `WALL_OK` (출력에 초 포함) |
| G15 | race 전체 통과 | `go test -race ./... -count=1` rc=0 | `ALL_PASS` |
| G16 | self-verify 통과 | `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`의 ok | `SELF_VERIFY_OK=True` |
| G17 | PR CI 통과 | 현재 HEAD의 `ci.yml` 실행이 success | `CI_GREEN` |

intent 성공 기준 대응: lint·CI → G3·G17, battery → G4, recv 50ms → G7, 취소 → G6, retention → G5, 테스트 시간 → G14, 로그 → G9, 은퇴 경로 → G10, host 통합 → G11, gofmt·vet·race·self-verify → G1·G2·G15·G16.

## 검증 계획

1. RED: 채널 범위 조회, recv 취소, retention, doctor 분류, 은퇴 경로 정리, capped writer에 대한 실패 테스트를 먼저 쓴다.
2. GREEN: 위 변경으로 통과시킨다.
3. SURFACE: G7 채널 측정, 임시 HOME·state에서 은퇴 경로를 만든 뒤 `install --dry-run --json`의 messages와 실제 install 뒤 삭제 확인, install dry-run JSON의 Omo·omp 항목 비교, 로그 writer는 임시 파일에서 상한을 넘겨 rotation 확인.
4. CLEAN: G1~G3, G15, G16을 다시 실행한다.

## 의존성과 로컬 설정

새 외부 의존성은 없다. 로컬 전용 설정 파일은 링크하지 않는다.
