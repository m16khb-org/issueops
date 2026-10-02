# I3/I4/I5: host inspection과 MCP 계약

기준 HEAD는 `92eaa00143841f964c285dacdd41aedbeeecdf2e`다. `brief.md`의 승인 범위를 따른다. 이 노드는 설계만 작성하며 코드, 설정, dependency, 설치를 변경하지 않는다. 실제 `PI_MODEL=gpt-6.1-sol`이며 `PI_THINKING_LEVEL`은 export되지 않았다.

## 1. 파일 소유권과 선행 조건

| 구현 담당 | 독점 수정 범위 | 의존 관계 |
|---|---|---|
| I3 inspection | `internal/contract/inspect/types.go`, `internal/adapter/inspect/inspect.go`, `inspect_test.go`, `cmd/issueops/issueopsapp/app.go` | I10이 확정한 transport/config receipt를 읽는다. 설치 파일은 수정하지 않는다. |
| I4 catalog/result | `internal/contract/mcp/catalog_types.go`, `tool_schemas.go`, `internal/adapter/inbound/catalog/mcp/catalog.go`, 해당 catalog tests, `cmd/issueops/mcpcli/mcp_sdk_server.go`, `mcp_sdk_server_test.go` | I3 DTO 확정 후 inspect schema를 봉인한다. SDK 변경과 같은 파일을 병렬 수정하지 않는다. |
| I5 dependency/compatibility | `go.mod`, `go.sum`, 새 `cmd/issueops/mcpcli/mcp_revision_compatibility_test.go` | parent가 dependency 갱신을 먼저 수행한다. I4 adapter 갱신 후 I10 listener와 통합한다. |
| parent integration | `cmd/issueops/testdata/*.golden.*`, `cmd/issueops/issueopsapp/response_contract_*`, API 문서 게이트 | 모든 DTO와 transport 확정 후 golden을 한 번 갱신한다. |
| I10 configuration | Codex `install_config.go`/`activation.go`, Claude `install_mcp.go`/`activation.go`, Omo `mcp.go`/`activation.go`, 각 installer tests, `configs/{codex,claude,omo}/` | HTTP URL, credential 전달, lifecycle, 요청별 authority를 소유한다. I3/I4는 credential 값을 읽거나 출력하지 않는다. |

위 host adapter 경로의 prefix는 `internal/adapter/{codex,claude,omo}/`다. `mcp_sdk_server.go`의 options/register 함수가 I10에도 필요하므로 I4가 먼저 수정하고, I10이 후속 수정한다. `mcp_transport.go`와 새 HTTP inbound 파일은 I10 소유다. 공용 서버에서도 mutation을 지원하며 세션별 상주 proxy를 추가하지 않는다.

## 2. I3: 설치와 실제 연결을 분리한다

현재 `inspect.go:15-40`은 Codex/Claude만 표시한다. `CodexMCPConfigured`는 TOML section 문자열 포함 여부만 확인하고, Claude project MCP는 harness root의 파일 존재만 확인한다. `app.go:newHarnessInspector`는 `CODEX_HOME`을 전달하지 않는다. `InspectInfo`의 기존 JSON 필드는 유지하고 `integration.hosts`를 추가한다.

호스트마다 `host`, `config_path`, `transport`, `skill_path`, 그리고 다음 독립 관측을 제공한다. 각 관측은 `status=verified|failed|unknown|not_checked`, `source`, `observed_at`, redacted `reason`을 가진다. 단일 ready 점수로 합치지 않는다.

- **installed:** binary/skill 파일이 존재하고 읽힌다.
- **linked:** symlink가 canonical `skills/` 원본을 가리킨다. 파일 존재와 다르다.
- **configured:** 대상 server entry를 파싱해 command/args 또는 URL의 의미를 확인한다. 깨진 설정, stale target, 중복 entry를 구분한다.
- **discovered:** 해당 host의 실제 MCP catalog에서 대표 도구를 관측했다. 설정 readback이나 SDK client의 `tools/list`로 대체하지 않는다.
- **connected:** 해당 host가 `docs_index`를 호출하고 결과를 받았다. HTTP listener 응답은 reachability에 불과하다.
- **protocol:** host 버전, transport, 요청/협상 revision, 성공한 feature를 기록한다. generic HTTP 지원은 revision 지원 증거가 아니다.

기본 inspect는 수동 파일 관측만 수행한다. discovery/connectivity는 명시적 host probe receipt가 없으면 `not_checked`다. mock receipt는 live 증거로 승격하지 않는다.

설정 표면은 installer source와 대조한다.

| host | user configuration | 명시적 project-local 및 보조 표면 |
|---|---|---|
| Codex | `$CODEX_HOME/config.toml`, 기본 `$HOME/.codex/config.toml`, `[mcp_servers.issueops]` | `configs/codex/mcp.config.toml`은 harness template다. hooks는 `$CODEX_HOME/hooks.json`, skill은 Codex home의 `skills/`다. |
| Claude | `$HOME/.claude.json`, `mcpServers.issueops` | target `.mcp.json`; `$HOME/.claude/settings.json`, `$HOME/.claude/skills/` |
| Omo | `$HOME/.omo/mcp.json`, `mcpServers.issueops` | target `.omo/mcp.json`의 `issueops_project`; `$HOME/.omo/extensions/issueops.js`, `$HOME/.omo/agent/skills/` |

`{codex,claude,omo}/activation.go:VerifyActivation`은 설정 readback 증거다. host 연결 증거가 아니다. `doctor/checks.go:ProbeGatewayHTTP`는 `2025-03-26 initialize`에 대한 **어떤 HTTP status든** 성공으로 처리하므로 protocol probe에 재사용하지 않는다.

## 3. I4: schema와 구조화 결과를 끝까지 보존한다

정보 손실 지점은 `catalog_types.go:Tool`, `catalog.go:ToolMaps`, `mcp_sdk_server.go:registerAllTools`와 `sdkToolHandlerWithContext`다. SDK-neutral `Tool.OutputSchema`와 `Tool.Annotations`를 추가하고 map assembly와 SDK 등록 양쪽에 전달한다. annotation DTO의 boolean은 pointer로 두어 omitted와 explicit false를 보존한다.

첫 적용은 `harness_inspect`, `docs_index` 두 도구다. 기존 input은 각각 `{repo?:string}`, `{}`다(`tool_schemas.go:CoreProjectTools`). output은 JSON Schema 2020-12 object로 다음 실제 DTO를 모델링한다.

- `harness_inspect`: `ok:boolean`, `version/issueops_root/target_repo/generated_at:string`, `skills:SkillInfo[]`, `docs:string[]`, `integration:IntegrationStatus`; I3 host observations도 명시한다(`internal/contract/inspect/types.go`).
- `docs_index`: `ok:boolean`, `version/issueops_root/generated_at:string`, `docs:DocIndexInfo[]`; 항목은 `rel_path/path/title:string`, `headings:string[]`, `bytes:integer`다(`internal/contract/docs/types.go`).

현재 nil slice가 JSON null로 나올 수 있으므로 schema는 해당 array에 null을 허용한다. 실제 DTO를 바꿔 schema에 맞추지 않는다. 향후 필드 추가를 허용하되 현재 필수 필드와 타입은 검사한다.

기존 JSON text block을 유지하고 같은 JSON 객체를 `structuredContent`로 함께 보낸다. top-level CLI DTO는 그대로다. raw `Server.AddTool`은 자동 output 검증을 보장하지 않으므로 representative 결과를 schema validator로 검사하고 invalid output은 내부 계약 실패로 반환한다. protocol argument error와 tool `isError`를 합치지 않는다. direct Markdown `commit_policy`는 구조화하지 않고 exact text/isError를 보존한다.

두 대표 도구에는 `readOnlyHint=true`, `openWorldHint=false`를 명시한다. mixed-action `issueops_execution`, `state_prune`, `self_verify_candidates(save_state)`는 전체 행동을 기준으로 read-only를 표시하지 않는다. destructive/idempotent는 모든 action에서 증명되는 경우에만 제한하며 보수적 default를 유지한다. `worker_run_read_only`도 job/evidence를 쓰므로 read-only가 아니다. hint는 authorization이 아니다.

## 4. I5: SDK와 wire revision을 따로 검증한다

현재 pin은 `go.mod:33`의 v1.6.1이다. 캐시된 v1.6.1 `mcp/protocol.go:71-90,1325-1383`에도 StructuredContent/OutputSchema/annotations가 있다. I4는 SDK bump에 종속된 기능이 아니다.

v1.8.0 채택 근거는 기존 연구 `fable-5.1-review.md`의 M2 직접 확인이다. 공식 release API의 v1.7.0은 2026-07-28 지원을, v1.8.0은 2026-09-14 정식판이며 추가 revision 없이 그 지원을 유지함을 기록했다. 로컬 module cache에는 v1.6.1만 있었다. parent는 v1.8.0 source와 checksum을 확인한 뒤 migration을 실행한다.

2025-11-25 HTTP는 initialize/session/선택적 GET·replay 모델이다. 2026-07-28은 요청별 metadata, header/body 일치, handshake/session 제거, POST subscriptions 모델이며 요청 SSE 종료를 취소로 처리한다. stdio 취소는 별도 계약이다. 근거는 기존 `mcp-revision-correction.md`, `streamable-spec.md`, `shared-02.md`다. v1.7 release 기록상 새 HTTP에는 `Stateless=true`가 필요하고 stateful은 2025로 내려간다. 따라서 I10은 SDK의 revision-aware binding을 사용하고 자체 protocol을 복제하지 않는다.

미결정: 세 호스트의 정확한 revision 지원과 요청별 credential 전달은 live 검증 전 확정할 수 없다. 지원 가능한 2025 호환 HTTP를 함께 제공할지, 별도 endpoint로 분리할지는 SDK binding 확인 후 parent가 결정한다. 어느 선택에서도 transport session/clientInfo를 actor 권위로 사용하지 않는다.

## 5. 실행 검증 계약

기존 anchors는 `inspect_test.go:TestInspectHarnessIndexesSkillsAndDocs`, `mcp_sdk_server_test.go:TestSDKDirectContentPreservesTextAndError`, `TestSDKCommitPolicyReturnsExactResourceText`, `TestInitSDKServerKeepsDependenciesPerServer`, `mcp_transport_test.go:TestSDKTransportPreservesStructuredToolErrors`다. handshake assertion은 2025 전용으로 유지하고 2026 discovery를 별도 검사한다.

추가 테스트는 임시 HOME의 세 host 설정, CODEX_HOME override, broken/stale symlink, malformed/duplicate config, receipt 없는 discovery를 검사한다. SDK round-trip은 schema/annotation 보존, text와 structured JSON 동등성, schema 위반, null slice, mixed-action hint를 검사한다. revision × stdio/HTTP matrix는 unsupported version, 2025 fallback, 2026 header mismatch, JSON/SSE, subscriptions, disconnect cancellation을 검사한다. HTTP cancellation은 I10의 channel signal에 subscribe한 뒤 stream을 닫는다. sleep/polling은 사용하지 않는다.

golden은 `contractgolden/contract_golden_test.go`가 `cmd/issueops/testdata/{mcp_tools,mcp_resources}.golden.json`을, `issueopsapp/response_contract_golden_test.go:TestResponseContractsGolden`이 `response_contracts.golden.json`을 소유한다. `response_contract_docs_projection_helper_test.go`는 text JSON projection만 확인하므로 structured parity 테스트를 별도로 추가하고 docs required/count projection은 유지한다.

parent 실행 명령:

```bash
go get github.com/modelcontextprotocol/go-sdk@v1.8.0
go mod tidy
go test ./internal/adapter/inspect ./internal/adapter/inbound/catalog/mcp -count=1
go test ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/omo -count=1
go test ./cmd/issueops/mcpcli -run 'TestSDK|TestInitSDK|TestMCPRevision' -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
./bin/issueops api-doc static-check --json
```

실제 세 host 검증은 격리 설정으로 stdio와 직접 HTTP 각각 `docs_index`를 호출하고 host/version/revision/schema/result receipt를 남긴다. `codex mcp get issueops`, `claude mcp list`, Omo 설정 존재만으로 이 검증을 통과시키지 않는다. 이 설계 노드에서는 위 구현 검증을 실행하지 않는다.
