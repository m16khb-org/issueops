# MCP 결과 표현의 실제 경계 확인

조회일: 2026-10-02.

## 직접 읽은 issueops 코드

- `internal/contract/mcp/catalog_types.go:3-8`의 Tool은 name, description,
  inputSchema만 가진다.
- `cmd/issueops/mcpcli/mcp_sdk_server.go:98-130`은 direct 결과의 content 중
  text만 남기고, 일반 payload도 JSON 문자열로 TextContent에 넣는다.
- 같은 파일 `:134-151`은 SDK Tool 등록 시 Name, Description, InputSchema만
  전달한다.
- `go.mod:33`은 `github.com/modelcontextprotocol/go-sdk v1.6.1`을 고정한다.

## 현재 SDK에서 이미 가능한 부분

메인은 `go env GOMODCACHE`가 반환한 `/Users/habin/go/pkg/mod` 아래의
`github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/protocol.go`를 읽었다.

- `:71-90`: CallToolResult에 `StructuredContent any`가 이미 있다.
- `:1325-1339`: Tool에 `OutputSchema any`가 있다.
- `:1349-1383`: ToolAnnotations는 read-only, destructive, idempotent,
  open-world hint를 정의한다.
- OutputSchema의 자동 검증은 typed AddTool 경로와 schema draft 지원 조건을
  따른다. 필드를 전달하는 것만으로 모든 결과가 검증된다고 가정하지 않는다.
- annotations는 hint이며 신뢰할 수 없는 서버의 내용을 권한 판정으로 사용하면
  안 된다는 주석도 확인했다.

## 제안 범위에 미치는 영향

구조화 결과·출력 스키마·행동 hint를 검토하기 위해 반드시 SDK 버전을 올리거나
HTTP로 바꿔야 하는 것은 아니다. 현재 v1.6.1에도 필요한 표현이 있다.
공유 catalog와 SDK 변환 경계에서 손실되는 정보를 먼저 검토할 수 있다.

반대로 MCP 2026-07-28의 cache hint나 새로운 요청·취소 계약까지 사용하려면
SDK·호스트 compatibility를 별도로 확인해야 한다. 두 과제를 한 변경으로
묶어 비용과 리스크를 부풀리지 않는다.

최종 검토에서 공식 릴리스를 다시 확인했다. v1.7.0은 2026-07-28을 지원하며,
현재 최신 정식 v1.8.0(2026-09-14)도 그 revision을 협상한다. 의존성 갱신은
같은 v1 계열의 minor bump다. 새 HTTP binding의 session·취소 계약 migration과
호스트 상호운용성 비용은 버전 번호와 별도로 평가한다.

효과는 machine-readable 결과 보존과 검증 가능성의 개선 가설이다.
TextContent와 structuredContent를 함께 보내면 bytes가 늘 수 있으므로
토큰 절감이나 처리 시간 개선으로 단정하지 않는다.
