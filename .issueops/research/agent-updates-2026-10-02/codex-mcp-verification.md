# Codex MCP 지원 직접 확인

조회일: 2026-10-02.
요청 URL: <https://developers.openai.com/codex/mcp>.
실제 응답 URL: <https://learn.chatgpt.com/docs/extend/mcp?surface=cli>.
HTTP 200, 본문 변환 후 16,598 bytes, 출력 잘림 없음.

## 확인된 문서 계약

- Codex는 STDIO와 Streamable HTTP MCP를 모두 지원한다고 명시한다.
- HTTP 서버는 `url`, bearer token 환경변수, OAuth, HTTP headers 등을 설정한다.
- 기본 server startup timeout은 10초, tool timeout은 60초다.
- optional MCP 서버의 초기 catalog 대기는 별도
  `mcp_optional_startup_grace_ms`이며 기본 1,000 ms다.
  이를 0으로 설정하면 각 서버의 startup timeout을 기다린다고 명시한다.
  required 서버는 자신의 startup timeout을 따른다.
- `enabled_tools`/`disabled_tools`로 도구 노출을 제어할 수 있다.
- initialize 응답의 server `instructions`를 사용한다.
  처음 512자를 독립적으로 이해 가능하게 작성하라고 안내한다.

## 적용성 해석

Streamable HTTP 연결 가능 여부와 issueops 서버를 HTTP로 제공해도 되는지는
다른 질문이다. Codex 클라이언트 지원은 문서로 확인했지만 native actor 계보,
서버 인증, 수명주기, 재전송 안전성은 서버 설계에서 추가로 검증해야 한다.

startup timeout과 optional catalog grace를 혼동하면, 도구가 첫 turn에서
보이지 않는 문제를 단순히 timeout 증대로 해결하려 할 수 있다.
현재 issueops의 설정값과 실제 host 버전을 함께 확인한 실험이 필요하다.
라이브 문서의 모든 설정이 Codex 0.160.0에 들어 있다는 버전 고정 주장은 하지 않는다.
