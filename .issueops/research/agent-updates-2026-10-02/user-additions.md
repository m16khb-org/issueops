# 추가 조사: Claude.dev와 Streamable MCP

사용자 추가 요청: “Claude.dev도 확인해보고 streamable mcp 도 확인해봐”

## 범위

- `https://claude.dev/` 자체와 관련 기술 글을 조사한다.
  예전 Claude Dev 확장의 새 이름인 Cline과 섞지 않는다.
- 사이트 발행 주체와 글의 원출처를 확인한다.
  재게시 글은 독립 근거로 중복 계산하지 않는다.
- Streamable HTTP MCP의 정식 명세, 호스트 지원, Go SDK 지원을 확인한다.
- issueops의 in-process stdio MCP, native actor 계보, generation fence와
  독립 HTTP 서버의 차이를 비교한다.
- 로컬 stdio를 HTTP로 바꾸면 빨라진다는 전제를 두지 않는다.

## 실행

추가 DAG: `dag_fef1e2f8-dae7-4ba3-9023-94bce6150f86`.

| 작업 | 요청 모델 | 목적 |
|---|---|---|
| claudedev-sources | GPT-6 Luna | 사이트 정체와 날짜·글 목록 확인 |
| claudedev-content | GPT-6.1 Sol | 주요 글의 메커니즘·반례·적용 후보 분석 |
| streamable-spec | GPT-6 Luna | 명세와 호스트 지원 근거 수집 |
| streamable-boundary | GPT-6 Astra | 전송 방식 변경과 실행 주체 계약의 상충 평가 |
| verify-additions | GPT-6.1 Sol | 원문·코드 교차검증 및 집계 |

## 메인의 초기 원문 확인

2026-10-02에 [공식 2025-11-25 transports 명세](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)를 읽었다.

- stdio와 Streamable HTTP를 두 표준 전송 방식으로 명시한다.
- Streamable HTTP는 기존 2024-11-05 HTTP+SSE를 대체한다.
- 하나의 MCP endpoint를 사용하며 POST 응답은 JSON 또는 SSE일 수 있다.
- GET 기반 SSE를 지원하지 않으면 405 응답을 허용한다.
- 연결 단절은 요청 취소가 아니다. 명시적인 cancellation notification과 구분한다.
- `Last-Event-ID` 기반 재전송은 선택 기능이며 다른 stream의 메시지를 섞어
  재전송해서는 안 된다.
- `MCP-Session-Id`를 반환했다면 이후 요청에 포함해야 하고 세션 만료 뒤에는
  404와 재초기화 규칙을 따른다.
- Origin 검증, 로컬 loopback 바인딩, 인증 요구를 별도로 다룬다.

위 사실은 명세에 대한 확인이며 issueops 도입 효과나 실제 호스트 상호운용성
검증 결과가 아니다. 추가 조사에서 현재 정식 버전과 구현을 대조한다.
