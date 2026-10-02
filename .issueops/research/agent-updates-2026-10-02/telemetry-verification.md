# Claude 관측 기능 원문 검증

조회일: 2026-10-02.
원문: <https://code.claude.com/docs/en/monitoring-usage>

## 수집 경로

webfetch는 HTTP 200을 반환했지만 136.3KB 중 49.6KB만 표시했다.
이는 전체 문서가 아니므로 하단의 지표가 없다고 판정하지 않았다.
같은 공개 URL에 `Accept: text/markdown`으로 직접 요청해 HTTP 200과
139,517자의 전체 본문을 받은 뒤 해당 절을 확인했다.
로그인·차단 우회나 비공개 데이터 접근은 없었다.

## 직접 확인한 사실

- 추적이 켜지면 Bash·PowerShell 자식 프로세스에 W3C `TRACEPARENT`가 전달된다.
  자식 CLI가 같은 trace 아래에 자신의 span을 연결할 수 있다.
  전파는 별도 switch의 영향을 받는다. 기본값은 `ANTHROPIC_BASE_URL`이 없거나
  Anthropic API를 가리킬 때이며, custom endpoint에서는
  `CLAUDE_CODE_PROPAGATE_TRACEPARENT=1`로 허용해야 한다. `0`이면 비활성화한다.
- `claude_code.cost.usage`, `claude_code.token.usage`,
  `claude_code.active_time.total`이 공식 지표 목록에 있다.
- 비용 지표는 근삿값이다. 공식 청구 데이터는 API 공급자를 확인하라고 안내한다.
- v2.1.214 이전에는 여러 frame으로 usage를 보내는 gateway/proxy 응답에서
  비용·토큰을 중복 계산할 수 있었다고 문서화되어 있다.
  이는 과거 버전의 알려진 동작이지 현재 재현한 결함이 아니다.
- `claude_code.hook` span은 상세 beta tracing과 endpoint가 필요하다.
  대화형 CLI에서는 조직 allowlist도 필요하며 Agent SDK·비대화형 `-p`는
  해당 allowlist를 요구하지 않는다고 명시한다.
- raw API body logging은 전체 대화 이력을 포함할 수 있다.
- `query_source_safe`는 v2.1.268 이후의 bounded attribution 필드다.
  계측 설계에서는 raw 이름과 고카디널리티 식별자를 무조건 metric label로
  추가하지 않아야 한다.

## issueops에 대한 후속 확인

`TRACEPARENT` 전달은 hook에 계측 코드를 넣지 않고 CLI 호출과 호스트 trace를
연결할 가능성을 제공한다. 현재 issueops가 이 값을 소비하는지, 자체 trace ID와
어떤 관계인지 로컬 관측 담당자에게 확인을 요청했다.

현재 단계에서는 새 collector·daemon·SDK 도입을 권고하지 않는다.
먼저 기존 trace와의 상관관계를 보완할 수 있는지 확인하며, 토큰 절감·실행 시간
개선이나 beta 기능의 실제 사용 가능 여부는 아직 측정하지 않았다.
