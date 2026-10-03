# MCP 요청 경로 조사

## 확인한 근거

- `cmd/issueops/mcpcli/mcp_http.go`의 `NewHTTPHandler`는 SDK server를 한 번
  생성하고 HTTP 요청마다 같은 인스턴스를 반환한다. 서버를 매 요청 재생성한다는
  가설은 기각한다.
- `cmd/issueops/mcpcli/mcp_sdk_server.go`의 `sdkToolHandlerWithContext`는 output
  schema를 handler 생성 시 한 번 컴파일하지만 input은 요청마다
  `validateMCPToolArguments`에 전체 catalog를 넘긴다.
- `cmd/issueops/mcpcli/mcp_tools.go`의 `validateMCPToolArguments`는 매 요청
  도구 목록을 선형 탐색하고 `ClosedProjection(schema)`로 schema 전체를 복제한다.
- `internal/domain/toolconformance/schema.go`의 `ClosedProjection`은 중첩 map과
  slice를 새로 생성한다. `Validate`는 지원 schema 검사와 입력 검사를 수행한다.
- `cmd/issueops/mcpcli/catalog_isolation_test.go`는 서로 다른 schema를 가진
  서버의 격리, invalid input의 effect 이전 거부, 공급된 catalog 광고를 검사한다.

## 제안: handler별 input schema 준비

서버 생성 시 각 handler가 자기 input schema의 closed projection을 준비하도록
옮긴다. 요청별 입력 검증과 오류 코드는 유지한다. package-global cache는 만들지
않는다. 도구 목록 전체를 요청마다 순회하거나 동일 schema를 복제하지 않는다.

수정 범위:

- `cmd/issueops/mcpcli/mcp_tools.go`
- `cmd/issueops/mcpcli/mcp_sdk_server.go`
- 같은 패키지의 focused regression 및 benchmark 파일

위험: schema 준비 오류의 반환 시점, unknown tool 및 invalid schema 오류 코드,
서버별 catalog 격리를 보존해야 한다. 공유한 schema는 검증 중 수정하면 안 된다.

## 측정과 수용 기준

실행 시간과 allocation 개선 수치는 아직 측정하지 않았다. 구현 전 동일한
handler 호출 benchmark를 추가해 기준선을 확보하고 구현 후 같은 명령으로 비교한다.

```bash
go test ./cmd/issueops/mcpcli -run '^$' -bench BenchmarkSDKToolHandler -benchmem -count=5
go test -race ./cmd/issueops/mcpcli -count=1
```

- invalid input이 effect를 호출하지 않고 기존 protocol error를 반환한다.
- nested object의 unknown property 거부와 서로 다른 catalog의 격리가 유지된다.
- stdio 및 실제 loopback HTTP 요청으로 정상·오류 응답을 검증한다.
- 정상 호출의 allocation 감소를 확인하며 전역 상태나 권한 우회를 추가하지 않는다.

## 제외

HTTP transport 교체, pprof endpoint 추가, actor 권한 cache, 상태 잠금 범위 변경은
이 개선의 필요조건이 아니다. 요청별 schema 복제보다 먼저 구현할 근거가 없다.

## 조사 이력

최초 MCP 조사 노드가 사용자 지정 밖 모델로 실행된 사실을 확인해 중단했다.
이 보고서는 메인 에이전트가 위 소스를 직접 읽어 작성했으며, 다음 검증 노드가
현재 소스와 대조한 뒤에만 구현 후보로 채택한다.
