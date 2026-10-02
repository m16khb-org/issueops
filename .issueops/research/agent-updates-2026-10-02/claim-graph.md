# 주장 및 반증 기록

조회일은 모두 2026-10-02다. `supported`는 적힌 범위의 주장만 지지하며
issueops에서 효과를 측정했다는 뜻이 아니다.

| ID | 주장 | 근거 | 상태 | 독립성·반증 |
|---|---|---|---|---|
| C01 | Claude Code v2.1.287은 2026-10-01T18:00:22Z에 공개된 정식 릴리스다 | [공식 API](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287)의 tag_name, published_at, prerelease:false | supported | 메인이 API 원문을 읽었다. 단일 공식 릴리스 출처 예외다. |
| C02 | v2.1.287은 OTel user_prompt에 prompt_text를 추가하며 prompt와 함께 마스킹하라고 안내한다 | 위 API body | supported | 원문에 같은 요구가 있다. issueops에 prompt 수집을 추가하라는 뜻은 아니다. |
| C03 | v2.1.287은 큰 MCP 결과의 메모리·세션 파일 사용량을 줄였다고 발표했다 | 위 API body의 Improved handling of large MCP tool results | supported | 공급자 발표만 확인됐다. 개선량이나 issueops 효과는 측정하지 않았다. |
| C04 | v2.1.287은 MCP alwaysLoad:false를 해당 서버의 모든 도구 지연 로딩으로 바꿨다 | 위 API body 마지막 Changed 항목군 | supported | 호스트 기능이다. core에 검색기를 새로 넣어야 한다는 결론은 아직 없다. |
| C05 | v2.1.286은 whole-model-call 단위 재시도 제한과 --bare 동작을 바꿨다 | [공식 API](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.286)의 body | supported | published_at 2026-09-30T19:10:13Z, prerelease:false를 메인이 확인했다. |
| C06 | v2.1.286 릴리스 본문이 많은 deny rule·MCP 도구 조건의 per-turn 성능 개선을 명시한다 | claude-01.md의 두 번째 주장 | unresolved | 메인이 읽은 해당 API body에는 이 문장이 없다. 다른 릴리스와 섞였을 수 있으므로 최종 보고서에서는 제외한다. |
| C07 | issueops MCP 서버는 전체 공개 도구가 하나뿐이다 | claude-03.md 초기 finding 5 | refuted | `internal/adapter/inbound/catalog/mcp/catalog.go:18-44`는 여러 영역의 catalog를 합친다. `IssueOpsBasicTools` 하나를 전체 서버로 일반화한 오류다. 담당자에게 교정을 전달했다. |
| C08 | 2025-11-25 Streamable HTTP는 연결 단절을 요청 취소로 취급하지 않는다 | [이전 transport 명세](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#sending-messages-to-the-server) | supported, historical scope | 2026-07-28에는 적용할 수 없다. 새 HTTP binding은 SSE 응답 stream 종료를 취소로 처리한다. |
| C09 | Streamable HTTP는 기존 HTTP+SSE를 대체하며 stdio도 표준으로 남는다 | [정식 transport 명세](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) | supported | 명세의 두 표준 transport와 대체 대상 문구를 직접 확인했다. stdio 폐기 근거가 아니다. |
| C10 | 최신 정식 MCP 2026-07-28은 handshake·transport session을 제거하고 HTTP 취소·알림 계약을 바꿨다 | [발표](https://blog.modelcontextprotocol.io/posts/2026-07-28/), [binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) | supported | 메인이 두 원문을 읽었다. 기존 probe의 2025-11-25 성공과 최신 지원 여부는 별개다. |

## 후속 질문

- C03·C04를 현재 issueops descriptor/result 크기와 연결할 때 이미 있는 기능을
  누락으로 오인하지 않는지 로컬 조사와 대조한다.
- C02는 prompt 내용 수집보다 host-neutral 사용량·지연 필드와 redaction 범위를
  구분해야 한다.
- C06은 독립 검증 집계에서 출처 버전을 다시 확인한다.
- C07의 교정에 따라 MCP 도구 검색의 적용성은 전체 advertised catalog의
  실제 크기와 호스트의 로딩 동작을 기준으로 다시 평가한다.
