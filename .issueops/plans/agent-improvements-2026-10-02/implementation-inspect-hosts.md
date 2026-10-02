# K 구현 보고: host별 관측과 receipt 수집 (`inspect`, `harness_inspect`)

범위: contract.md §4 "I3 / I4" 중 I3와 §5 K 행이다. 구조화 결과 schema(L), golden·inventory·운영 문서(Z), 설치기(J)는 건드리지 않았다. commit, 전역 설치, 실제 `$HOME` 설정 접근은 없다. 실측은 모두 `/tmp/kinsp-1790951136` 아래 격리 HOME·CODEX_HOME·state·root 사본과 테스트 전용 launchd 라벨(`io.issueops.mcp.ktest-kinsp-1790951136`)로 했다.

## 결과 요약

`IntegrationStatus.Hosts []HostIntegration`이 codex, claude, omo 세 host를 이 순서로 항상 담는다. 기존 JSON 필드는 그대로다.

| 관측 | 판정 근거 | receipt 필요 |
|---|---|---|
| Installed | 스킬 경로의 `SKILL.md`가 심볼릭 링크를 따라 읽힌다 | 아니오 |
| Linked | 스킬 경로가 심볼릭 링크이고 `<root>/skills/<name>`으로 해석된다 | 아니오 |
| Configured | host 설정에 `issueops` entry가 정확히 하나 있고 transport가 확정된다 | 아니오 |
| Discovered | host가 받은 실제 catalog에 `docs_index`가 있다(receipt feature `tools/list`) | 예 |
| Connected | host가 실제로 `docs_index`를 호출해 성공했다(feature `docs_index`) | 예 |
| Protocol | host 쪽 artifact에 협상된 revision이 있다 | 예 |

- 기본 inspect는 파일 읽기와 파싱만 한다. receipt가 없으면 뒤 세 관측은 `not_checked`(reason `host_receipt_required`)다. 프로세스 실행도 하지 않는다.
- `inspect --host-receipts FILE`과 MCP `harness_inspect.host_receipts`가 schema_version 1 receipt를 읽는다. 알 수 없는 필드, 중복 host, 다른 schema_version, 1MiB 초과, 후행 데이터는 모두 세 관측을 `unknown`으로 만들고 reason을 남긴다. inspect 자체는 `ok=true`로 끝난다.
- receipt 관측이 `verified`로 남으려면 다음이 모두 맞아야 한다. 하나라도 틀리면 `unknown`으로 내리고 reason에 이유를 적는다. `failed`·`unknown`·`not_checked`로 보고된 관측은 stale 검사와 artifact 검사만 통과하면 그대로 전달한다.

| 검사 | reason(어긋날 때) |
|---|---|
| receipt의 transport가 현재 설정의 transport와 같다 | `receipt_stale_transport` |
| `config_sha256`이 현재 계산값과 같다 | `receipt_config_sha_missing`, `receipt_stale_config` |
| `host_version`이 현재 host의 `--version` 출력과 같다(semver 부분 비교) | `receipt_host_version_missing`, `host_version_unobservable`, `receipt_stale_host_version` |
| `source`가 실제로 존재하는 비어 있지 않은 일반 파일이고 URL·`scheme:`·`synthetic` 이름이 아니다. 상대 경로는 receipt 파일 위치 기준이다 | `receipt_source_not_artifact`, `receipt_source_synthetic` |
| `observed_at`이 RFC3339다 | `receipt_observed_at_invalid` |
| Discovered는 `tools/list`, Connected는 `docs_index` feature가 있다. Protocol은 requested·negotiated revision 중 하나가 있다 | `receipt_missing_tools_list`, `receipt_missing_docs_index`, `receipt_missing_revision` |

- `config_sha256`은 비밀을 제거한 **issueops entry**의 canonical JSON(SHA-256)이다. `headers`·`http_headers`·`env`의 값은 `<redacted>`로 바꾸고, `token|secret|password|authorization|api_key`에 맞는 키의 값도 가린다. 계약은 "parsed config 전체"라고 적었지만 `~/.claude.json`은 `numStartups` 같은 값이 매 실행마다 바뀌어 receipt가 항상 stale이 되므로 entry 단위로 좁혔다. bearer를 회전해도 해시는 그대로이고, URL이나 command가 바뀌면 달라진다(테스트로 고정).
- 현재 host 버전은 `Observer.HostVersion` seam으로 얻는다. 운영 wiring은 `CommandHostVersion`을 쓴다. 이 함수는 `codex`·`claude`·`omo` 세 이름만 PATH에서 찾아 `--version`을 10초 제한으로 실행한다. **receipt를 줬을 때만** 실행한다.
- `Observer.ReceiptRoot`가 있으면 receipt 파일은 그 root 안에 있어야 한다(상대 경로는 root 기준, 심볼릭 링크는 해석 후 판정, 밖이면 `receipt_outside_workspace`). MCP 경로(`scopedHarnessInspector`)가 요청 root를 넣는다. CLI는 root 제한이 없다. 공유 HTTP 서버가 요청자가 지정한 임의 경로를 읽지 않게 하려는 경계다.

## 변경 파일

| 경로 | 내용 |
|---|---|
| `internal/contract/inspect/types.go` | `Options{CodexHome,HostReceipts}`, `HostIntegration`, `Observation`, `HostReceipts`, status·host 상수, `IntegrationStatus.Hosts` |
| `internal/adapter/inspect/inspect.go` | `Inspect(root,target,home,version,skillName,options)`, `Observer`에 `Now`·`HostVersion`·`ReceiptRoot` seam |
| `internal/adapter/inspect/hosts.go` (신규) | host별 경로, 스킬 Installed/Linked, Configured 관측 |
| `internal/adapter/inspect/hostconfig.go` (신규) | Codex TOML의 `[mcp_servers.issueops]` 표와 JSON `mcpServers.issueops` 해석, 중복·malformed 판정, 비밀 제거 hash |
| `internal/adapter/inspect/receipts.go` (신규) | receipt 읽기·검증·demote, `CommandHostVersion` |
| `internal/adapter/inspect/hosts_test.go` (신규), `inspect_test.go` | 아래 테스트, 기존 호출 시그니처 갱신 |
| `cmd/issueops/basiccli/{command,inspect_doctor_cli}.go` | `InspectHarness func(string, inspectcontract.Options)`, `--host-receipts` 플래그, 텍스트 출력에 host당 한 줄 |
| `cmd/issueops/basiccli/{basic_composition_test,inspect_doctor_cli_test}.go` | 시그니처 갱신, 플래그 전달 테스트 2개 |
| `cmd/issueops/mcpcli/{mcp_tools,mcp_tool_project}.go` | `MCPDependencies.Inspect func(repo, hostReceipts string) any`, `host_receipts` 인자 한 개 전달 |
| `cmd/issueops/mcpcli/{mcp_tool_project_test,instance_services_test}.go` | 인자 전달 테스트, 시그니처 갱신 |
| `cmd/issueops/issueopsapp/app.go` | `harnessInspectorWithDefault(defaultTarget, receiptRoot)`, `newHarnessHostInspector`, `scopedHarnessInspector`, `CODEX_HOME` 환경 변수를 `Options.CodexHome`으로, `HostVersion`·`ReceiptRoot` 주입. `newHarnessInspector()`는 status용으로 `func(string)` 시그니처를 유지한다 |

범위 밖이지만 빌드를 깨지 않으려고 한 줄씩 고친 곳이 있다(Z가 알아야 한다): `issueopsapp/basic_wiring.go`의 `InspectHarness: newHarnessHostInspector()`, `issueopsapp/mcp_facade.go`의 stdio 의존성(`Inspect: scopedHarnessInspector(resolveTarget(""))`, 지역 변수 `inspector` 삭제), `issueopsapp/assistant_wiring_test.go`의 `d.Inspect("", "")`. `mcp_facade.go`의 다른 diff는 H/J 몫이다.

## RED → GREEN

구현을 먼저 쓰고 테스트를 나중에 썼다. 그래서 RED는 변이(mutation)로 확인했다. 각 줄을 무력화하고 실패를 본 뒤 원복했다.

| 무력화한 규칙 | 실패한 테스트 |
|---|---|
| config hash 비교 | `TestHostsReceiptStalenessDemotesVerifiedToUnknown` |
| host version 비교 | 같은 테스트 |
| JSON 중복 entry 판정 | `TestHostsConfigFailuresCarryDistinctReasons` |
| `env`·`headers` 값 제거 | `TestHostsCodexParserKeepsStdioEnvAndSecretsOutOfHash` |
| `synthetic` 이름 거부 | `TestHostsReceiptSyntheticOrMissingSourceNeverVerifies` |
| 복사본(`not_symlink`) 판정 | 컴파일 오류로 실패(변이 후 `isLink` 미사용) |

`scheme:` 거부 한 줄을 무력화하면 테스트가 통과한다. 그 source는 파일이 존재하지 않아 `os.Stat` 검사가 같은 `receipt_source_*` reason으로 막기 때문이다. 이 검사는 존재하는 파일 이름이 `scheme:` 형태일 때를 위한 방어선이다.

테스트(모두 임시 HOME, `t.TempDir`, 고정 `Now`, `HostVersion` seam 사용, sleep 없음): 기본 상태(여섯 관측과 증거), JSON 필드 유지와 bearer 미유출, hash의 비밀 무시·endpoint 추적, `CODEX_HOME` 이동, broken symlink, 복사본/잘못된 대상/스킬 없음, 설정 없음·malformed·entry 없음·중복(Codex TOML 표 중복, JSON 키 중복)·transport 없음·모호, stdio+env의 비밀 제거, receipt 정상 검증, stale 6종, synthetic/없는 source 5종, 상대 source, 필요한 증거 누락, failed 관측 유지, receipt 파일 문제 6종, 읽을 수 없는 파일, `ReceiptRoot` 경계(상대·절대·심볼릭 링크·`..`), 버전 정규화, `CommandHostVersion`(허용되지 않은 host, 없는 바이너리, 가짜 바이너리). basiccli는 플래그 전달·미지정 두 테스트, mcpcli는 `host_receipts` 전달(있음/없음) 테스트를 더했다.

## 명령 결과

| 명령 | 결과 |
|---|---|
| `gofmt -l`(추적·미추적 `*.go` 전체) | 출력 없음 |
| `go build ./...` | 오류 없음 |
| `go vet ./...` | 출력 없음 |
| `go test ./internal/adapter/inspect ./cmd/issueops/basiccli ./cmd/issueops/mcpcli ./cmd/issueops/statuscli ./internal/application/status -count=1` | 모두 `ok` |
| `go test -race ./internal/adapter/inspect ./cmd/issueops/basiccli ./cmd/issueops/mcpcli -count=1` | 모두 `ok` |
| `go test ./... -count=1` | 실패 두 건만 남았다. 둘 다 K 몫이 아닌 후속 갱신이다 |

`go test ./...`의 실패:

- `issueopsapp`의 `TestResponseContractsGolden`: `golden mismatch for response_contracts.golden.json: $.cli.inspect.integration.hosts unexpected in actual value`. 새 필드 때문이며 golden은 Z가 갱신한다.
- `internal/architecture`의 `TestDDDResponsibilityInventoryMatchesSource`: `production file or symbol has no recorded responsibility`. J가 끝냈을 때부터 RED였고 K의 신규 파일·심볼이 더해졌다. 인벤토리는 Z가 갱신한다.

## 실제 격리 실측

J의 installer가 만든 구성으로 시작했다. 현재 작업 트리를 `.git` 없이 rsync한 root 사본에 바이너리를 빌드하고 `HOME=$T/home CODEX_HOME=$T/home/.codex ISSUEOPS_STATE_DIR=$T/state ISSUEOPS_ROOT=$T/root ISSUEOPS_MCP_SERVICE_LABEL=io.issueops.mcp.ktest-…`로 `issueops install --path-mode=skip --mcp-transport=http`를 실행했다(exit 0, ok). `mcp service status`는 `running`, pid 4107이었다. 시작 전에 47831 포트는 비어 있었고 실제 `io.issueops.mcp` 라벨은 로드되지 않았다(`launchctl print` exit 113). 끝나고 `mcp service stop`으로 내렸고 테스트 라벨 0개, 47831 listener 0개, 실제 라벨 exit 113을 다시 확인했다. 격리 디렉터리는 증거용으로 남겼다(config 파일에 격리 bearer가 있고 0600이다. 실제 설정이 아니다).

### 1. 기본 inspect (receipt 없음)

`issueops inspect --json --repo $T/root` → exit 0. 세 host 모두 같은 결과다.

```text
codex  http  installed verified   linked verified   configured verified (config_sha256 11cc7c4cd363…)
claude http  installed verified   linked verified   configured verified (03bc9d748cf3…)
omo    http  installed verified   linked verified   configured verified (03bc9d748cf3…)
각 host  discovered/connected/protocol = not_checked, reason host_receipt_required
```

- config 경로는 `$T/home/.codex/config.toml`, `$T/home/.claude.json`, `$T/home/.omo/mcp.json`이고 `inspect --json` 출력에 `Bearer` 문자열은 0건이다.
- 텍스트 출력에도 `host codex (http): installed=verified linked=verified configured=verified discovered=not_checked connected=not_checked protocol=not_checked` 줄이 나온다.

### 2. 실제 host 실행으로 얻은 artifact

세 host가 installer가 쓴 설정 그대로 같은 서비스(pid 4107)에 연결해 `docs_index`를 호출했다. 설정 파일 외에 override는 없다.

| host | 실행 | artifact | 확인한 내용 |
|---|---|---|---|
| Codex 0.128.0 (`codex-runtime/codex app-server`, `CODEX_HOME=$T/home/.codex`) | `mcpServerStatus/list`, `mcpServer/tool/call` | `$T/artifacts/codex-app-server-run.jsonl`(host 응답 원문 343,621 bytes) | issueops 도구 51개 중 `docs_index` 존재, 호출 `isError=false`, `ok=true`, docs 444 |
| Claude 2.1.287 (`claude --print`, model sonnet, `--strict-mcp-config`, 설치된 `.claude.json`의 issueops entry를 그대로 `--mcp-config`로 전달) | native tool_use/tool_result | `$T/artifacts/claude-stream-run.jsonl`(stream-json 원문) | init에서 `issueops` connected, `mcp__issueops__docs_index` tool_use, tool_result 오류 아님, 최종 `success` |
| Omo 5.1.9 설치된 native transport(`.omo/mcp.json` entry) | `listTools`, `callTool` | `$T/artifacts/omo-transport-run.jsonl` | 51개 도구, `ok=true`, docs 444, spawn한 stdio 자식 없음, 요청 헤더 `Mcp-Protocol-Version: 2025-11-25` |

Codex는 `http_headers` 형식의 bearer를 읽어 인증에 성공했다. J의 http 설정 형식이 실제 host에서 동작한다는 증거이기도 하다. artifact 3개에서 `Bearer` 문자열은 0건이다.

### 3. receipt 파생과 inspect

receipt(`$T/artifacts/host-receipts.json`)는 위 artifact를 스크립트가 파싱해 증거가 있을 때만 만든다. host version은 각 바이너리의 `--version`, `config_sha256`은 1번 출력의 값, `observed_at`은 artifact 파일의 수정 시각이다. Protocol은 Omo만 artifact(요청 헤더 `2025-11-25`)에 revision이 있다. Codex와 Claude는 host 출력에 MCP revision이 없어서 `unknown`/`revision_not_observed_by_host_artifact`로 적었다. 부모 QA에서 proxy로 본 revision(2025-06-18, 2026-07-28)은 다른 설정으로 한 실행이라 가져오지 않았다.

`PATH`에 Codex runtime을 앞세우고 `issueops inspect --json --repo $T/root --host-receipts $T/artifacts/host-receipts.json` → exit 0:

```text
codex  discovered verified  connected verified  protocol unknown revision_not_observed_by_host_artifact
claude discovered verified  connected verified  protocol unknown revision_not_observed_by_host_artifact
omo    discovered verified  connected verified  protocol verified (negotiated 2025-11-25)
installed/linked/configured: 세 host 모두 verified
```

같은 receipt에서 한 가지씩만 바꿔 본 결과다.

| 변형 | 결과 |
|---|---|
| Omo `mcp.json`의 URL만 `:47999`로 변경(검증 후 원복, `cmp`로 원복 확인) | Omo의 세 관측이 `unknown`/`receipt_stale_config`. Configured는 여전히 verified |
| Claude의 `connected.source`를 `synthetic:docs_index-fixture`로, `discovered.source`를 존재하는 `SYNTHETIC-claude.jsonl`로 변경. Codex `connected.source`는 없는 경로로 변경 | Claude connected `unknown`/`receipt_source_not_artifact`, discovered `unknown`/`receipt_source_synthetic`, Codex connected `unknown`/`receipt_source_not_artifact`. 나머지는 verified 유지 |
| receipt의 Codex `connected.host_version`을 `codex-cli 0.127.0`으로 변경 | `unknown`/`receipt_stale_host_version`. discovered는 verified 유지 |

### 4. MCP `harness_inspect`

- 저장소 그대로의 바이너리로 `host_receipts`를 보내면 SDK가 `-32602 invalid_tool_arguments`(`/host_receipts` unknown_key)로 거부한다. catalog schema에 속성이 아직 없기 때문이며 L/Z 몫이다. 인자 없이 호출하면 정상이고 세 관측이 `not_checked`로 나온다.
- plumbing은 임시 사본(`$T/src-variant`)에서 `tool_schemas.go`에 `host_receipts` 속성 한 줄만 더해 빌드한 스크래치 바이너리로 확인했다. 저장소 파일은 바꾸지 않았다. stdio MCP에서 작업 디렉터리 안의 상대 경로 receipt는 위 3번과 같은 결과(codex·claude discovered/connected verified, omo protocol verified)였고, 작업 디렉터리 밖의 절대 경로 receipt는 세 host 모두 `unknown`/`receipt_outside_workspace`였다.
- 공유 HTTP 서비스로는 `host_receipts`를 실행하지 못했다. 위 schema 거부 때문이다. 서비스(launchd)의 기본 PATH에는 host 바이너리가 없어서 현재 버전을 얻지 못하고 `host_version_unobservable`이 될 것으로 예상하지만 실행으로 확인하지는 않았다.

## 알려진 한계

- Codex와 Claude의 Protocol은 host artifact에서 revision을 얻을 수 없어 `unknown`이다. revision을 보려면 본 proxy 방식이 필요한데 그러면 설정 URL이 달라져 `config_sha256`이 어긋난다.
- receipt의 `host_version`은 `--version` 출력의 첫 semver 부분을 비교한다. 같은 버전의 다른 빌드는 구분하지 못한다.
- Codex TOML은 `[mcp_servers.issueops]` 표, 하위 표, `[mcp_servers]` 안의 `issueops = {…}` 인라인 형태만 읽는다. 여러 줄 문자열(`"""`)이나 dotted key(`mcp_servers.issueops.url = …`)는 지원하지 않으며, 이 경우 `config_malformed`나 `entry_missing`으로 보고된다.
- Installed/Linked는 `atomic-commit-push` 스킬 하나만 본다(기존 inspect와 같다). Omo의 lifecycle extension은 보지 않는다.
- `CODEX_HOME`은 inspect를 실행한 프로세스의 환경 변수를 쓴다. launchd로 뜬 공유 서비스에는 보통 없어서 `<home>/.codex`로 떨어진다. 기존 `codex_skill_path` 등 평면 필드는 계속 `<home>/.codex` 기준이다(JSON 필드 불변 요구).

## 남은 작업

- **L**: `harness_inspect`의 input schema(`internal/contract/mcp/tool_schemas.go`)에 `host_receipts`(string)를 추가한다. 이 속성이 없으면 MCP에서 `-32602`로 거부된다. output schema는 `integration.hosts` 배열, `Observation.features`가 항상 배열(`[]`)이라는 점, 모든 관측 필드가 항상 존재한다는 점을 모델링한다.
- **Z**
  - `cmd/issueops/testdata/response_contracts.golden.json`을 갱신한다. 현재 `TestResponseContractsGolden`이 `$.cli.inspect.integration.hosts unexpected in actual value`로 실패한다. MCP `harness_inspect` 투영과 status의 inspect 부분도 같은 필드를 포함할 수 있다.
  - `TestDDDResponsibilityInventoryMatchesSource`(J 변경으로 이미 RED)에 K의 신규 production 파일 `internal/adapter/inspect/{hosts,hostconfig,receipts}.go`와 신규 심볼을 등록한다. 대상은 `inspect.CommandHostVersion`, `Observer.{Now,HostVersion,ReceiptRoot}`, `inspectcontract.{Options,HostIntegration,Observation,HostReceipts}`와 상수, `issueopsapp.newHarnessHostInspector`다.
  - 호출자 정리: 시그니처가 바뀐 곳은 `Observer.Inspect`, `basiccli.Command.InspectHarness`, `mcpcli.MCPDependencies.Inspect`다. 이 중 K 범위 밖은 위 "범위 밖 한 줄" 세 곳이며 이미 고쳤다. `statuscli`·`internal/application/status`는 `func(string)` 시그니처를 유지해 영향이 없다. `.issueops/evidence/**`의 `InspectHarness` 문자열은 증거 보관용이라 손대지 않았다.
  - 운영 문서(OPERATIONS 등)에 `inspect --host-receipts`와 receipt schema(schema_version 1, 위 검증 표), `harness_inspect.host_receipts`의 workspace 경계, `CommandHostVersion`이 receipt 사용 시에만 host 바이너리를 실행한다는 점, reason 코드 목록을 반영한다.
- **J/H 확인 요청**: 공유 서비스(launchd)의 PATH 한계가 `host_receipts` 사용에도 영향을 준다. 서비스에서 receipt를 쓰려면 host 바이너리 위치를 알 방법이 필요하다(J가 보고한 PATH 결정과 같은 사안).
- doctor의 `loopbackMCPEndpoints`가 issueops http entry를 구분할지(J가 K에 넘긴 항목)는 inspect 범위가 아니라서 건드리지 않았다.
