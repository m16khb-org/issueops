# MCP 버전 경계 교정

조회일: 2026-10-02.

`shared-01.md`가 제기한 2026-07-28 정식 릴리스 여부를 메인이 아래 원문으로
확인했다.

- <https://blog.modelcontextprotocol.io/posts/2026-07-28/>
- <https://modelcontextprotocol.io/specification/2026-07-28/basic/transports>
- <https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http>

## 교정

`user-additions.md`의 초기 명세 확인과 `claim-graph.md` C08·C09는
**2025-11-25에 한정된 관측**이다. 최신 HTTP 계약으로 일반화하면 틀린다.
특히 C08의 “연결 단절은 취소가 아니다”는 새 HTTP binding에는 반대다.
최신 명세는 해당 요청의 SSE 응답 stream 종료를 취소로 처리하도록 요구한다.

최신 정식 명세에서는 initialize/initialized, transport session ID,
독립 GET stream, Last-Event-ID 재개가 제거됐다. 요청마다 metadata를 보내고
HTTP header/body 일치를 검증한다. 변경 알림은 subscriptions/listen의
응답 stream으로 전달한다.

메인이 실제 issueops 바이너리로 관측한 초기화 응답은 2025-11-25였다.
이는 해당 버전으로 성공한 증거이며, 그 probe만으로 SDK의 최신 버전 지원
여부나 모든 협상 가능한 버전을 판정하지 않는다.

이 내용을 streamable-spec, streamable-boundary, shared-02 담당자에게
전달했다. 최종 판단에서는 protocol revision, transport, Tasks extension,
application의 durable lease를 각각 구분한다.
