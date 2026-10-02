# I10 공용 HTTP MCP 권한 설계

기준: HEAD `92eaa001`. 실행 모델 `PI_MODEL=claude-opus-5-5`, `PI_THINKING_LEVEL`은 export되지 않았다.

## 현재 결합

- MCP mutation 가운데 native actor를 증명하는 경로는 `issueops_execution` 하나다. `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go:55`가 `os.Getpid()`로 서버 자신의 계보를 관측하고, `:62-73`은 caller가 보낸 `session_pid` 등의 receipt를 그 계보와 대조한다. `mcpcli` 안의 다른 `ObserveNativeProcessAncestry(` 호출은 없다(grep 결과).
- 계보 검증은 `internal/domain/issueops/native_actor.go:23-43`과 `internal/application/issueopscycle/native_actor.go:10-19`(live 재검사)에 있다. 같은 검사가 9곳에서 반복된다: `issueopslease/{claim:50,release:37,resume:53,reseed:55}`, `issueopscompletion/complete.go:45`, `issueopsexecution/execution_reconcile.go:20`, `issueopsreplacement/service.go:27`, `adapter/issueops/execution_lease_identity.go:14`, `execution_sync_base.go:84`.
- claim 권한은 `claim_transaction.go:24-73`이 holder, generation, canonical cwd, 일회용 token SHA(`record.go:103-112`)로 판정하고, 성공하면 token을 지운다(`:65,:72`).
- lease record는 `records(bucket,id,data)` 테이블의 `issueops_v1` bucket에 저장된다(`sqlstore.go:207-212`, `outbound/issueopslease/sqlite.go:22`). `_txlock=immediate`로 직렬화된다.
- 서버 process의 cwd를 기본값으로 쓰는 wiring이 있다: `basic_wiring.go:16`, `project_docs_wiring.go:22`, `pathutil/path_helpers.go:17-18,75`, `issueops_cleanup_wiring.go:83,102`.

## 후보 비교

**A. holder에 묶인 authority capability (채택).** native 세션이 shell에서 CLI를 실행하면 CLI 계보에 그 세션이 들어간다(`executioncmd/execution.go:427`). 기존 증명이 통과하면 CLI가 무작위 token을 한 번 출력하고, 서버는 SHA만 저장한다. 공용 서버는 token으로 저장된 holder를 찾아 actor로 대입한다. caller가 보낸 actor 필드는 권한 근거로 쓰지 않는다.

**B. loopback TCP peer → PID 역추적 후 계보 관측 (기각).** macOS는 소켓 소유 PID를 `lsof`로 조회해야 해서 요청마다 비용이 크다. 연결 재사용과 포트 재사용 사이에 race가 있고, sandbox proxy를 거치면 실제 세션이 아닌 proxy PID가 관측된다. ADR 2026-09-23이 기각한 "peer PID 전달"과 같은 신뢰 경계 문제도 다시 생긴다.

## 결정 (A)

1. **발급:** `issueops execution authority --id ID --generation N ACTOR_FLAGS --cwd PATH --json`. 기존 lease fence 안에서 기존 `NormalizeNativeActor`로 계보와 live 상태를 확인하고, `status=active`, `generation=N`, `holder==actor`, `cwd`가 canonical root인지 검증한다. 검증을 통과하면 32바이트 token을 만들어 stdout에 한 번만 출력한다. 이 명령은 holder를 만들지 않는다. claim, prepare, resume, takeover처럼 holder를 세우는 전이는 CLI 또는 stdio에서만 수행한다. 공용 HTTP에는 이 전이를 위한 증명 수단이 없다.
2. **저장:** 같은 data DB의 새 bucket `issueops_authority_v1`을 쓰고 key는 cycle id다. JSON `{schema_version:1,id,generation,holder,canonical_root,token_sha256,issued_at,expires_at}`. 기존 규칙에 따라 missing·0·future schema는 거부한다. lease record schema는 바꾸지 않는다. 별도 신원 DB를 만들지 않는다.
3. **유효 조건:** 요청마다 다음을 모두 검사한다. SHA 상수시간 비교, `now<expires_at`(TTL 12h 고정), `lease.status==active`, `lease.generation==authority.generation==요청 generation`, `sameActor(lease.holder, authority.holder)`, `cwd` canonical 일치, holder `session_process` live 재검사(`inspectNativeProcessReceipt`는 PID 조회라 어느 process에서도 동작한다).
4. **대입과 원자성:** inbound adapter는 검증을 통과하면 `Actor=authority.holder`, `ProcessAncestry=[holder.session_process]`를 만들어 기존 core에 넘긴다. 9곳의 검증 코드는 바꾸지 않는다. holder·generation·status·cwd는 core가 기존 transaction 안에서 다시 검사하므로 token 확인과 commit 사이에 release, replace, reseed가 끼어들어도 거부된다. 이 race에서 남는 경우는 같은 holder가 token을 재발급하는 것뿐이고 무해하다.
5. **철회와 만료:** release, replace, reseed, cleanup이 holder나 generation을 바꾸면 기존 record는 자동으로 무효가 되므로 삭제 코드가 필요 없다. 재발급하면 같은 key를 덮어써서 이전 token이 철회된다. 세션이 재시작되면 PID가 바뀌어 live 검사에서 실패하므로 기존 CLI resume 뒤에 재발급한다. 서버를 재시작해도 상태가 SQLite에 있으므로 영향이 없다.
6. **전달:** tool 인자 `authority_token`으로 넘긴다. 헤더는 host config에 정적으로 고정되므로 세션별 값을 실을 수 없다. trace·audit·응답에서 redact한다(저장 위치는 S3 구현 담당이 찾는다). HTTP 요청에 `host/session_id/session_pid*` 인자가 있으면 혼동을 막기 위해 거부한다.
7. **stdio:** 기존 계보 증명을 그대로 유지한다. `authority_token`도 같은 의미로 받아서 schema가 transport별로 달라지지 않게 한다.
8. **transport:** SDK `StreamableHTTPHandler`를 Stateless로 두고 `*mcp.Server`는 하나만 만든다. 요청별 상태는 인자에만 둔다. transport session은 권한이 아니다. 전역 current actor·workspace는 두지 않는다. HTTP 의존성 조립 시 `Getwd`는 오류를 반환하므로, 위에 적은 cwd 기본값 지점은 HTTP에서 명시적 `cwd`/`workspace_root`가 없으면 fail-closed한다. 취소는 commit 전에만 효과가 있고, commit된 결과는 그대로 남는다.
9. **노출 제한:** `127.0.0.1`에만 bind하고 Host와 Origin을 검증한다. 설치 시 만든 정적 bearer(0600 파일)를 `Authorization`으로 요구한다. 같은 OS 사용자는 SQLite를 직접 쓸 수 있으므로 이미 신뢰 경계 안에 있다. 이 설계가 막는 대상은 다른 세션의 잘못된 mutation과 다른 OS 사용자·브라우저다.

## 서비스 수명

- `issueops mcp service start|stop|status --json`, 서버 본체는 `issueops mcp --http ADDR`. 단일 인스턴스는 `internal/adapter/daemon`의 `acquireFileLock`(`lock.go:12`)과 `WriteInstance`(`instance.go:31`)를 별도 디렉터리 `<state>/mcp-http/`에서 재사용해 보장한다. instance에는 pid, executable, build id, addr를 기록한다. 고정 포트가 충돌하면 start가 실패하고 status가 원인을 보고한다.
- 재부팅 뒤 기동: install이 darwin LaunchAgent, linux systemd user unit을 HOME 아래에 쓴다. 다른 OS에는 HTTP를 설치하지 않고 stdio를 유지한다.
- update/bootstrap: `update/runtime.go:55`의 `StopDaemon` 옆에 service stop(`http.Server.Shutdown` drain) → 교체 → start → status build id 일치 확인 단계를 둔다.
- host 설정: codex `[mcp_servers.issueops] url`과 header(`install_config.go:56-63` 대체), claude `"type":"http"`(`install_mcp.go:18-26`), omo `url`(`omo/mcp.go:47-66`). agy는 stdio로 남는다.

## 구현 범위 (쓰기 경계 분리)

| ID | 파일 | 선행 |
|---|---|---|
| S1 | `internal/contract/issueopslease/authority.go`, `internal/domain/issueopslease/authority.go`, `internal/adapter/outbound/issueopslease/authority_store.go` | 없음 |
| S2 | `cmd/issueops/issueopscli/executioncmd/*`, `internal/contract/cli/issueops_catalog.go`, `internal/domain/commandparse/issueops.go` | S1 |
| S3 | `cmd/issueops/mcpcli/mcp_http*.go`, `mcp_tool_issueops_execution.go`, `internal/contract/mcp/issueops_catalog.go`, HTTP용 `issueopsapp` wiring | S1, I5 SDK |
| S4 | service 명령, `internal/adapter/update/runtime.go`, `cmd/issueops/updatecli/*` | S3 |
| S5 | `internal/adapter/{codex,claude,omo}` 설치, unit 파일 | S4 |
| S6 | ADR 2026-09-23 대체 기록, `.issueops/architecture/runtime.md` | 전체 |

## 검증

필수 테스트는 다음과 같다. 모두 fake inspector와 시계를 주입하고 sleep을 쓰지 않는다.
- S1: `TestAuthorityRejects{WrongToken,StaleGeneration,ForeignHolder,Expired,ReleasedLease,DeadHolder,OtherWorkspace}`, `TestAuthorityRecordRejectsMissingZeroFutureSchema`.
- S2: 계보 밖 actor가 발급을 요청하면 거부한다. 재발급하면 이전 token이 무효가 된다.
- S3: `httptest`로 두 caller를 만든다. owner는 성공하고, 다른 caller의 token 재사용·caller actor 인자·bearer 누락·외부 Origin은 거부한다. 동시 release와 mutation 중 하나만 commit된다. commit 전 취소는 record를 바꾸지 않는다. 서버를 재시작해도 token이 유지된다.
- S5: 격리 HOME에서 세 host config와 unit을 검사하고, dry-run이 아무것도 쓰지 않는지 확인한다(`install_contract_matrix_test.go:60,98` 확장).

```bash
go test ./internal/domain/issueopslease ./internal/adapter/outbound/issueopslease ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./internal/adapter -count=1
go test -race ./cmd/issueops/mcpcli ./internal/adapter/outbound/issueopslease -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
tmp="$(mktemp -d)" && HOME="$tmp" ISSUEOPS_STATE_DIR="$tmp/state" ./bin/issueops mcp service start --json && HOME="$tmp" ISSUEOPS_STATE_DIR="$tmp/state" ./bin/issueops mcp service status --json && HOME="$tmp" ISSUEOPS_STATE_DIR="$tmp/state" ./bin/issueops mcp service stop --json; rm -rf "$tmp"
```

## 남은 제약과 미결정

- holder를 세우는 전이(claim, prepare, resume, takeover)는 HTTP로 할 수 없다. 공용 서버는 holder 범위 mutation과 조회를 맡는다.
- token이 CLI stdout을 거쳐 host 전사에 남는다. 같은 사용자 범위이고 holder·generation·live·TTL에 묶여 있어 허용한다.
- 세 host가 정적 header와 HTTP config를 지원하는지는 설치된 binary를 grep해 확인해야 한다. codex는 문서로 확인했다(`research/.../codex-mcp-verification.md:11`). claude와 omo는 미확인이다. 지원하지 않는 host는 stdio로 남는다.
- SDK v1.6.1(`go.mod:33`)이 지원하는 버전은 2025-11-25까지다. 2026-07-28 binding의 취소 의미는 I5 결과에 따른다.
- 고정 포트 번호, TTL 12h, LaunchAgent 채택은 parent가 승인해야 한다.
- redaction 대상 인자 목록이 어디에 있는지는 확인하지 못했다.
