# J/K/L 독립 검증: 서비스·설치, host 관측, 구조화 결과

기준: `contract.md` §3, §4(I3/I4), §5, §6과 기존 stdio 동작. 입력은 `implementation-service-install.md`(J), `implementation-inspect-hosts.md`(K), `implementation-structured-output.md`(L), 작업 트리의 `git diff`다.
이 보고서 외에 저장소 파일은 바꾸지 않았다. 실측은 `/tmp/vjkl/x` 아래 격리 HOME·CODEX_HOME·ISSUEOPS_STATE_DIR·root 사본(`.git` 제외 rsync, `configs/upstream.json`의 plugins/skills는 비움)에서 했다. launchd 라벨은 테스트 전용 `io.issueops.mcp.vtest-vjkl64505` 하나만 썼다.

## 판정 요약

| Owner | 판정 | 결정적 근거 |
|---|---|---|
| J (서비스·설치·갱신) | **conditional** | 서비스 수명·설치·갱신·실패 무변경은 실측으로 모두 통과했다. 직접 `issueops install`이 root 검증 전에 서비스를 기동하는 부작용(D-J1), 계약 밖 빈 `status`(D-J2), 범위 밖 파일 수정(D-J3)이 남아 있다. |
| K (host 관측) | **conditional** | 여섯 관측, receipt 없음/stale/synthetic 이름/schema 판정은 실측대로 동작한다. 다만 이름에 `synthetic`이 없고 비어 있지 않은 파일이면 무엇이든 artifact로 인정해서, `/etc/hosts`를 source로 위조한 receipt가 `verified`가 된다(D-K1). |
| L (구조화 결과) | **conditional** | stdio·HTTP × 2025-11-25/2026-07-28에서 schema·annotation·structuredContent 동등성과 golden 범위가 맞다. 하지만 L의 catalog 변경이 **보고되지 않은 테스트 실패 2건**을 만들었다(D-L1). 상위 문서의 "Z 몫 실패 2건 외에는 없다"는 전제는 틀렸다. |

세 owner 모두 실제 호스트 연결 receipt가 현재 상태 기준으로는 없다. 이 부분은 PASS가 아니라 후속 작업이다(아래 "후속 작업").

## 명령 결과 (이 세션에서 실행)

| 명령 | 결과 |
|---|---|
| `go build -o /tmp/vjkl/issueops ./cmd/issueops` | exit 0 |
| `go vet ./...` | `VET_EXIT=0`, 출력 없음 |
| `gofmt -l $(git ls-files -m -o --exclude-standard '*.go')` | 출력 없음 |
| `go mod tidy -diff` / `go mod verify` | diff 0줄, `all modules verified`. `golang.org/x/time v0.15.0`은 go-sdk v1.8.0의 `go.mod:12` 요구사항이다 |
| `go test ./... -count=1` | `TEST_EXIT=1`. 319개 패키지 ok, 실패 패키지 4개(아래 표) |
| `go test -race -count=1`, 대상: mcpservice, installcli, codex, claude, omo, installutil, application/update, updatecli, inspect, basiccli, mcpcli(+argmap, resources), catalog/mcp, contract/mcp, issueopsapp(+responsecontract), contractgolden | `RACE_EXIT=1`. data race 0건. 실패는 `omo`(TestTrackedTemplatesMatchGeneratedContent), `issueopsapp`(TestResponseContractsGolden) 두 패키지이고 둘 다 아래 표의 비race 실패와 같다 |
| `go test ./cmd/issueops/mcpcli -run '<L 테스트 7개>' -v` | 7개 모두 PASS(`TestSDKInvalidStructuredOutputIsProtocolInternalErrorWithoutPayload` 포함) |

`go test ./...` 실패 전체:

| 테스트 | 원인 | 귀속 |
|---|---|---|
| `issueopsapp/TestResponseContractsGolden` | `$.cli.inspect.integration.hosts unexpected in actual value` | 알려진 Z 몫(K 신규 필드) |
| `architecture/TestDDDResponsibilityInventoryMatchesSource` | 신규 production 파일·심볼 미등록 | 알려진 Z 몫 |
| `architecture/TestDDDContractFunctionsHaveExplicitRoles` | `internal/contract/mcp/output_schemas.go`의 함수 9개에 role 없음 | Z 몫. L이 Z 인계 목록에 적었지만 상위 문서의 "알려진 2건"에는 빠져 있다 |
| `internal/adapter/TestNativeInstallAdapterContractMatrix` | agy·omo MCP config의 `content_sha256` 10곳 불일치 | **L이 유발했고 보고하지 않음(D-L1)** |
| `internal/adapter/omo/TestTrackedTemplatesMatchGeneratedContent` | `tracked Omo MCP config drifted from generated template` | **L이 유발했고 보고하지 않음(D-L1)** |

## J: 서비스·설치·갱신

### 계약 항목별 실측

| 계약 항목(§3/§6) | 증거 | 판정 |
|---|---|---|
| `mcp service start/stop/status --json`, DTO `{ok,status,pid,build_id,url,error_code}`, 비밀 없음 | status `stopped` → start `running pid 67613 build b7b27d15…`(= `shasum -a 256 root/bin/issueops`) → 두 번째 start도 같은 pid 67613 | PASS |
| unit Exec이 절대 binary + `mcp --http`, env는 ISSUEOPS_ROOT·ISSUEOPS_STATE_DIR만 | `plutil -p`: ProgramArguments `[/tmp/vjkl/x/root/bin/issueops, mcp, --http]`, EnvironmentVariables는 이 두 키뿐, `launchctl print` program이 같은 경로 | PASS |
| bearer 0600, 상위 0700, 401/403, identity 헤더 | `mcp-http` drwx------, bearer·server.json·server.lock·server.log -rw-------. bearer 있음 200(헤더 pid·build 있음). 없음/틀림 401, Host `localhost:47831` 403, Origin `http://evil.example` 403. 모두 identity 헤더 없음. 같은 origin은 200 | PASS |
| 단일 인스턴스, 포트 고정 | 같은 state의 foreground: `another issueops mcp http instance holds the service lock (conflict)`. 다른 state: `127.0.0.1:47831 is unavailable (conflict)` | PASS |
| 신원 일치 때만 stop | `server.json` build_id를 바꾸면 status·stop 모두 `conflict/identity_mismatch` exit 1, launchd pid 67613 유지. 원복 뒤 stop은 `stopped`, label exit 113, pid 종료, `server.json` 삭제 | PASS |
| stale | 죽은 pid record → `stale/stale_record` exit 1, 이어서 start하면 `running`(새 pid) | PASS |
| 포트 충돌 시 conflict, job 미로드 | python이 47831 점유 → status·start 모두 `conflict/port_in_use` exit 1, label exit 113 | PASS |
| supervisor 미가용은 명시 오류 | `PATH=/nonexistent mcp service status` → `error_code=supervisor_unavailable` exit 1 | PASS(단, D-J2) |
| 서비스 출력·로그에 비밀 없음 | 실제 bearer 문자열이 server.log, plist, install stdout/stderr, update stdout/stderr에서 모두 0건. server.log는 `method/status/duration_ms`만 남긴다 | PASS |
| dry-run 무쓰기 | 빈 HOME/state에서 `install --dry-run --json`(exit 0, ok, 모든 MCP file `written=false`)과 `install-native.sh --dry-run`(exit 0)을 실행했다. HOME·state 파일 0개, root 트리 해시 불변, `bin/`에 새 파일 없음, label 113, 47831 listener 0 | PASS |
| 세 host entry만 교체, 0600, stdio entry 제거 | 0644 사전 설정(Codex `[mcp_servers.issueops]`+`.env` stdio, Claude stdio entry와 `numStartups`, Omo stdio entry, 각각 `other` entry)에 `install-native.sh` 실행. exit 0, `committed=true`. 세 파일 모두 -rw------- + bearer 1회. Codex는 `url`/`http_headers` block만 남고 `.env` 제거, Claude/Omo issueops는 `{headers,type:http,url}`, `other`와 `numStartups` 보존. agy(`~/.gemini/config/mcp_config.json`)는 `args,command,env` stdio + bearer 0회 | PASS |
| `--mcp-transport=stdio` 보존 | `install --mcp-transport=stdio` exit 0. Claude `args,command,env,type`, Omo `args,command,env`, Codex `command/args/startup_timeout_sec`와 `[mcp_servers.issueops.env]` 복귀. 다시 http로 설치하면 `headers,type,url` | PASS |
| update 순서와 build 교체 | root에 marker 파일을 추가한 뒤 `issueops update`: 로그가 staging → stopping → starting 순서. 전 `running 70458 b7b27d15…`, 후 `running 72434 141bed0b…` = 새 `bin/issueops` 해시, launchd pid 72434 | PASS |
| 실패를 성공으로 기록하지 않음 | 47831 점유 상태에서 세 경로를 실행했다. (a) 직접 `install --mcp-transport=http`: exit 1, `host MCP configs were not changed`. (b) `install-native.sh` 전체 빌드: stop 단계에서 `conflict/port_in_use`로 exit 1, `bin/issueops` 해시 불변. (c) `--skip-build`: seal 단계 exit 1. 세 경우 모두 네 host 설정 해시 불변, label 113 | PASS |
| project-local http는 entry·secret을 만들지 않음 | root의 `.mcp.json`·`.omo/mcp.json`에 `issueops_project`+`keep`을 두고 `install --project-local --mcp-transport=http` → 두 파일 모두 `['keep']`만 남고 bearer 0회 | PASS |

### 결함

- **D-J1 (low, 계약 순서 위반은 아님): 직접 `issueops install`이 root 검증 전에 서비스를 바꾼다.** `cmd/issueops/installcli/install.go:81`의 `applyMCPTransport`가 `:86`의 `executeInstall`(skills 검증 포함)보다 먼저 실행된다. `install_mcp_transport.go:54`의 `Prepare`가 bearer·unit을 쓰고, `:62`의 `EnsureRunning`이 서비스를 기동한다. 실측: `skills/`가 없는 root 사본에서 `install --mcp-transport=http`는 exit 1(`open …/root2/skills: no such file or directory`)이고 host 설정 해시는 그대로였다. 그런데 서비스는 `running pid 85173`(program `…/root2/bin/issueops`)으로 남았다. 같은 root에서 `install-native.sh --skip-build`는 `preflight_install`(dry-run)이 먼저 실패해 서비스가 `stopped`로 남는다.
  - **판단:** §3의 순서는 "credential/unit → build → stop → 교체 → start → 확인 → merge"와 "실패하면 HTTP 설정 성공으로 기록하지 않는다"까지만 정한다. host 설정은 바뀌지 않았고 성공으로 기록되지도 않았으므로 계약 문언 위반은 아니다. 다만 실패한 설치가 실행 중인 서비스라는 부작용을 남긴다.
  - **권고:** `runInstall`에서 root/plan 검증(또는 dry-run plan)을 `applyMCPTransport`보다 앞에 둔다.
- **D-J2 (low): 계약 enum 밖의 빈 `status`.** §3은 status를 running/stopped/stale/conflict로 정한다. `internal/adapter/mcpservice/service.go:114`(supervisor_unsupported), `:117`(supervisor_unavailable), `:138`(state_unreadable), `:305`(binary 읽기 실패)는 `status:""`를 반환한다. `supervisor_unavailable`에서 실측으로 확인했다.
- **D-J3 (범위): J 쓰기 범위 밖의 수정.** §5의 J 행은 installer/activation/templates, `adapter/mcpservice`, service unit, update/bootstrap이다. 다음은 H 또는 Z 소유 파일이다.
  - H 소유 `cmd/issueops/issueopsapp/mcp_http_command.go`: instance lock·record·identity handler 추가.
  - H 소유 `mcp_service_command.go`: 기본 구현을 교체했다.
  - H 소유 `mcp_service_command_test.go`: H 테스트 `TestMCPServiceDefaultReportsMissingSupervisorWithoutClaimingState`를 삭제했다.
  - Z의 composition: `install_wiring.go`, `host_installers.go`, 신규 `mcp_service_wiring.go`.

  모두 J 보고서에 공개돼 있고 H→J 순서를 지켰다. 기능상 필요한 수정이지만 범위 밖 편집으로 기록한다. `port/install.go`의 `MCPTransport/MCPURL/MCPBearer`는 J 보고서의 변경 목록에 없으므로 Z-foundation 몫으로 본다.
- **D-J4 (계약 편차, 수용 가능):** 계약의 "credential/unit 준비 → build" 순서가 실제로는 "build → begin(Prepare) → stop"이다. 준비 코드가 새 build 안에 있으므로 불가피하다. 서비스 중단과 binary 교체 전에 실패한다는 목적은 지켜진다(J 보고서에 공개).
- **관찰(계약 수준의 빈틈, J 결함 아님):** unit이 계약대로 두 env만 넘기므로 서비스 프로세스는 launchd 기본값으로 실행된다. 실측한 서비스 프로세스 env는 `PATH=/usr/bin:/bin:/usr/sbin:/sbin`, `HOME=<실제 사용자 HOME>`이고 `CODEX_HOME`이 없다. 그래서 격리 검증에서도 HTTP 서비스는 HOME 기반 경로를 실제 HOME으로 해석한다(아래 K 참조). 계약에서 PATH·HOME·CODEX_HOME 전달 여부를 정해야 한다.
- **관찰(테스트 위생, low):** `cmd/issueops/issueopsapp/mcp_service_wiring_test.go:52`의 `Stop`이 실제 `gui/<uid>` 도메인에 `launchctl print io.issueops.unit-test`를 실행하고 실제 47831에 dial한다. 읽기 전용이고 라벨도 고유하지만, 실제 서비스가 떠 있으면 결과 status가 달라진다(이 테스트는 URL만 단언하므로 실패하지는 않는다).
- linux systemd는 이 darwin 장비에서 실제로 실행하지 못했다. 렌더링·파싱 단위 테스트만 있다(J 보고서와 같다).

## K: host 관측 (I3)

### 계약 항목별 실측

| 계약 항목 | 증거 | 판정 |
|---|---|---|
| `IntegrationStatus.Hosts`, 기존 필드 유지 | `inspect --json`의 integration 키: 기존 9개 + `hosts`. response golden의 유일한 차이도 `$.cli.inspect.integration.hosts unexpected`이므로 기존 필드는 그대로다 | PASS |
| receipt 없으면 뒤 세 관측은 not_checked | J 설치 직후 세 host 모두 installed/linked/configured `verified`, discovered/connected/protocol `not_checked/host_receipt_required`. 텍스트 출력에 `host codex (http): …` 줄. 출력에 bearer 0건 | PASS |
| broken symlink, malformed, duplicate, CODEX_HOME | HOME 사본에서 Claude skill을 broken symlink로 → installed·linked `failed/broken_symlink`. Omo `mcp.json` 잘림 → configured `failed/config_malformed`. Codex `[mcp_servers.issueops]` 중복 → `failed/entry_duplicate`. `CODEX_HOME`을 빈 디렉터리로 → codex config_path가 그 경로, `failed/config_missing`(평면 `codex_skill_path`는 `<home>/.codex` 유지) | PASS |
| 실제 receipt / stale | K가 남긴 실제 receipt(`/tmp/kinsp-1790951136/artifacts/host-receipts.json`)를 내 환경에 적용했다. claude discovered·connected `verified`, protocol `unknown/revision_not_observed_by_host_artifact`. omo 세 관측은 `unknown/receipt_stale_host_version`(설치된 omo가 5.1.9 → 5.1.10으로 바뀜). codex는 `unknown/host_version_unobservable`(격리 HOME에서 `codex --version` 출력이 비어 있음). config_sha256을 0으로 바꾸면 세 host 모두 `receipt_stale_config` | PASS |
| synthetic source 비승격 | source를 `…/SYNTHETIC-claude.jsonl`로 → `unknown/receipt_source_synthetic` | PASS(이름 기준만, D-K1) |
| host 누락, schema | claude만 있는 receipt → codex·omo `not_checked/no_receipt_for_host`. `schema_version:2` → 모두 `receipt_schema_unsupported` | PASS |
| MCP `host_receipts` 경계 | HTTP `harness_inspect`: 설치 root 밖 절대 경로는 세 host 모두 `receipt_outside_workspace`. root 안 상대 경로는 판정까지 진행했다(결과는 아래 관찰 참조) | PASS |

### 결함

- **D-K1 (medium): receipt source의 진위를 파일 이름과 존재 여부로만 판정한다.**
  - **위치:** `internal/adapter/inspect/receipts.go:208-224`의 `artifact()`. 거부 조건은 URL·`scheme:` 형태, base name의 `synthetic` 포함, 비정규 파일·빈 파일·없는 경로뿐이다.
  - **실측:** 실제 receipt를 복사해 세 관측의 source를 모두 `/etc/hosts`로 바꾸고, protocol을 `verified`/`negotiated_revision:2026-07-28`로, omo host_version을 현재 값으로 고쳤다. 그 결과 claude·omo의 discovered·connected·protocol이 **모두 `verified`** 가 됐다.
  - **함께 드러난 점:** receipt가 현재 환경에 묶여 있지 않다. 다른 HOME·state·bearer·서버 pid에서 만든 K의 receipt도 내 환경에서 claude를 `verified`로 올렸다. 어긋난 `config_path`와 서버 신원은 비교하지 않는다.
  - **계약 위반:** §4 I3의 "receipt의 source는 실제 host run artifact 경로여야 하며 synthetic source는 verified로 승격하지 않는다"와 "실제 host catalog와 docs_index 응답만 발견·연결 증거다"에 어긋난다.
  - **권고:** artifact 내용에서 host가 받은 tools/list와 `docs_index` 호출 결과를 확인하거나, 최소한 receipt의 `config_path`가 현재 경로와 같은지 대조한다.
- **D-K2 (계약 편차, 공개됨):** `config_sha256`이 parsed config 전체가 아니라 issueops entry만 해싱한다(`hostconfig.go`). `~/.claude.json`의 `numStartups` 등이 매번 바뀌기 때문이라는 근거는 타당하다. 계약 문구와 다르므로 Z가 계약 또는 문서를 맞춰야 한다. 실측상 Claude와 Omo entry의 해시가 같다(`03bc9d748cf3…`, 같은 entry 형태).
- **D-K3 (범위):** K 행(inspect DTO/adapter, basiccli 입력, app.go wiring) 밖의 수정이 있다. 모두 K 보고서에 공개돼 있다.
  - H 소유: `cmd/issueops/mcpcli/mcp_tools.go`, `mcp_tool_project.go`(`MCPDependencies.Inspect` 시그니처), `mcp_tool_project_test.go`, `instance_services_test.go`.
  - Z/H composition: `issueopsapp/basic_wiring.go`, `mcp_facade.go`, `assistant_wiring_test.go`.
- **관찰:** 공유 HTTP 서비스에서 `harness_inspect`는 launchd HOME을 읽는다. 격리 서비스의 `config_path`가 `$REAL_HOME/.codex/config.toml`, `$REAL_HOME/.claude.json`, `$REAL_HOME/.omo/mcp.json`이고 transport가 `stdio`로 나왔다(읽기만 했고 실제 HOME은 바꾸지 않았다). 그래서 root 안 receipt가 `receipt_stale_transport`로 판정됐다. 운영에서는 같은 사용자 HOME이라 문제가 없지만, 사용자가 `CODEX_HOME`을 바꿨다면 HTTP inspect가 틀린 Codex 설정을 보고한다. PATH에 host binary가 없어서 생기는 `host_version_unobservable`은 이번 실측의 stale_transport에 가려져 직접 확인하지 못했다.

## L: 구조화 결과 (I4)

### 계약 항목별 실측

| 계약 항목 | 증거 | 판정 |
|---|---|---|
| OutputSchema/annotations는 `harness_inspect`·`docs_index`에만 | stdio 2025-11-25, stdio(2026 요청은 2025-11-25로 협상), HTTP 2025-11-25, HTTP 2026-07-28(`resultType=complete`, `_meta`+`Mcp-Method`/`Mcp-Name` 헤더) 네 경우 모두 51개 도구 중 이 둘만 `outputSchema`·`annotations`를 갖는다. 값은 `{readOnlyHint:true, openWorldHint:false, idempotentHint:false}` | PASS(관찰 참조) |
| structuredContent = text JSON | 네 경우 모두 두 도구 `structured=True`, `json.loads(text)==structuredContent`. `commit_policy`는 structuredContent 없이 Markdown(`--- name: COMMIT_POLICY.md`) 그대로 | PASS |
| invalid output → -32603, payload 없음 | `TestSDKInvalidStructuredOutputIsProtocolInternalErrorWithoutPayload` PASS. 실제 binary에서는 유효하지 않은 출력을 만들 수 없어 단위 테스트 증거만 있다 | PASS(단위 테스트) |
| catalog `Build()` = SDK list | contractgolden(= `Build()`) PASS. 실제 tools/list를 이름 기준으로 golden과 비교하면 SDK가 붙이는 `idempotentHint:false`를 빼고 stdio·HTTP 네 경우 모두 일치한다. 차이는 SDK의 이름순 정렬뿐이다 | PASS |
| golden diff 범위 | HEAD 대비 `mcp_tools.golden.json`을 도구별로 분류했다. 51개 이름은 같다. 변경 도구 26개: `harness_inspect`(annotations, outputSchema, `authority_file/cwd/host_receipts/workspace_root`), `docs_index`(annotations, outputSchema), workspace 도구 24개(없던 authority 필드만 추가. 예: `command_policy_check`·`gates_check`·`worker_run_read_only`는 `authority_file`만, `issueops_execution`은 `authority_file`·`workspace_root`). 삭제·변경된 기존 속성 0개 | PASS |
| `withAuthorityFields` 이동 | `internal/adapter/inbound/catalog/mcp/authority_fields.go`의 25개 집합. `TestWorkspaceAuthorityClassificationMatchesCatalogAuthorityFields` PASS | PASS |
| go.mod | jsonschema-go 직접 의존. `go mod tidy -diff` 0줄, verify 통과 | PASS |
| repo 없는 HTTP `harness_inspect` | `authority_file` 없이 bearer만으로 성공(§2 표의 "repo 없는 호출은 설치 root"와 일치) | PASS |

### 결함

- **D-L1 (medium): 보고되지 않은 회귀 2건.**
  - **증상:** `internal/adapter/TestNativeInstallAdapterContractMatrix`(agy·omo project/user MCP config의 `content_sha256` 10곳)와 `internal/adapter/omo/TestTrackedTemplatesMatchGeneratedContent`(`tracked Omo MCP config drifted`)가 실패한다.
  - **원인:** `cmd/issueops/issueopsapp/host_installers.go:20,74`의 `MCPCatalogSHA256 = SemanticSHA256(mcpcatalog.AdvertisedTools())`가 agy·Omo stdio 설정의 `ISSUEOPS_MCP_CATALOG_SHA256`에 들어간다. L이 `internal/contract/mcp/tool_schemas.go:22-24,45-46`에 `OutputSchema`·`Annotations`·`host_receipts`를 추가하면서 이 해시가 바뀌었다.
  - **원인 격리:** 작업 트리 사본에서 이 세 가지만 되돌렸다. outputSchema/annotations만 되돌렸을 때는 매트릭스가 여전히 FAIL했고, `host_receipts`까지 되돌리자 매트릭스와 omo 템플릿 테스트가 모두 `ok`였다.
  - **영향:** tracked `configs/omo/mcp.json`과 `internal/adapter/testdata/native_install_contract_matrix.golden.json`이 생성값과 어긋난다. L 보고서는 `go test ./...` 결과를 적지 않았고 이 두 실패를 언급하지 않는다.
  - **수정 소유:** golden·tracked template 재생성은 Z(또는 템플릿 owner J) 몫이다. 이 검증에서는 수정하지 않았다.
- **D-L2 (범위):** L 행(MCP catalog types/schemas/assembly, `mcp_sdk_server.go`와 테스트) 밖의 수정이 있다. 모두 L 보고서에 공개돼 있다.
  - F 소유: `go.mod`·`go.sum`.
  - Z 소유: `cmd/issueops/testdata/mcp_tools.golden.json` 재생성.
  - H 소유: `cmd/issueops/mcpcli/mcp_tool_authority.go`(함수 이동으로 삭제), `instance_services_test.go`(fixture를 DTO로 교체).
- **관찰(low):** 실제 tools/list에는 catalog에 없는 `idempotentHint:false`가 나간다(SDK의 non-pointer 필드 때문). 읽기 전용 도구에 "멱등이 아님"을 주장하는 셈이다. 공개돼 있고 계약의 두 hint 요구는 충족한다.

## 기존 stdio 동작

- 격리 HOME에서 `issueops mcp`(stdio)의 initialize, tools/list(51개), `harness_inspect`·`docs_index`·`commit_policy` 호출이 정상이었다.
- `state_read`의 없는 state 오류는 기존대로 `-32602` protocol error다.
- 2026-07-28 initialize 요청은 stdio에서 2025-11-25로 협상됐다.
- `--mcp-transport=stdio` 설치는 세 host의 기존 stdio entry를 복원하고, agy는 transport와 무관하게 stdio다.

## 후속 작업 (PASS로 세지 않음)

- **실제 host 연결 receipt.** 현재 상태 기준의 증거가 없다.
  - K의 artifact(2026-10-02 실행)는 claude만 지금도 `verified`이고, omo는 버전이 바뀌어 stale, codex는 격리 HOME에서 버전을 관측할 수 없다.
  - §6이 요구하는 "세 host 각각 stdio와 직접 HTTP로 tools/list·docs_index, 같은 서버 PID, native authorize 뒤 HTTP prepare/claim과 holder mutation"은 이 검증에서 실행하지 않았다. Z-integration의 실제 표면 검증으로 남긴다.
- D-K1을 고치기 전에는 receipt 기반 `verified`를 연결 증거로 쓰면 안 된다.
- Z: 위 실패 5건(알려진 2건 + `TestDDDContractFunctionsHaveExplicitRoles` + D-L1 2건)을 갱신하고, D-J2·D-K2의 계약/문서 정합과 서비스 env(PATH·HOME·CODEX_HOME) 결정을 반영한다.

## 정리 상태

- 마지막 확인 결과:
  - 테스트 라벨 `launchctl print` exit 113, `launchctl list`의 `vtest` 0개, 47831 listener 0개.
  - 실제 `io.issueops.mcp` exit 113(시작 전과 같음), 실제 `~/Library/LaunchAgents`의 issueops 파일 0개.
  - `io.issueops.unit-test` exit 113.
- 격리 디렉터리 `/tmp/vjkl`(bearer가 든 0600 격리 설정 포함)는 증거로 남겨 두었다.
- 실제 HOME은 쓰지 않았다. 다만 격리 서비스가 launchd HOME으로 실행되면서 HTTP `harness_inspect`가 실제 HOME의 세 host 설정을 **읽었다**. 이 보고서에는 그 경로를 `$REAL_HOME`으로만 적었다.
