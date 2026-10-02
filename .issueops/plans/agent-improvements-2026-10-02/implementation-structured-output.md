# 구현 보고: L, MCP 구조화 결과와 authority 필드 배치

contract.md §4 "I3 / I4"의 I4 부분을 구현했다. `harness_inspect`와 `docs_index`는 기존 JSON text와
같은 object를 `structuredContent`로 돌려주고, 결과는 `outputSchema`로 검사한다. 두
도구만 `readOnlyHint=true`, `openWorldHint=false`를 광고한다. H가 SDK 서버 생성 시점에
붙이던 `withAuthorityFields`는 catalog assembly로 옮겼다. commit은 만들지 않았다.

## 결정

| 항목 | 결정 | 근거 |
|---|---|---|
| Tool 모델 | `mcp.Tool`에 `OutputSchema map[string]any`, `Annotations *ToolAnnotations`를 더했다. `ToolAnnotations`의 네 hint는 모두 `*bool`이다. | contract §I4. 값이 없는 hint는 tools/list에서 빠져 주장으로 읽히지 않는다. |
| ToolMaps | `outputSchema`와 `annotations` key를 값이 있을 때만 넣는다. 나머지 49개 도구의 map은 바뀌지 않는다. | golden 변경 범위를 두 도구와 authority 필드로 한정한다. |
| SDK 등록 | `registerAllTools`가 catalog map의 두 key를 `mcp.Tool`로 옮긴다. | go-sdk v1.8.0의 `ToolAnnotations.ReadOnlyHint`, `IdempotentHint`는 non-pointer라 `Annotations`가 있으면 `false`도 직렬화한다. 그래서 annotation이 없는 도구는 nil을 유지한다. |
| 검증기 | `google/jsonschema-go`(go-sdk가 쓰는 라이브러리)를 직접 의존으로 올렸다. | 기존 `toolconformance.Validate`는 `type` 배열, `$defs`, `$ref`를 지원하지 않는 닫힌 부분집합이라 2020-12 schema를 검사할 수 없다. |
| schema 모델 | DTO의 JSON 필드와 정확히 같은 `properties`/`required`, `additionalProperties:false`, 모든 slice는 `["array","null"]`이다. `status`는 Go 타입처럼 enum 없이 string이고 값은 description에 적었다. | nil slice가 `null`로 직렬화된다. enum을 두면 빈 status 하나로 도구 전체가 -32603을 반환한다. DTO와 schema 필드 일치는 reflect 테스트가 지킨다. |
| 결과 변환 | `structuredContent`는 payload를 JSON object로 풀어 schema를 통과시킨 값이다. text는 기존과 같이 `MarshalIndent(payload)`다. | 두 값이 같은 object임을 테스트가 비교한다. |
| 오류 구분 | `IsError` 결과는 text만 보내고 `structuredContent`를 붙이지 않는다. direct Markdown은 그대로 둔다. schema 불일치와 schema compile 실패는 protocol error -32603 "Invalid tool output"이며 메시지와 data에 tool 이름만 있고 payload는 없다. | 테스트가 sentinel 문자열이 오류에 없음을 확인한다. |
| authority 필드 | `withAuthorityFields`를 `internal/adapter/inbound/catalog/mcp/authority_fields.go`로 옮겼다. `Build()`가 적용하므로 `Build()`, golden, SDK tools/list가 같은 schema를 가진다. `newSDKServer`의 호출은 지웠다. | catalog 패키지는 mcpcli를 import할 수 없어 workspace 도구 이름 집합(25개)을 자체로 둔다. mcpcli 테스트가 이 집합과 `mcpToolAuthorities`의 workspace 분류가 같음을 강제한다. |

## 범위 밖이지만 함께 고친 두 가지

1. `harness_inspect`의 입력 schema에 `host_receipts`가 없었다. K가 handler에서 읽도록 해 뒀지만 닫힌 입력 검증이 거부하므로 MCP 호출자는 값을 보낼 수 없었다. contract §I3가 요구하는 입력이라 schema에 추가하고, 값이 inspector까지 전달되는지 테스트했다.
2. `instance_services_test.go`의 `harness_inspect` fixture가 `{"owner": ...}` map을 반환했다. 새 검증이 이를 -32603으로 거절해 실패했다. DTO를 쓰도록 `InspectInfo{OK: true, TargetRepo: owner}`로 바꿨다. 단언 내용(인스턴스 격리)은 그대로다.

## 변경 파일

- 새 파일: `internal/contract/mcp/output_schemas.go`, `internal/adapter/inbound/catalog/mcp/authority_fields.go`, `internal/adapter/inbound/catalog/mcp/structured_output_test.go`, `cmd/issueops/mcpcli/mcp_structured_output_test.go`
- 수정: `internal/contract/mcp/{catalog_types,tool_schemas}.go`, `internal/adapter/inbound/catalog/mcp/catalog.go`, `cmd/issueops/mcpcli/{mcp_sdk_server,mcp_tool_authority}.go`, `cmd/issueops/mcpcli/instance_services_test.go`
- `go.mod`, `go.sum`: `go mod tidy` 결과다. `jsonschema-go`의 `// indirect`가 빠졌고 `golang.org/x/time`이 추가됐다. 같은 파일에 다른 owner의 변경(go-sdk v1.8.0)이 이미 있어 diff가 섞여 있다.
- `cmd/issueops/testdata/mcp_tools.golden.json`: 아래 "golden"을 본다.

## RED → GREEN

| 테스트 | RED | GREEN |
|---|---|---|
| catalog: `TestOnlyStructuredToolsAreReadOnlyClosedWorld`, `TestToolMapsCarryOutputSchemaAndAnnotations`, `TestBuildAdvertisesAuthorityFieldsOnWorkspaceToolsOnly`, `TestInspectOutputSchemaModelsInspectInfo`, `TestDocsIndexOutputSchemaModelsDocsIndexResult` | 구현 전 `Tool.Annotations`, `Tool.OutputSchema`, `contract.ToolAnnotations`, `WorkspaceScopedTools` 미정의로 컴파일 실패(LSP) | 통과 |
| mcpcli: `TestSDKStructuredContentOverStdioAndStream`(stdio, daemon_conn), `TestSDKToolsListAgreesWithCatalog`, `TestSDKInvalidStructuredOutputIsProtocolInternalErrorWithoutPayload`, `TestSDKStructuredToolErrorResultKeepsTextWithoutStructuredContent`, `TestSDKDirectMarkdownHasNoStructuredContent`, `TestSDKHarnessInspectForwardsHostReceiptsArgument`, `TestWorkspaceAuthorityClassificationMatchesCatalogAuthorityFields`, `TestHTTPToolsListAndCallCarryStructuredOutput`(2025-11-25, 2026-07-28) | 구현 뒤 작성한 테스트라 별도 RED 실행은 없다. 대신 기존 `TestMCPDirectAndSDKKeepProjectAndExecutionServicesIsolated`가 구현 직후 `harness_inspect direct: Invalid tool output`으로 실패해 검증이 실제로 물고 있음을 보였다. `TestSDKToolsListAgreesWithCatalog`는 처음에 SDK가 항상 내는 `idempotentHint:false`를 차이로 잡았다. | 통과 |

`TestSDKToolsListAgreesWithCatalog`는 SDK의 non-pointer hint를 "unset == false"로 비교한다.
카탈로그가 `idempotentHint:false`를 주장하도록 바꾸지는 않았다. 읽기 전용 도구에서는
의미가 없는 hint라서다. 대신 tools/list에는 `idempotentHint:false`가 실제로 나간다.

## 실제 바이너리 실측

`go build -o /tmp/l-out/issueops ./cmd/issueops` 뒤 임시 HOME, CODEX_HOME, `ISSUEOPS_STATE_DIR`로
`/tmp/l-out/verify.py`를 실행했다(증거: `/tmp/l-out/evidence.txt`).

| 표면 | 결과 |
|---|---|
| `issueops mcp` stdio, 2025-11-25 | initialize 협상, tools/list 51개. 두 도구는 annotations `{readOnlyHint:true, openWorldHint:false, idempotentHint:false}`와 2020-12 `outputSchema`를 가진다. 두 도구 호출은 `isError:false`이고 `structuredContent`가 text JSON과 같다. |
| `issueops mcp` stdio, 2026-07-28 | `resultType=complete`. 같은 내용이다. |
| `issueops mcp --http`, 2025-11-25와 2026-07-28 | ready line 확인. bearer는 `<state>/mcp-http/bearer`에서 읽었다. tools/list 51개, 두 도구의 annotations·outputSchema·structuredContent가 stdio와 같다. SIGTERM 뒤 exit 0. |
| 변경 도구 | 네 실행 모두 `state_write`와 나머지 도구에 `annotations`, `outputSchema`가 없다. `commit_policy`는 `structuredContent` 없이 Markdown(`---\nname: CO...`)을 그대로 돌려준다. `gates_init`은 `authority_file`, `workspace_root`, `cwd`를 광고한다. |

실제 환경에서는 `skills`와 `docs`가 비어 있지 않아 null 필드는 없었다. null slice는
SDK round-trip 테스트(`InspectInfo{}`, `DocsIndexResult{}`)가 검증한다.

## golden

`cmd/issueops/testdata/mcp_tools.golden.json`은 `go test ./cmd/issueops/contractgolden -run Golden -update`로 다시 만들었다. 이 golden은 내 변경의 순수 재생성 결과다. 갱신 전 파일이 HEAD와 같다는 것과,
모든 차이가 아래 둘뿐이라는 것을 스크립트로 분류해 확인했다. 순수 재생성이라 직접 갱신했다.

- `harness_inspect`, `docs_index`: `outputSchema`, `annotations` 추가. `harness_inspect`는 `host_receipts` 입력도 추가.
- workspace 25개 도구: 도구가 이미 갖지 않은 `authority_file`, `workspace_root`, `cwd` 입력 추가(예: `command_policy_check`는 `authority_file`만 추가).
- 도구 수와 순서는 51개 그대로. diff는 HEAD 대비 +560/−2.

## Z에게 넘기는 항목

- **inventory**: `TestDDDResponsibilityInventoryMatchesSource`와 `TestDDDContractFunctionsHaveExplicitRoles`가 새 production 파일과 심볼을 요구한다. `internal/contract/mcp/output_schemas.go`의 함수 9개(`ReadOnlyClosedWorldAnnotations`, `InspectOutputSchema`, `DocsIndexOutputSchema`, `schemaString`, `schemaBool`, `schemaInteger`, `schemaArray`, `schemaRef`, `schemaObject`), `catalog/mcp/authority_fields.go`(`WorkspaceScopedTools`, `withAuthorityFields`), 그리고 `mcp_sdk_server.go`의 `compileToolOutputSchema`, `validatedStructuredContent`, `sdkToolAnnotations`와 `mcp_tool_authority.go`에서 사라진 `withAuthorityFields`를 반영한다.
- **golden**: 위 재생성분을 그대로 받아들이면 된다. response-contract golden의 MCP 목록은 이름만 고정하므로 영향이 없었다.
- **문서**: H 보고서의 "withAuthorityFields를 catalog assembly로 옮길지" 결정은 "옮김"으로 닫혔다. 운영 문서에 tools/list의 `outputSchema`, `annotations`, `structuredContent`를 적는 일이 남는다.

## 한계와 주의

- structuredContent 검증은 `harness_inspect`, `docs_index` 두 도구에만 걸려 있다. DTO가 바뀌면 `output_schemas.go`와 reflect 일치 테스트가 같이 깨지므로 같은 변경에서 schema를 고쳐야 한다. `additionalProperties:false`이므로 schema 없이 필드만 추가하면 실제 호출이 -32603을 반환한다.
- host receipt가 `connected`를 verified로 올리는 부분은 K의 몫이고 이번에 건드리지 않았다. 실제 host가 `docs_index`를 호출했다는 주장도 하지 않는다.
