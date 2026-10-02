# I5 SDK 및 stdio 리비전 호환성 구현 증거

상태: F에 할당된 구현과 검증을 완료했다. HTTP 및 실제 세 호스트 검증을
완료했다고 주장하지 않는다. 기준 계약은 같은 폴더의 `contract.md`다.

## 변경 파일과 실행 환경

- `go.mod`: MCP Go SDK를 v1.6.1에서 **v1.8.0**으로 올렸다.
  SDK가 요구하는 `golang.org/x/time v0.15.0`을 indirect로 추가했다.
- `go.sum`: 이전 SDK checksum 두 줄을 새 버전 checksum으로 교체하고
  x/time checksum 두 줄을 추가했다. 다른 dependency 버전은 바꾸지 않았다.
- `cmd/issueops/mcpcli/mcp_revision_compatibility_test.go`: production
  `ServeMCPStreamContextWithDependencies`를 split stdio pipe로 호출하는
  결정적 wire 회귀 테스트를 추가했다.
- 이 보고서만 추가했다. `mcp_sdk_server.go`의 production 구현은 읽었지만
  수정하지 않았다. SDK API break 때문에 범위를 넓힐 필요는 없었다.

실제 export는 `PI_MODEL=gpt-6.1-sol`, `PI_REASONING_LEVEL=high`다.
환경을 변경하지 않았다. 이는 현재 세션의 export 관측이며 provider 요청
내부 설정을 별도로 관측했다는 뜻은 아니다. 원장의 이전 medium 기록과
현재 세션의 high 관측을 구별한다. toolchain은 `go1.26.4 darwin/arm64`다.

`go mod download -json github.com/modelcontextprotocol/go-sdk@v1.8.0`은 exit 0이었다.
origin은 공식 저장소의 `refs/tags/v1.8.0`,
commit `3f3b699b2b67e1ed033a63d6651671dab53c2d32`였다.
모듈 checksum은 `h1:KIvahhYqwtbeniWVPs3TcXEA7b8jEtwfBpOTAI+Urx4=`,
go.mod checksum은 `h1:dL7u98E/zjJTGzEq+j30jQ8K2k1mb6LeAH4inEcSGts=`다.
캐시된 공식 원본의 `mcp/shared.go`, `server.go`, `client.go`, `protocol.go`,
`go.mod`를 직접 읽었다.

## stdio matrix

| 입력 또는 동작 | 검증한 결과 |
|---|---|
| initialize: 2025-06-18 / 2025-11-25 | 요청한 legacy revision을 그대로 협상하고 tools/list가 성공한다. |
| initialize: 2026-07-28 / 2099-01-01 | 2025-11-25로 fallback하며 catalog를 조회할 수 있다. initialize로 2026을 협상하지 않는다. |
| 2026 server/discover | initialize 없이 성공하며 2025-11-25와 2026-07-28 지원을 광고한다. |
| 2026 tools/list / tools/call | 요청별 metadata만으로 성공한다. catalog의 docs_index와 실제 contract_schema 결과를 검사한다. |
| 2026 성공 응답 | `resultType=complete`와 서버 identity metadata가 보존된다. |
| clientInfo 생략 | SDK 계약대로 허용한다. clientCapabilities는 제공한다. |
| 요청별 2099-01-01 | 오류 -32022와 requested/supported 데이터가 반환된다. |
| clientCapabilities 누락 / clientInfo 잘못된 타입 | 오류 -32602로 거부한다. |
| legacy server/discover / 2026 ping | 오류 -32601로 거부한다. |
| 2025 및 2026 execution tools/call 취소 | handler 진입 채널을 관측한 뒤 정확한 requestId를 취소하면 production dispatch의 context가 취소된다. |
| 2026 subscriptions/listen | toolsListChanged 승인 notification을 먼저 관측하고 명시적 stdio 취소 후 listen 완료를 확인한다. |

테스트는 handshake, metadata, execution cancellation, subscription cancellation의
네 그룹이다. sleep이나 polling은 없다. 응답과 채널 대기는 5초로 제한하고,
test context timeout이 cancellation 성공으로 오인되지 않도록 검사한다.
pipe와 server goroutine은 cleanup에서 닫고 종료를 기다린다.

새 테스트 파일은 247 pure LOC다. 책임은 stdio 리비전 회귀 검증 하나이며,
wire JSON은 경계에서 파싱한다. SDK 서버와 기존 catalog/service fixture를
사용하고 production dispatcher를 복제하지 않았다. 기존 테스트는 수정하지 않았다.
추가 확장이 필요하면 이 파일에 계속 누적하지 않고 소유권을 먼저 조정해야 한다.

## RED와 GREEN

모든 Go 테스트는 기존 격리 Python 환경의 bin을 PATH 맨 앞에 두었다.
아래 `$PYTHON_VENV`는 원장의 `/tmp/issueops-ten-improvements-01a0fa31/venv`다.
`$SDK_FIXTURE`는 시스템 임시 디렉터리의 `issueops-sdk-compat-ii3D5U`다.
full RED/GREEN 로그는 각각 `red.log`, `green-matrix.log`, `final-green.log`로
그 fixture에 보존했다. monitor는 command 종료 상태를 확인하는 데 사용했다.

| 단계 | 명령 | exit 및 관측 |
|---|---|---|
| 최초 RED, SDK v1.6.1 | `PATH="$PYTHON_VENV/bin:$PATH" go test ./cmd/issueops/mcpcli -run TestMCPRevision -count=1 -v` | 1, 2026 metadata 및 execution 취소 실패 |
| 최종 matrix RED, 원래 SDK pin 재현 | 같은 명령, subscription 테스트도 포함 | 1, monitor `mon_RP8TW5KRVY5KQSC7`; 전문은 아래에 보존 |
| dependency 계산 | `go mod tidy -diff` | 첫 실행 1은 적용할 x/time delta가 있다는 의미였다. resolver diff를 apply_patch로 적용했다. |
| matrix GREEN | `go mod tidy -diff && PATH="$PYTHON_VENV/bin:$PATH" go test ./cmd/issueops/mcpcli -run TestMCPRevision -count=1 -v` | 0, monitor `mon_JKGTHKX2FX62GEHQ`; 4개 그룹 PASS, 0.692s |
| 최종 필수 검증 | `go mod verify && PATH="$PYTHON_VENV/bin:$PATH" go test ./cmd/issueops/mcpcli -count=1` | 0, `all modules verified`; package 4.940s |
| 최종 focused race | `PATH="$PYTHON_VENV/bin:$PATH" go test -race -shuffle=on ./cmd/issueops/mcpcli -run TestMCPRevision -count=1 -v` | 0, 4개 그룹 PASS, 1.948s |
| build | `go build -o "$SDK_FIXTURE/issueops" ./cmd/issueops` | 0, monitor `mon_KJ67EY7A89JXS5HF` |
| format 및 공백 | `gofmt -l cmd/issueops/mcpcli/mcp_revision_compatibility_test.go`; `git diff --check -- go.mod go.sum cmd/issueops/mcpcli/mcp_revision_compatibility_test.go` | 0, 출력 없음 |
| LSP | 새 테스트 파일, severity=all | `No diagnostics found` |

최종 필수 검증과 race는 같은 command chain에서 모두 성공했다
(`mon_8BWXB6WW62FZG1RF`). timeout 오인 방지 assertion을 추가한 뒤의 결과다.
중간 GREEN 시도에서는 새 테스트가 SDK 응답 필드를 `type`으로 잘못 읽어
실패했다. 공식 wire 필드 `resultType`으로 바로잡고 다시 RED/GREEN을 확인했다.
production 오류를 덮거나 기존 테스트의 기대값을 약화한 변경은 아니다.

최종 matrix RED 전문:

```text
=== RUN   TestMCPRevisionStdioLegacyNegotiation
=== RUN   TestMCPRevisionStdioLegacyNegotiation/2025-06-18
=== RUN   TestMCPRevisionStdioLegacyNegotiation/2025-11-25
=== RUN   TestMCPRevisionStdioLegacyNegotiation/2026-07-28
=== RUN   TestMCPRevisionStdioLegacyNegotiation/2099-01-01
--- PASS: TestMCPRevisionStdioLegacyNegotiation (0.02s)
    --- PASS: TestMCPRevisionStdioLegacyNegotiation/2025-06-18 (0.01s)
    --- PASS: TestMCPRevisionStdioLegacyNegotiation/2025-11-25 (0.00s)
    --- PASS: TestMCPRevisionStdioLegacyNegotiation/2026-07-28 (0.00s)
    --- PASS: TestMCPRevisionStdioLegacyNegotiation/2099-01-01 (0.00s)
=== RUN   TestMCPRevisionStdioRequestMetadata
=== RUN   TestMCPRevisionStdioRequestMetadata/discover
    mcp_revision_compatibility_test.go:167: request error: method "server/discover" is invalid during session initialization
=== RUN   TestMCPRevisionStdioRequestMetadata/list_without_initialize
    mcp_revision_compatibility_test.go:167: request error: method "tools/list" is invalid during session initialization
=== RUN   TestMCPRevisionStdioRequestMetadata/call_without_initialize
    mcp_revision_compatibility_test.go:167: request error: method "tools/call" is invalid during session initialization
=== RUN   TestMCPRevisionStdioRequestMetadata/optional_client_info
    mcp_revision_compatibility_test.go:167: request error: method "tools/list" is invalid during session initialization
=== RUN   TestMCPRevisionStdioRequestMetadata/unsupported_revision
    mcp_revision_compatibility_test.go:150: error = method "tools/list" is invalid during session initialization, want code -32022
=== RUN   TestMCPRevisionStdioRequestMetadata/missing_capabilities
    mcp_revision_compatibility_test.go:150: error = method "tools/list" is invalid during session initialization, want code -32602
=== RUN   TestMCPRevisionStdioRequestMetadata/invalid_client_info
    mcp_revision_compatibility_test.go:150: error = method "tools/list" is invalid during session initialization, want code -32602
=== RUN   TestMCPRevisionStdioRequestMetadata/legacy_discover
    mcp_revision_compatibility_test.go:150: error = method "server/discover" is invalid during session initialization, want code -32601
=== RUN   TestMCPRevisionStdioRequestMetadata/removed_ping
    mcp_revision_compatibility_test.go:150: error = <nil>, want code -32601
--- FAIL: TestMCPRevisionStdioRequestMetadata (0.02s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/discover (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/list_without_initialize (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/call_without_initialize (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/optional_client_info (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/unsupported_revision (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/missing_capabilities (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/invalid_client_info (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/legacy_discover (0.00s)
    --- FAIL: TestMCPRevisionStdioRequestMetadata/removed_ping (0.00s)
=== RUN   TestMCPRevisionStdioCancellation
=== RUN   TestMCPRevisionStdioCancellation/2025-11-25
=== RUN   TestMCPRevisionStdioCancellation/2026-07-28
    mcp_revision_compatibility_test.go:234: execution handler did not start
    mcp_revision_compatibility_test.go:61: stdio shutdown: context deadline exceeded
--- FAIL: TestMCPRevisionStdioCancellation (5.09s)
    --- PASS: TestMCPRevisionStdioCancellation/2025-11-25 (0.09s)
    --- FAIL: TestMCPRevisionStdioCancellation/2026-07-28 (5.00s)
=== RUN   TestMCPRevisionStdioSubscriptionCancellation
    mcp_revision_compatibility_test.go:260: subscription acknowledgement = {ID:7 Method: Params:[] Result:[] Error:method "subscriptions/listen" is invalid during session initialization}, parse error = unexpected end of JSON input
--- FAIL: TestMCPRevisionStdioSubscriptionCancellation (0.00s)
FAIL
FAIL	issueops/cmd/issueops/mcpcli	5.522s
FAIL
```

## 실제 진입점 사용

방금 빌드한 `$SDK_FIXTURE/issueops mcp`를 fixture 디렉터리에서 실행했다.
`ISSUEOPS_ROOT`는 저장소 root, `ISSUEOPS_STATE_DIR`는 fixture의 `state`로
명시했다. stdio stdin에 newline-delimited JSON-RPC를 쓰고 EOF를 보냈다.
stdout/stderr와 process exit를 동시에 수집하고 응답 ID로 결과를 매칭했다.
mock SDK client나 production 대체 dispatcher가 아니라 실제 CLI binary 호출이다.

legacy는 initialize → notifications/initialized → tools/list → docs_index를,
2026은 server/discover → tools/list → docs_index를 보냈다.
2026의 각 요청에는 protocolVersion/clientInfo/clientCapabilities metadata를
넣었다. docs_index의 text JSON을 파싱해 `ok=true`와
`.issueops/CONSTITUTION.md` 항목 존재를 확인했다.

| 실제 입력 | 관측 출력 | process exit |
|---|---|---|
| legacy 2025-11-25 | negotiated 2025-11-25, tools 51개, docs_index ok=true, docs 557개 | 0 |
| sessionless 2026-07-28 | 지원 목록에 2026/2025 포함, tools 51개, docs_index ok=true, docs 557개, resultType=complete | 0 |
| legacy 2099-01-01 | negotiated 2025-11-25, tools 51개, docs_index ok=true, docs 557개 | 0 |
| sessionless 2099-01-01 | JSON-RPC -32022, requested=2099-01-01, supported 목록 포함 | 0 |

지원 목록은 `2026-07-28`, `2025-11-25`, `2025-06-18`, `2025-03-26`,
`2024-11-05`였다. tools/docs 수는 이 실행 시점의 관측값이지 고정 수용 기준이
아니다. 별도 2026 docs_index 호출로 stderr도 확인했다. server start/connect/
disconnect/end의 INFO 진단이었고 응답 ID=3, process exit=0이었다.

## 남은 통합 범위

- H: HTTP의 2025/2026, header/body mismatch, JSON/SSE, POST subscription,
  disconnect cancellation 및 authority/request-local dispatch 검증을 소유한다.
  이번 stdio 명시적 cancellation 결과를 HTTP disconnect 증거로 사용하면 안 된다.
- L: structuredContent/outputSchema/annotations와 SDK 등록 변경을 소유한다.
  SDK bump가 이미 있던 구조화 기능을 새로 구현했다고 주장하지 않는다.
- Z 및 후속 host owner: 전체 go vet/test/race, golden, API 문서 게이트,
  실제 Codex/Claude/Omo stdio·직접 HTTP 검증과 설치·서비스 검증이 남아 있다.
  이번 작업에서는 사용자 HOME 설치나 config 변경을 하지 않았다.
- `.issueops/TECH_STACK.md:99,104`의 현행 SDK pin 설명은 아직 v1.6.1이다.
  문서 owner가 통합 문서 갱신에 반영해야 한다. 과거 조사·ADR의 역사적 기록과 구별한다.
- SDK v1.8.0의 legacy 미래 revision fallback에서 initialize 응답은
  2025-11-25지만 후속 docs_index에 `resultType=complete`가 관측됐다.
  SDK `mcp/shared.go:674-684`가 요청 metadata가 없으면 원래 InitializeParams의
  revision을 반환하는 동작과 일치한다. catalog와 docs 호출은 성공했다.
  엄격한 host parser의 추가 필드 수용 여부는 Z의 live fallback 검증에 포함해야 한다.

최종 source diff는 할당한 모듈 두 파일과 새 테스트뿐이다.
`git diff --numstat -- cmd/issueops/mcpcli/mcp_sdk_server.go`는 출력이 없었다.
commit/push, 환경 설정 변경, 다른 작업자의 파일 수정은 하지 않았다.
