# Fable 5.1 최종 독립 검토와 반영 결과

검토일: 2026-10-02.
작업: `st_01a0fb21`, completed.
실제 모델: `anthropic-subscription/claude-fable-5-1`.
도구가 보고한 검토 시간: 5분 12초, 도구 호출 15회.
파일 수정 권한 없이 한 번만 검토했다.

## 원래 판정

**수정 후 게재 가능(ready with corrections).**
차단 수준 오류는 없고 중간 2건·낮음 4건을 제시했다.
관측 정확성 우선, native mutation의 stdio 유지, 성능 효과를 가설로 한정하는
핵심 결론은 코드와 원문이 뒷받침한다고 판단했다.

이 문서의 반영 결과는 메인의 확인이다. Fable에 재검토를 요청하거나
수정 후 무조건 승인 판정을 새로 받았다고 주장하지 않는다.

## Findings와 처리

| ID | 심각도 | Fable의 지적 | 메인의 확인·반영 |
|---|---|---|---|
| M1 | 중간 | duration 결함이 한 곳이 아니라 네 probe에 있음 | 네 성공 경로를 직접 읽었다. 결론·상세 설명·issueops-09에 네 파일:라인과 label을 반영 |
| M2 | 중간 | 최신 SDK v1.8.0은 이미 MCP 2026-07-28을 협상함 | 공식 v1.7.0/v1.8.0 및 latest API를 직접 조회. SDK minor 갱신과 application migration 비용을 분리하고 P1 비용·digest-3·출력 근거 문서 수정 |
| L1 | 낮음 | TRACEPARENT 전파는 custom ANTHROPIC_BASE_URL 조건의 영향을 받음 | 원문 전체의 switch 설명을 확인. custom endpoint의 CLAUDE_CODE_PROPAGATE_TRACEPARENT=1 조건을 본문·관측 근거에 반영 |
| L2 | 낮음 | HTTP session ID는 이전 revision의 identity 후보임 | 2025-11-25에 한정해 표현. 최신 request metadata와 구분 |
| L3 | 낮음 | NormalizeNativeActor 인용 끝 줄은 43임 | 실제 함수 범위를 읽어 메인 참조를 23-43으로 수정. live 측정값은 그대로 유지 |
| L4 | 낮음 | upstream 수량 drift는 TECH_STACK.md:45에도 있음 | 파일을 직접 읽어 문서 불일치 목록에 추가. 운영 문서는 수정하지 않음 |

L3의 선택적 제안으로 golden 직렬화 크기와 live 크기의 2-byte 차이를 각주로
남길 수 있다는 의견도 있었다. 본 보고서는 서로 다른 직렬화 표면의 숫자를
새 근거로 추가하지 않고 실제 stdio 관측 30,888 bytes를 유지했다.
이 차이를 오류나 동등성 증거로 해석하지 않는다.

## M1의 네 위치

다음 파일은 모두
`internal/adapter/verification/probe/contractauditworker/` 아래에 있다.

- `validation_contract_check.go:47`.
- `validation_tool_conformance.go:47`.
- `validation_worker_lifecycle.go:46`.
- `validation_command_audit.go:42`.

메인은 각 성공 경로가 앞서 얻은 command result의 시간을 버리고
`time.Since(time.Now()).Milliseconds()`로 assertion 시간을 만드는지 직접 읽었다.
정확한 영향량이나 원래 명령이 느리다는 사실까지 측정한 것은 아니다.

## M2의 직접 확인

API 응답은 모두 HTTP 200이었다.

- [v1.7.0](https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/tags/v1.7.0):
  `published_at: 2026-07-28T13:09:53Z`, `prerelease:false`.
  원문: “This release brings full support for protocol version 2026-07-28.”
- [v1.8.0](https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/tags/v1.8.0):
  `published_at: 2026-09-14T08:05:46Z`, `prerelease:false`.
  원문은 “새 revision 추가 없음” 뒤에 2026-07-28이 여전히 최신 협상
  revision이라고 명시한다.
- [latest](https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/latest):
  같은 v1.8.0을 반환했다.

v1.7.0 release는 새 HTTP revision을 받을 때 `Stateless=true`가 필요하고
stateful handler는 2025-11-25로 협상한다고 명시한다.
현재 pin v1.6.1에서 `Stateless`만 켜면 새 revision이 된다는 뜻은 아니다.
구조화 결과·annotation은 이미 v1.6.1에 있어 별도 과제로 남겼다.

## 검토 범위와 한계

Fable은 메인 보고서 전체, 네 digest, 재현·검증·출처 근거, 결정적 로컬 코드와
공식 릴리스·MCP·Claude.dev·monitoring 원문을 읽었다고 보고했다.
60개 조사 노트의 모든 URL을 전수 재조회하거나 host 성능 비교·self-verify를
다시 실행하지 않았다. 메인은 채택한 지적을 직접 확인했다.

Fable도 JSONL 누락과 self-verify 실패 기록의 의미, 단일 공급자 출처 예외,
설치본·현재 세션·공개 릴리스의 구분을 유지했다.
그 판정을 실제 HTTP 도입 성공이나 성능 개선의 실험 증거로 사용하지 않는다.
