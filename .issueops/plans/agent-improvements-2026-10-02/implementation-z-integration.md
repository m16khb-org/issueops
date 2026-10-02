# Z-integration: 공유 golden, architecture inventory, usage, 운영 문서와 최종 battery

기준 트리는 HEAD `92eaa001`에 A-M producer 변경이 얹힌 작업 트리다. Z는 마지막 단일 writer로 일했다.
commit과 push는 하지 않았다. `.issueops/plans/wmux-windows-port.md`, `.issueops/research/2026-10-02-ghostty-cmux-windows.md`,
`openwiki/`, 실제 `$HOME`, 저장소의 `bin/issueops`는 건드리지 않았다. 빌드는 모두 `/tmp/z-bin/issueops`(또는 `/tmp` 사본)로 했다.

## 1. 변경한 파일 (목적별)

### 1.1 Golden과 tracked 템플릿 (각 재생성 뒤 diff를 읽고 원인을 대조함)

| 파일 | 재생성 방법 | diff 판독 결과 |
|---|---|---|
| `cmd/issueops/testdata/response_contracts.golden.json` | `go test ./cmd/issueops/issueopsapp -run 'TestResponseContractsGolden$' -update` | HEAD 대비 JSON 경로 단위로 추가 7곳, 삭제·변경 0곳. `$.cli.inspect.integration.hosts`와 `$.mcp.harness_inspect...integration.hosts`(I3/K, host 3개, `$HOME`·`$TIMESTAMP` 정규화, 실경로 `/Users/` 0건), `self_verify_compare`의 baseline/candidate `reused_count` CLI·MCP 4곳(I2), `$.cli.trace_analyze.complete`(I1). 문서 편집 뒤 다시 갱신했지만 docs count placeholder 덕분에 추가 diff는 없었다 |
| `cmd/issueops/testdata/usage.golden.txt` | `go test ./cmd/issueops/contractgolden -run TestCLIUsageGolden -update` | 아래 usage.go 변경 5줄 교체와 3줄 추가만 있다 |
| `cmd/issueops/testdata/mcp_tools.golden.json` | L이 재생성한 것을 Z가 검증만 했다 | 51개 도구 이름이 같다. 변경된 도구는 26개이며 모두 추가다. `harness_inspect`는 `authority_file/cwd/host_receipts/workspace_root`와 `annotations`·`outputSchema`, `docs_index`는 `annotations`·`outputSchema`, 나머지 workspace 도구 24개는 `authority_file`(+필요하면 `workspace_root`/`cwd`)만 늘었다. 삭제·변경된 기존 속성은 0개다 |
| `internal/adapter/testdata/native_install_contract_matrix.golden.json` | `go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract` | 바뀐 줄은 `content_sha256` 10곳뿐이다. 모두 agy/omo MCP config·template 항목(`agy_project_mcp_config`, `agy_project_mcp_template`, `agy_user_mcp_config`, `omo_project_mcp_config`, `omo_project_mcp_template`, `omo_user_mcp_config`)이며, 원인은 catalog SHA-256 변화(D-L1)다 |
| `configs/omo/mcp.json`, `configs/agy/mcp_config.json` | 손으로 고치지 않았다. `go test -overlay`로 임시 테스트를 주입해 `testInstaller().omoProjectMCPConfig()`/`agyProjectMCPConfig()` 출력을 그대로 썼다. 저장소에 테스트 파일은 남지 않는다 | `ISSUEOPS_MCP_CATALOG_SHA256`이 `f0f4f6ba…6eed`에서 `d0b943dd…a913`(= `SemanticSHA256(mcpcatalog.AdvertisedTools())`)으로 바뀐 한 줄뿐이다. 구 해시가 남은 tracked 파일은 0개다(`f0f4f6baf1eead8b` grep 0건) |

### 1.2 Architecture inventory

| 파일 | 방법 | 내용 |
|---|---|---|
| `internal/architecture/testdata/ddd_responsibility_inventory.json` | `go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -update-ddd-inventory` | 재생성 전 파일은 HEAD와 같았다. 신규 파일 47개와 기존 파일 45개의 심볼 변화를 읽었다. `artifacts`와 `policies`는 바이트 단위로 같다. 삭제된 심볼은 모두 producer 리팩터링의 이동이다: `issueOpsCompletionProcessInspector`, `ValidateNativeActorProcess`, `resolveActor`, `ProcessInspector`는 G의 `ActorVerifier`로, `observeIssueOpsWorktreeState`와 `recentCommits`는 I6의 `nextGitObservation`/`commitHistory`로, `sdkToolHandler`/`issueOpsExecutionSDKToolHandler`는 H의 `mcp_request_dispatch.go`로, gates `append*`는 `gateFileScan`으로, `Store.observe`는 `Store.ObserveSpans`로, projectdoc catalog 함수는 I7의 `DiscoverProjectDocsReport`로 옮겨졌다. parent의 D-K1(`receipts.go`의 `artifactEvidence`, `receipt_stale_config_path`)과 D-J1(`install_mcp_transport.go`의 `Command.preflightHostPlan`)도 등록돼 있다. 신규 파일은 기존 prefix 규칙에 따라 owner와 task를 받았고(예: `installcli` T11, `gates` T10, `catalog/mcp` T19, `trace` T15, 그 밖은 T20), `dddTask` 표는 바꾸지 않았다 |
| `internal/architecture/testdata/ddd_contract_function_roles.json` | 정렬 위치에 직접 추가 | `internal/contract/mcp/output_schemas.go`의 함수 9개(`DocsIndexOutputSchema`, `InspectOutputSchema`, `ReadOnlyClosedWorldAnnotations`, `schemaArray/Bool/Integer/Object/Ref/String`)를 `shape`로 등록했다. 모두 순수 map·pointer를 만드는 schema 생성 함수다(`output_schemas.go:8-30,90`). 테스트 조건은 바꾸지 않았다 |

### 1.3 Root usage (유일한 Go 소스 변경)

`internal/adapter/inbound/catalog/cli/usage.go`:
- `:31` `trace analyze --input <jsonl|state-key|-> [--input-format issueops|claude-stream|codex-exec|omo-json]` (`cmd/issueops/basiccli/trace.go:32,93`과 일치)
- `:66-68` `install`/`update`/`bootstrap`에 `[--mcp-transport=http|stdio]`를 추가했다. `install.go:25`와 `update_bootstrap.go:23`이 같은 플래그를 파싱한다
- `:77` `mcp --http [--addr 127.0.0.1:47831]` (`mcp_http_command.go:24-25`, `mcp_http.go:20`)
- `:78` `mcp service start|stop|status [--json]` (`mcp_service_command.go:26-50`)
- `:79` `mcp authorize --workspace-root PATH --host ... --session-pid PID --session-started-at RFC3339 --session-executable PATH [--cwd PATH] [--json]` (`mcp_authorize_command.go:42-50`)

### 1.4 운영 문서

project_docs MCP 도구는 이 세션의 도구 목록에 없었으므로 MCP를 거치지 않고 직접 편집했다(project_docs 계약 미사용).

| 파일 | 변경 |
|---|---|
| `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md` (신규 ADR) | 결정, 맥락, 결과를 담았다. 같은 OS 사용자 신뢰 경계, stdio 호환 표면 유지, 2026-09-23 in-process 결정을 stdio 경로로 좁히는 내용, unit env 한계, `idempotentHint:false`, 대안·기각 사유, 테스트 근거를 포함한다 |
| `.issueops/ADR.md`, `.issueops/adr/README.md` | 색인 행을 추가하고, 2026-09-23 행에 축소 표기를 했다. supersession 목록에도 항목을 추가했다 |
| `.issueops/ARCHITECTURE.md` | 실행 모드 요약에 공용 HTTP 서비스를 넣고 요청별 capability·신뢰 경계 문단을 추가했다 |
| `.issueops/OPERATIONS.md` | Core Surfaces 2를 HTTP 기본, `mcp --http`/`service`/`authorize`, `--mcp-transport=stdio`, status DTO와 빈 status 규칙(D-J2), unit env 한계, `idempotentHint:false`로 교체했다 |
| `.issueops/TECH_STACK.md` | 실행 모드, MCP 행(go-sdk v1.8.0, jsonschema-go v0.4.3, `go.mod:33-34`), IPC 행(loopback HTTP), 직접 의존성 목록을 고쳤다 |
| `.issueops/architecture/runtime.md` | HTTP 서비스 행을 추가하고 stdio 행을 "호환 표면"으로 바꿨다 |
| `.issueops/architecture/hexagonal-core.md` | 다이어그램에서 세 host가 공용 HTTP 서비스로 연결되고 stdio는 선택 경로가 되도록 고쳤다. `cmd/issueops` 책임 행도 고쳤다 |
| `.issueops/cautions/runtime.md` §13 | 서비스 build_id 교체, 수동 `go build` 뒤 서비스 skew, 서버 cwd 비사용, unit env, 테스트 label 정리를 추가했다 |
| `.issueops/cautions/audit-and-process.md` §22 | HTTP smoke 방법(두 curl, 401/403, `authority_file`)과 같은 사용자 credential 경계를 추가했다 |
| `.issueops/cautions/lessons/2026-10-02-real-host-qa-tool-results-service-tier-amfi-exact-edits.md` (신규 lesson), `.issueops/CAUTIONS.md` 색인 | Claude tool-results 파일 저장, Codex 0.128.0 `service_tier="default"` 거부, AMFI SIGKILL과 QA 사본 ad-hoc 서명, apply_patch 없는 정확 치환 편집을 기록했다 |
| `.issueops/operations/cli-and-mcp.md` | `integration.hosts`, `--host-receipts`, receipt 판정, `config_sha256` 범위(D-K2), HTTP inspect env 한계, mcp HTTP/authorize 안내를 추가했다 |
| `.issueops/operations/install.md` | HTTP 기본 설치 순서와 stdio 선택, **Omo HTTP catalog cache 주의**(아래 §5)를 추가했다 |
| `AGENTS.md` | §5 통합 표면 행, §8 `cmd/issueops/` 행(mcp 하위 명령), §9 명령(`mcp service status`, stdio dry-run)을 고쳤다 |
| `README.md` | 설치 표의 MCP 서비스 행, HTTP 기본·stdio 선택·authorize 안내, update의 서비스 교체, 다이어그램, 아키텍처 문단을 고쳤다 |
| `.issueops/plans/agent-improvements-2026-10-02/contract.md` | §3 D-J2와 §4 I3 D-K2·D-K1 보완 문단을 추가했다(근거 file:line 포함) |

## 2. D-J2, D-K2, 서비스 env 한계, idempotentHint 처리

- **D-J2 (빈 status)**: 동작은 바꾸지 않고 문서화했다. 코드상 supervisor 미지원(`internal/adapter/mcpservice/service.go:114`), supervisor 미가용(`:117`), lock 읽기 실패(`:138`), start 때 binary 읽기 실패(`:305`, `build_mismatch`)는 `status:""`, `ok:false`를 반환한다. `contract.md` §3과 `OPERATIONS.md` Core Surfaces 2에 "관측하지 못하면 status는 빈 문자열이고 error_code가 원인"이라고 적었다.
- **D-K2 (config_sha256 범위)**: `hostconfig.go:152-157`은 secret을 지운 issueops entry만 해싱한다. 이유(`~/.claude.json`의 `numStartups` 등 host가 바꾸는 필드)와 함께 `contract.md` §4 I3와 `operations/cli-and-mcp.md`에 적었다. Claude와 Omo가 같은 hash를 가질 수 있다는 점과 D-K1의 config_path·artifact 내용 조건(`receipts.go:196,205,213`)도 함께 적었다.
- **서비스 env 한계**: unit은 `ISSUEOPS_ROOT`·`ISSUEOPS_STATE_DIR`만 넘긴다(`supervisor.go:31`). HTTP `harness_inspect`는 supervisor 기본 `HOME`/`PATH`로 동작하고 `CODEX_HOME`이 없으므로 `host_version_unobservable`이 나올 수 있다. 이 내용을 새 ADR, `OPERATIONS.md`, `cautions/runtime.md`, `operations/cli-and-mcp.md`에 적었다. unit env는 넓히지 않았다.
- **idempotentHint:false**: SDK가 non-pointer 필드라서 두 읽기 전용 도구에 붙인다는 사실을 ADR과 `OPERATIONS.md`에 적었다. 실제 HTTP tools/list에서 `{'idempotentHint': False, 'openWorldHint': False, 'readOnlyHint': True}`를 관측했다(§3 HTTP smoke).
- 새 ADR 경로: `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md`.

## 3. 명령과 exit code

| 명령 | exit | 결과 |
|---|---:|---|
| Z 시작 시 실패 재현: `go test ./cmd/issueops/issueopsapp ./internal/architecture ./internal/adapter ./internal/adapter/omo ./internal/adapter/agy -count=1` | 1 | 알려진 5건(TestResponseContractsGolden, TestDDDResponsibilityInventoryMatchesSource, TestDDDContractFunctionsHaveExplicitRoles 9개 함수, TestNativeInstallAdapterContractMatrix, TestTrackedTemplatesMatchGeneratedContent). agy는 ok |
| `go test ./internal/adapter/omo ./internal/adapter/agy -count=1` (템플릿 재생성 뒤) | 0 | ok, ok |
| `go test ./internal/adapter -count=1` (매트릭스 재생성 뒤) | 0 | ok |
| `go test ./internal/architecture -count=1` (inventory·roles 갱신 뒤) | 0 | ok |
| `gofmt -l $(git ls-files '*.go') $(git ls-files -o --exclude-standard '*.go')` | 0 | 출력 없음 |
| `go vet ./...` | 0 | 출력 없음 |
| `go build -o /tmp/z-bin/issueops ./cmd/issueops` (저장소 `bin/issueops`는 실제 stdio host가 쓰므로 빌드하지 않음) | 0 | sha256 `f19fd7ca…62ea`. Z는 저장소 `bin/issueops`에 쓰지 않았다. 다만 01:06에 누군가 다시 빌드했다(§4 참조) |
| `go test ./... -count=1` (골든 갱신 직후, `mon_T41MYSYQ8EPDP73R`) | 0 | ok 323 |
| `go test ./... -count=1` (문서 편집 뒤 최종, `mon_9SEZSRB2ST1MQN0X`, `/tmp/z-gotest-final.log`) | 0 | ok 323, FAIL 0 |
| `go test -race ./... -count=1` (`mon_8PSFP741DPZJ81QR`, `/tmp/z-race.log`) | 0 | ok 323, FAIL 0, `DATA RACE` 0건 |
| `go test ./cmd/issueops/contractgolden -run Golden -count=1` | 0 | ok |
| `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` | 0 | ok |
| `PATH=/tmp/issueops-ten-improvements-01a0fa31/venv/bin:$PATH python3 scripts/python_suite_runner.py` (Python 3.12.14, `include-system-site-packages = false`) | 0 | `root=1 skills=6 skill_files=10`, 마지막 `Ran 27 tests ... OK`. skip은 1건뿐이며 기존 `meeting_notes_skill_contract_test`의 gitignored local background다. `skills/meeting-notes`는 변경되지 않았고 새 skip은 없다 |
| `/tmp/z-bin/issueops api-doc static-check --json` (격리 state) | 0 | `ok:true`, `skipped:true`, `reason:no_api_doc_candidate_files`. 후보 파일이 없으므로 api_doc_review는 해당 없다 |
| `uv run --offline --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json` | 1 | 문서 755개를 검사했고 위반 1건이다. `.issueops/evidence/delivery-20261001/535/review-input.md`의 `../TESTING.md` broken_link이며, 이 파일은 `.gitignore:19`(`evidence`)로 무시되는 기존 증거 파일이라 Z 변경과 무관하다. Z가 편집한 문서의 위반은 0건이다 |

### 실제 표면 (격리 `HOME=/tmp/z-home.*`, `ISSUEOPS_STATE_DIR=/tmp/z-state.*`, `/tmp/z-bin/issueops`)

| 명령 | exit | 관측 |
|---|---:|---|
| `inspect --json` | 0 | `ok:true`. hosts는 codex·claude·omo 3개이고, 빈 HOME이라 configured=failed, connected=`not_checked/host_receipt_required`다 |
| `docs --json` | 0 | `ok:true`, docs 559 |
| `hook session-start --repo <root> --json` (빈 stdin payload, EOF) | 0 | 키 `compact, project_docs, should_inject, user_view`, `should_inject:true`, project_docs 13 |
| stdio `issueops mcp` one-shot JSON-RPC(initialize 2025-11-25, initialized, tools/list, docs_index) | 0 | 협상 2025-11-25, 도구 51개, outputSchema는 `docs_index`·`harness_inspect`만 있다. docs_index `ok:true`, docs 559, `json(text) == structuredContent` |
| `mcp --http --addr 127.0.0.1:<free port>` + 두 curl 동시 클라이언트(`/tmp/z-http-smoke.sh`) | 서버 SIGTERM 후 0 | 같은 서버 PID에 client A `docs_index` 200, client B `tools/list` 200. docs 559, text==structured, 도구 51개. 두 도구의 annotations는 `{idempotentHint:false, openWorldHint:false, readOnlyHint:true}`다. bearer 없음 401, `Origin: http://evil.example` 403, `authority_file` 없는 `project_docs_route`는 200 + `isError:true`, `error_code:authority_required`. 응답·헤더·서버 stdout/stderr에 bearer 0건. 종료 뒤 listener closed |
| `trace analyze --input /tmp/issueops-ten-improvements-01a0fa31/claude-stream-real.jsonl --input-format claude-stream --json` | 0 | `complete:true`, `usage.coverage:complete`, warnings 없음. haiku 927/19/0/0 USD 0.001022, sonnet 4/286/37902/603 USD 0.0128604로 parent 실측과 같다 |

첫 HTTP smoke 실행은 스크립트 버그로 멈췄다. 마지막 `wait`가 서버 job까지 기다렸기 때문이다. session을 종료하고 client PID만 기다리도록 고쳐 다시 실행했다(위 결과). 남은 서버 프로세스와 listener는 없다.

### Self-verify

실제 `$HOME`에 쓰지 않는 방법으로 실행했다. 저장소의 `bin/issueops`는 쓸 수 없고, `install`은 canonical `bin/issueops` 또는 같은 디렉터리의 staged binary만 허용한다(`/tmp/z-bin`에서 실행하면 `native install candidate must be the canonical target...`, exit 1). 그래서 `.git`을 포함한 사본 `/tmp/z-svroot.*/issueops`를 만들었다(`bin`·`.omo`·`.codegraph` 제외, `git status --porcelain` 해시가 원본과 같음). 사본에서 `bin/issueops`를 빌드하고, 임시 HOME/CODEX_HOME/state에 `install --mcp-transport=stdio --path-mode=skip --json`(exit 0, ok)을 실행한 뒤 `self-verify --seed=100 --target-score=95 --llm-eval=false --json`을 실행했다. self-verify의 build 단계는 temp binary로 빌드하고(`internal/application/selfverify/steps.go:95-96`), native integration은 HOME을 읽기만 한다(`validation_native_integration.go:14-33`). 실제 `io.issueops.mcp` label은 실행 전후 모두 exit 113이었다.

| 명령 | exit | 결과 |
|---|---:|---|
| 1차 `self-verify ...` (`/tmp/z-svroot.*` 사본, `mon_J5JMHW1E51C78CAA`) | 중단 | monitor 기본 timeout이 프로세스를 종료해 출력이 0바이트다. 결과로 세지 않는다 |
| 2차 `self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json` (같은 `/tmp` 사본, `mon_SSARVERWFPKYG331`) | 1 | `ok:false`, 점수 없음(fail-fast). `failed_step: risk QA tier`, `risk QA race test: exit status 1`, 마지막 성공 단계는 `Go test match guard`(504409ms) |
| 원인 격리: 같은 사본에서 `go test -race ./... -count=1` (`mon_EGJPZM8R18XXSGCV`) | 1 | 실패는 `cmd/issueops/installcli`의 `TestInstallCommandDryRun{JSONDispatches,AutoPathMode...,ManualPathMode...,SkipPathMode...}` 4건뿐이고 `DATA RACE`는 0건이다 |
| `go test ./cmd/issueops/installcli -run TestInstallCommandDryRunSkipPathModeDoesNotPlanShellRC$`, 사본을 `/tmp/...` 경로로 연 경우 | 1 | 재현된다 |
| 같은 테스트 묶음, 같은 사본을 실제 경로 `/private/tmp/...`로 연 경우 | 0 | ok |
| 같은 테스트 묶음, 원본 저장소 | 0 | ok |
| 3차 설치와 `self-verify` (사본 실제 경로, 새 임시 HOME/state) | 0 | install exit 0(`root=/private/tmp/z-svroot.*/issueops`). `self-verify` exit 0, `ok:true`, `termination_eligible:true`, target 95, `minimum_goal_score` 100, 28단계 중 실패 0, 366730ms. 실행 뒤에도 실제 `io.issueops.mcp` label은 exit 113이고 `~/Library/LaunchAgents`의 issueops 파일은 0개다 |

## 4. 남은 실패와 원인

- Z 범위의 Go 테스트 실패는 0건이다. 최종 `go test ./... -count=1`과 `go test -race ./... -count=1`이 모두 exit 0이고, 사전 실패 4패키지 5건은 모두 해소됐다.
- self-verify 2차 실패(`risk QA race test`)의 원인은 검증 환경이다. 사본을 symlink인 `/tmp`(macOS에서 `/private/tmp`) 아래에서 열었기 때문이다. `installCommandFixture`는 기대 root를 `filepath.Abs("../../..")`로 잡고(`cmd/issueops/installcli/install_command_helpers_test.go:23`), installer가 계획한 link target과 문자열로 비교한다(`:129-136`). 실제 경로에서 열면 같은 사본에서도 통과하고, 원본 저장소의 race 전체도 통과했다. Z 변경이 만든 회귀는 아니다. 다만 symlink된 checkout 경로에서 이 테스트가 깨지는 것은 기존부터 있던 테스트 취약점이다(owner: installcli/T11, 후속 작업으로 남김).
- 저장소 `bin/issueops`의 변화를 관찰했다. Z 시작 시 sha256은 `cf50bb3b…cd4`였고, 2026-10-03T01:06:26에 수정돼 지금은 `f19fd7ca…2ea`다. 이 값은 현재 작업 트리를 빌드한 `/tmp/z-bin/issueops`와 같다. Z가 기록한 빌드는 모두 `/tmp/z-bin` 또는 `/tmp` 사본의 `bin/`을 대상으로 했다. 01:06 무렵 Z가 저장소에서 돌린 명령은 없다. Python suite는 01:00:32, race는 01:02:35에 끝났고, self-verify는 사본에서 돌았다. stability-audit 스크립트의 build 대상은 `Path.cwd()/bin/issueops`(`e2e_stability_audit.py:25-26`)다. `~/.local/bin/issueops` shim(09-07), `~/.codex/config.toml`(10-02 16:58), `~/.omo/mcp.json`(10-01)도 바뀌지 않아 update가 실행된 흔적은 없다. 동시에 작업하는 다른 세션이 다시 빌드한 것으로 보이지만 주체는 확인하지 못했다. 실제 stdio host는 이제 현재 소스 build를 쓴다. parent가 의도한 것인지 확인해야 한다.
- docs checker 위반 1건은 gitignored 기존 증거 파일에서 나왔다(§3). Z가 고칠 대상이 아니다.
- 본 계약 §6의 "세 host 각각 native authorize 뒤 HTTP prepare/claim·holder mutation" 실제 host 시나리오는 이번 Z battery에서 실행하지 않았다. 범위는 HTTP/stdio 발견·docs_index와 capability 미제시 거부까지다. host 실사용 연결 증거는 `host-qa-preflight.md`의 parent 실측에 있고, lifecycle mutation은 `TestHTTPExecutionActionsUseVerifiedCallerWithoutServerAncestry` 등 단위·통합 테스트로만 검증돼 있다(owner: parent/H, 후속 실측).

## 5. Z가 새로 찾은 후속 결함 (수정하지 않음, production 코드 범위 밖)

- **Omo HTTP 설정의 catalog cache token 누락 (owner J, medium).** stdio entry는 `env.ISSUEOPS_MCP_CATALOG_SHA256`을 넣지만(`internal/adapter/omo/mcp.go:83-90`), HTTP entry는 `type/url/headers`뿐이다(`mcp.go:50-54`). 설치된 Omo는 server config 전체의 SHA-256(`dist/core/extensions/builtin/mcp/config.js:240-247`의 `hashConfig`)을 키로 tool catalog를 최대 7일 재사용한다(`catalog-cache.js:6,20-28`, `CACHE_TTL_MS`). 그래서 HTTP 설치에서는 catalog가 바뀌어도 config hash가 그대로이고, Omo 새 세션이 이전 `tools/list`·schema를 쓸 수 있다. `operations/install.md`에 증상과 우회(캐시의 issueops 항목 삭제 뒤 재연결)를 적었다. 수정안은 HTTP entry에 catalog를 반영하는 무해한 필드(예: URL query 또는 header)를 넣고 Omo가 그 값을 hash에 포함하는지 실측하는 것이며, J의 installer와 테스트 변경이 필요하다.
- `issueops bootstrap` usage에 기존부터 `[--sync]`가 있지만 `updatecli.Command`(`update_bootstrap.go:16-23`)는 `--sync`를 파싱하지 않는다. Z 이전부터 있던 불일치라 usage에서 지우지 않았다.
- `.issueops/evidence/delivery-20261001/535/review-input.md`의 broken link는 gitignored 기존 증거 파일이다(docs checker 위반 1건).
