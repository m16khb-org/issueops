# 최신 에이전트 업데이트와 issueops 개선안

STATUS: research complete — Fable 5.1 지적 반영, self-verify 환경 실패 별도 기록

## 결론

현재까지 확인한 근거로는 **관측 데이터의 정확성을 먼저 고치고, 기존 실행
경계에서 반복 작업을 줄이는 순서**가 적절하다. MCP를 HTTP로 바꾸거나 새
오케스트레이터를 만드는 일보다 적용 범위가 작고 검증하기 쉽다.

가장 명확한 발견은 `trace analyze`의 큰 JSONL 행 뒤 이벤트 누락이다.
실제로 실패 이벤트가 사라져도 `ok:true`가 반환됐다. 같은 패키지의 네 probe는
실행이 끝난 뒤 `time.Since(time.Now())`를 계산해 원래 명령의 시간을 보존하지
않는다. contract check, tool contract conformance, worker lifecycle smoke,
command audit smoke가 해당한다. 이런 데이터로 최적화 우선순위를 정하면 판단이
잘못될 수 있다.
([재현](agent-updates-2026-10-02/trace-jsonl-reproduction.md),
`internal/adapter/verification/probe/contractauditworker/validation_contract_check.go:47`)

다음 후보는 기존 SDK에서 가능한 MCP 구조화 결과·행동 hint, 세 호스트를
구분하는 설치·발견 상태, request-local 중복 관측 축소다. 아래 우선순위는
Astra의 구조 검토와 Fable 5.1 독립 검토 지적을 반영한 제안이다.
구현 전후의 성능 효과는 아직 측정하지 않았다.

## 제품별 업데이트

### Claude Code: 도구 로딩과 실행 가시성

공식 API에서 확인한 v2.1.287은 2026-10-01 공개된 정식 릴리스다.
이 버전은 MCP 서버의 `alwaysLoad:false`를 “해당 서버의 모든 도구를 검색 뒤에
두는 설정”으로 바꿨다. 큰 MCP 결과를 처리할 때 메모리와 세션 파일 사용량을
줄였다는 개선도 포함한다. 단, 릴리스는 개선 비율을 제시하지 않는다.
이를 issueops의 예상 절감률로 사용할 수는 없다.
([공식 릴리스](https://github.com/anthropics/claude-code/releases/tag/v2.1.287))

같은 릴리스는 agents 화면에 이름·작업 검색을 추가하고, 승인 대기 중인 작업을
잘못 표시하거나 hook 내부 콜백까지 실행 개수에 포함하던 문제를 수정했다.
도구 실행의 시작·종료·승인 대기를 구별하고 관측값의 의미를 정확히 정의하는
방향은 참고할 만하다. 다만 호스트 화면을 issueops의 durable 실행 상태와
동일시해서는 안 된다.
([v2.1.286](https://github.com/anthropics/claude-code/releases/tag/v2.1.286),
[v2.1.287](https://github.com/anthropics/claude-code/releases/tag/v2.1.287))

OTel `user_prompt`에 추가된 `prompt_text`는 기존 `prompt`의 복사본이다.
공식 안내도 두 필드를 함께 제거하거나 마스킹하라고 한다. 관측 기능을
확장하더라도 지연·사용량·오류 분류와 프롬프트 원문 수집은 별도 결정이어야 한다.
([공식 릴리스](https://github.com/anthropics/claude-code/releases/tag/v2.1.287))

### Codex: 반복 작업과 SQLite 경합을 줄이는 개선

공식 GitHub `releases/latest`가 반환한 정식 버전은 `rust-v0.160.0`이며
공개 시각은 2026-10-01T20:19:13Z다. 더 높은 alpha 번호가 보이더라도
정식 최신 버전과 구별한다.
([공식 릴리스](https://github.com/openai/codex/releases/tag/rust-v0.160.0),
[조회 증거](agent-updates-2026-10-02/codex-release-verification.md))

이 릴리스에서 issueops와 비교할 가치가 큰 변경은 다음과 같다.

- SQLite 연결 초기화와 logging으로 발생하던 stall을 줄이고, 초기화 오류를
  timeout으로 덮지 않도록 수정했다.
- plugin manifest 파싱 결과를 재사용하고 remote plugin 요청의 HTTP connection
  pool을 재사용했다.
- 오래된 작업을 더 읽는 history pagination과 running turn의 incremental 추적을
  추가했다.
- 재연결 뒤에는 결과가 불확실한 전송을 먼저 해소한 후 미전송 메시지를 재개한다.
- 명시적인 provider model catalog를 권위로 삼아 미지원 모델이나 stale 목록을
  잘못 사용하는 경우를 줄였다.

공통점은 새 실행 엔진을 추가하는 대신 반복 I/O, 불필요한 갱신, 잠금 구간,
모호한 복구 상태를 좁혀 개선한다는 데 있다. 다만 Codex의 SQLx 구현에서 발생한
문제를 issueops의 Go SQLite 구현에도 있다고 단정해서는 안 된다. 로컬 호출
경로와 측정값을 대조한 뒤 적용 후보를 정한다.
([릴리스와 관련 PR 목록](https://github.com/openai/codex/releases/tag/rust-v0.160.0))

### Omo: 최신 공개 버전과 현재 설치본을 구분해야 한다

조사 시작 때 설치본은 `omo-ai 5.1.8` / Senpi `2026.10.1-2`였으며, 공식 공개 버전
`5.1.9`는 2026-10-02T01:13:10Z에 나왔다. 5.1.9에는 in-process 자식 작업의
timeout·retry 설정 상속, reload 뒤 tool search 복구, 모델이 지원하지 않는
thinking level 때문에 생기던 경고·묵시적 보정 문제가 포함된다.
이번 조사에서는 업데이트를 실행하지 않았다.
([5.1.9 릴리스](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.9),
[설치본 조사](agent-updates-2026-10-02/omo-01.md))

후속 점검에서는 배포 패키지가 5.1.9로 바뀌었지만, 현재 세션의 고정 runtime은
여전히 2026.10.1-2였다. 업데이트 원인은 이 조사에서 추적하지 않았다.
아래 설치 코드 분석은 조사 당시 runtime을 기준으로 하며 5.1.9의 수정 효과를
현재 세션에서 재현했다고 주장하지 않는다.
([검증 기록](agent-updates-2026-10-02/verification-ledger.md))

issueops가 참고할 부분은 다음 세 가지다.

- **요청 모델과 실제 모델의 분리:** category, profile, provider 가용성,
  fallback에 따라 실행 모델이 달라질 수 있다. category와 model을 함께
  지정하면 거부되는 계약도 있다. 실행 영수증에는 요청·선택·fallback을
  구별해야 한다. 이번 조사는 named-agent override로 Luna·Sol을 직접 지정하고
  실제 실행을 확인했다.
- **대기와 실행 시간의 분리:** concurrency lane, global budget, residency는
  서로 다르다. eval의 `queued_ms`, kernel duration, tool-call duration도
  같은 시간이 아니다. 병렬 tool duration을 모두 더해 wall time으로 표시하면
  중복 계산이 된다.
- **복구 범위의 명시:** DAG retry는 실패 노드와 관련 후속 작업만 재개한다.
  monitor 재시작은 이전 출력의 완전한 replay가 아니며, 취소 상태 기록과
  프로세스 정리 완료도 다르다.

Omo에 이미 있는 스케줄러·캐시·persistent eval을 issueops core에 다시 만들
필요는 없다. 호스트의 기존 관측값을 읽기 전용으로 연결하는 후보부터 검토한다.
([라우팅](agent-updates-2026-10-02/omo-03.md),
[동시성](agent-updates-2026-10-02/omo-04.md),
[monitor](agent-updates-2026-10-02/omo-05.md),
[eval](agent-updates-2026-10-02/omo-09.md),
[취소](agent-updates-2026-10-02/omo-10.md))

### Claude.dev: 하네스·스킬·캐시를 어떻게 설계하는가

Claude.dev는 Anthropic이 운영하는 기술 글 사이트다. 이용약관에 운영 주체가
명시되어 있으며, 예전 Claude Dev라는 이름을 쓰던 Cline 확장과는 다르다.
조사한 `claude.com/blog/<slug>` 주소 6개는 같은 Claude.dev 원문으로 이동하므로
독립 출처 12개로 세지 않았다.
([운영 주체](https://claude.dev/terms/),
[출처 계보](agent-updates-2026-10-02/claudedev-sources.md))

| 글·게시일 | issueops에 가져올 판단 | 그대로 가져오지 않을 부분 |
|---|---|---|
| [Seeing like an agent](https://claude.dev/blog/seeing-like-an-agent/), 2026-04-10 | 질문·상태·결과를 명시적인 도구 계약으로 분리 | 모든 코드 탐색·RAG를 일괄 폐기 |
| [Prompt caching is everything](https://claude.dev/blog/lessons-from-building-claude-code-prompt-caching-is-everything/), 2026-04-30 | 안정적인 tools/system prefix와 변동 메시지 분리 | MCP 서버가 호스트 cache hit를 보장한다는 해석 |
| [A harness for every task](https://claude.dev/blog/a-harness-for-every-task-dynamic-workflows-in-claude-code/), 2026-06-02 | 가치가 큰 독립 작업의 fan-out·검증 조합 | 모든 작업의 대규모 분할, exactly-once 효과 가정 |
| [How we use skills](https://claude.dev/blog/lessons-from-building-claude-code-how-we-use-skills/), 2026-06-03 | 짧은 trigger, 구체적인 gotcha, 필요한 reference만 로딩 | Claude 전용 hooks·plugin 경로의 공용 스킬 유입 |
| [Context engineering for Claude 5](https://claude.dev/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models/), 2026-07-24 | 중복·상충 지침 축소와 단계별 문맥 | 내부 80% 축소 사례를 근거로 안전·권한 규칙 삭제 |
| [What a task costs on Opus 5.5](https://claude.dev/blog/what-a-task-costs-on-opus-5-5/), 2026-09-25 | 완료 작업 단위의 turn·retry·cache read/write·output 비용 | 예시 비용이나 내부 절감률을 issueops 절감률로 전용 |

중요한 반례도 확인했다. workflow의 저장 결과 replay가 외부 쓰기의
exactly-once를 뜻하지 않고, 실패 결과 `null`을 제거하면 미완료 작업이 통계에서
사라진다. 글의 token budget 예시와 런타임이 강제하는 agent 수 상한도 다르다.
compaction 요약 요청에서 cache를 재사용하더라도, 요약된 새 문맥은 다시
cache write가 필요할 수 있다.
([글별 계약 대조](agent-updates-2026-10-02/claudedev-content.md))

issueops의 module map과 단계별 skill router는 이미 이 방향을 따른다.
다음 개선은 규칙을 무조건 줄이는 것이 아니라 “언제 어떤 규칙이 실제로
읽혔는가, 잘못 호출되거나 누락된 스킬은 무엇인가”를 측정하는 쪽이다.

### 다른 에이전트에서 확인한 패턴

아래 버전은 각 조사에서 확인한 배포 채널 기준이다. preview와 과거 사례를
현재 정식 기능으로 합치지 않았다.

| 제품 | 확인한 버전·날짜 | 참고할 변화 | issueops에 적용할 때의 한계 |
|---|---|---|---|
| Gemini CLI | v0.62.0, 2026-09-29 | PTY 정리·출력 마무리·terminal buffer 관리 | v0.63 preview의 memory 개선과 분리 |
| Copilot CLI | v1.0.91, 2026-10-01 | 중단 뒤 busy 상태 정리, bounded telemetry flush, MCP discovery 복구 | dynamic workflows는 preview |
| Cursor | 2026-08-13·08-19 업데이트 | 준비 환경 재사용, 이벤트 기반 재개, run diagnostics | 클라우드 자체 측정치를 로컬 성능으로 환산 불가 |
| OpenCode | v1.18.34, 2026-09-30 | session 계보 header, timeout·진단 개선 | Omo Native와 별도 제품·실행 경계 |
| Cline | desktop v4.1.22 / SDK v0.0.89, 2026-09-30 | persistent Git index, checkpoint outcome·duration | SDK 구현과 확장 문서의 snapshot 시점이 다름 |
| Goose | v1.52.0, 2026-09-23 | extension transport 분리, recipe·schedule·session 관측 | 새 scheduler 도입 필요성을 입증하지 않음 |
| OpenHands | v1.24.0, 2026-09-25 | 실행 환경·벤치마크 입력 고정과 task별 근거 | 일부 runtime 문서는 과거 예시, Docker 도입 근거 아님 |
| Aider | v0.86.1, 2025-08-13 | ranked repo map, 별도 map-refresh·prompt cache 제어 | 최근 업데이트가 아니라 검증된 과거 패턴으로 참고 |
| Continue | CLI 1.5.47, 2026-06 배포 근거 | headless JSON, 선택적 OTEL, config 우선순위 | IDE 릴리스와 CLI 버전 및 anonymous telemetry를 구분 |
| ACP | 프로젝트 1.10.2, 2026-10-01 | 정형화된 세션·도구 업데이트와 진단 | ACP v2는 draft이며 SDK 번호와 wire version은 다름 |

각 행의 공식 URL·버전·반례는
[Gemini](agent-updates-2026-10-02/other-01.md),
[Gemini 버전 교정](agent-updates-2026-10-02/release-corrections.md),
[Copilot](agent-updates-2026-10-02/other-02.md),
[Cursor](agent-updates-2026-10-02/other-03.md),
[OpenCode](agent-updates-2026-10-02/other-04.md),
[Aider](agent-updates-2026-10-02/other-05.md),
[Cline](agent-updates-2026-10-02/other-06.md),
[Continue](agent-updates-2026-10-02/other-07.md),
[OpenHands](agent-updates-2026-10-02/other-08.md),
[Goose](agent-updates-2026-10-02/other-09.md),
[ACP](agent-updates-2026-10-02/other-10.md)에 있다.

## issueops 적용 후보

### 먼저 관측 결과의 누락과 시간 계산을 바로잡는다

`trace analyze`의 JSONL parser는 기본 Scanner로 읽고 `scanner.Err()`를
확인하지 않는다. padding 100자일 때 두 실패를 모두 잡는 입력이 padding
70,000자에서는 첫 실패만 남기고 `ok:true`로 끝났다. 이는 추정 병목이 아니라
재현된 관측 누락이다.
([재현 방법과 대조군](agent-updates-2026-10-02/trace-jsonl-reproduction.md))

소요시간도 먼저 의미를 맞춰야 한다. `ValidateContractCheckWithDeps`의 성공
경로는 자식 명령의 duration을 쓰지 않고 마지막 순간에 새 시계를 시작한다.
같은 결함이 아래 네 곳에 있다.

- `internal/adapter/verification/probe/contractauditworker/validation_contract_check.go:47`
- `internal/adapter/verification/probe/contractauditworker/validation_tool_conformance.go:47`
- `internal/adapter/verification/probe/contractauditworker/validation_worker_lifecycle.go:46`
- `internal/adapter/verification/probe/contractauditworker/validation_command_audit.go:42`

기존 self-verify에는 per-label duration과 p95가 있지만 정상 단일 pass에서
표본이 하나면 p95는 그 한 값이다. “p95 개선”이라는 표시는 표본 수·재사용
여부·실제 측정 경계를 함께 밝혀야 의미가 있다.
([코드와 테스트 조사](agent-updates-2026-10-02/issueops-09.md))

제안은 새 모니터링 서비스를 만드는 것이 아니라, 누락·오류·재사용·미실행을
명시하고 현재 계측값을 정확하게 보존하는 것이다.

### 구조화 MCP 결과는 현재 SDK에서도 검토할 수 있다

공유 Tool DTO와 SDK 등록 경계는 name·description·inputSchema만 전달한다.
일반 응답은 JSON 문자열을 TextContent에 넣고, direct content는 text block만
보존한다. 현재 고정된 Go SDK v1.6.1에는 이미 StructuredContent,
OutputSchema, ToolAnnotations가 있다.
([직접 코드 확인](agent-updates-2026-10-02/mcp-output-verification.md))

대표적인 읽기 전용 도구부터 명시적 output contract를 보존하고 round-trip을
검증하는 변경은 최신 protocol/HTTP 전환과 분리할 수 있다. annotation은
도구 전체의 실제 행동을 설명해야 한다. 여러 action을 받는 `issueops_execution`
전체에 read-only hint를 붙이는 식의 단순 분류는 맞지 않는다. 권한·정책
판정을 hint에 맡기지도 않는다.

기대 효과는 기계가 결과를 검증하고 표시하기 쉬워지는 것이다. text와 structured
결과를 함께 보내면 payload가 커질 수 있으므로 토큰 절감으로 광고하지 않는다.

### MCP 목록 비용은 실제 호스트 컨텍스트와 분리해서 측정한다

현재 바이너리에 stdio로 `initialize`와 `tools/list`를 호출한 결과,
**51개 도구**와 **30,888 bytes**의 compact JSON 도구 배열을 관측했다.
이는 토큰 수가 아니며 다른 MCP 서버의 목록도 포함하지 않는다.
실제 응답과 구현은 “issueops MCP 도구는 하나뿐”이라는 초기 분석을 반박한다.
`IssueOpsBasicTools`는 전체가 아니라 여러 capability 목록 중 하나다.
([실행 근거와 환경](agent-updates-2026-10-02/mcp-baseline.md),
`internal/adapter/inbound/catalog/mcp/catalog.go:18-44`)

따라서 전체 descriptor 크기와 호스트별 도구 검색 동작을 측정할 이유는 있다.
그러나 서버 목록을 줄이는 변경부터 시작할 근거는 아직 없다. 같은 목록이라도
호스트가 지연 로딩하는지, 이미 선택한 도구를 어떻게 유지하는지에 따라 모델
컨텍스트 비용이 달라진다. 기존 CLI/MCP 계약을 보존한 채 관측부터 해야 한다.

### 호스트 trace와 CLI trace의 연결은 hook 밖에서 검토한다

Claude 문서는 추적과 전파가 활성화되면 Bash·PowerShell 자식 프로세스에 W3C
`TRACEPARENT`를 전달한다고 명시한다. 기본적으로 `ANTHROPIC_BASE_URL`이 없거나
Anthropic API를 가리킬 때 전파하며, custom endpoint에는
`CLAUDE_CODE_PROPAGATE_TRACEPARENT=1`이 필요하다. 비교 실험에서는 이 두 변수와
추적 활성화 조건을 고정해야 한다. 기존 issueops trace가 이를 활용할 수
있는지 확인하면 context-only hook을 확장하지 않고도 호스트 요청과 CLI 작업을
연결할 수 있다. 실제 지원 여부와 변경 필요성은 로컬 구현 조사 결과로 판단한다.
([공식 monitoring 문서](https://code.claude.com/docs/en/monitoring-usage),
[원문 검증](agent-updates-2026-10-02/telemetry-verification.md))

관측값에서는 API 지연, tool 실행, 승인 대기, cache token, 총 wall time을
분리해야 한다. 공식 비용 지표도 청구액이 아닌 근삿값이다. 프롬프트나 도구
원문을 수집해야 지연을 관측할 수 있는 것은 아니며, 일부 상세 span은 beta
endpoint와 조직 allowlist 등의 조건이 있으므로 기본 기능으로 가정하지 않는다.

### Streamable HTTP는 프로토콜 버전부터 나눠 비교해야 한다

최신 정식 MCP 명세는 **2026-07-28**이다. 초기 조사에서 읽은 2025-11-25는
현재 issueops의 관측된 협상 버전과 비교할 이전 명세이지 최신 기준이 아니다.
두 버전 모두 stdio와 Streamable HTTP를 정의하지만, HTTP의 세션·취소·재전송
계약은 크게 달라졌다.
([정식 발표](https://blog.modelcontextprotocol.io/posts/2026-07-28/),
[최신 transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http))

| 구분 | 2025-11-25 | 2026-07-28 |
|---|---|---|
| 초기화·세션 | initialize/initialized, 선택적 MCP-Session-Id | handshake·프로토콜 세션 제거, 요청마다 metadata |
| HTTP endpoint | POST와 선택적 GET SSE | POST 중심, 기존 GET·DELETE는 새 계약에 없음 |
| 변경 알림 | GET 기반 SSE | subscriptions/listen 요청의 응답 stream |
| HTTP 취소 | 단절을 취소로 해석하지 않음, 명시적 notification | 해당 요청의 SSE 응답 stream 종료를 취소로 처리 |
| 재전송 | 선택적 Last-Event-ID replay | Last-Event-ID 재개 미지원 |
| gateway 관측 | 이전 HTTP 계약 | Mcp-Method·Mcp-Name과 본문 일치 검증 |

이 비교는 두 공식 명세를 직접 읽은 결과다. 문서의 새 기능이 현재 설치된
호스트와 Go SDK에서 모두 동작한다는 뜻은 아니다.
([이전 transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports),
[최신 transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http))

따라서 “streamable이므로 장기 작업·재시작·중복 실행 문제가 자동 해결된다”는
주장은 채택하지 않는다. issueops의 native actor·generation·workspace 검증을
독립 HTTP 서버에 그대로 옮기면 현재 native actor 증명을 보존할 수 없다.
MCP adapter는 **서버 자신의 PID**에서 계보를 관측하고, 요청의 process receipt가
그 계보에 포함되는지 검사한다. 공유 HTTP 서버의 부모는 요청을 보낸 각각의
native 세션이 아니다. 이전 2025-11-25의 HTTP session ID, 요청별 clientInfo,
인증된 OS 사용자만으로는
특정 generation의 holder라는 증명을 대신하지 못한다.
(`cmd/issueops/mcpcli/mcp_tool_issueops_execution.go:54-75`,
`internal/domain/issueops/native_actor.go:23-43`)

**권고는 native mutation에 stdio를 유지하는 것이다.** 원격 조회 수요가 실제로
있다면 인증된 read-only 도구와 workspace 범위를 선별한 별도 HTTP adapter를
검토할 수 있다. 현재 전체 catalog를 공개하거나 caller가 보낸 PID를 신뢰하는
방식은 제외한다. mutation까지 원격화하려면 위임 principal·scope·만료·철회와
commit 직전 fencing을 새로 설계해야 하며, 이는 단순 transport 최적화가 아니다.
([Astra 구조 검토](agent-updates-2026-10-02/streamable-boundary.md))

현재 stdio 경로의 단일 응답 시간을 HTTP 전환의 성능 근거로 사용하지 않는다.
프로토콜 업그레이드, 전송 방식 추가, long-running Tasks 도입은 서로 다른
변경으로 분리해 평가한다. 최신 Tasks는 core API가 아닌 extension이며,
stateful application을 금지하는 것도 아니다.
([Tasks 설명](https://blog.modelcontextprotocol.io/posts/2026-07-28/#tasks))

현재 Go SDK v1.6.1에는 Streamable HTTP handler 자체가 있지만 최신 지원
프로토콜 목록은 2025-11-25까지다. `Stateless` 옵션을 켜는 것이
이 pin에서 2026-07-28 지원 스위치는 아니다.

반면 **Go SDK v1.7.0부터 2026-07-28을 지원하며**, 조회 시점 최신 정식판은
v1.8.0(2026-09-14)이다. SDK 의존성 자체의 갱신은 같은 v1 계열의 minor
업그레이드다. v1.7.0 이상에서는 새 HTTP revision을 쓰려면
`StreamableHTTPOptions.Stateless=true`가 필요하고, stateful handler에서는
이전 revision으로 협상한다. 릴리스가 session identifier 의존 코드의
migration 비용을 별도로 경고하므로 minor 갱신을 무위험으로 간주하지 않는다.
([v1.7.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0),
[v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0))

Claude·Codex·Omo 문서의 URL 기반 연결 지원도
최신 revision conformance를 증명하지 않는다. 호스트별로 version/header,
취소, subscription, legacy fallback을 별도 검증해야 한다.
([지원 범위 조사](agent-updates-2026-10-02/streamable-spec.md))

## 우선순위와 검증 실험

### 제안 순서

P0는 최적화 판단의 전제를 깨는 관측 정확성, P1은 좁은 경계에서 개선 가능한
가시성·상호운용성, P2는 측정 후 결정할 최적화, P3는 별도 수요가 필요한
확장이다. 비용은 상대적 범위이며 일정 추정치가 아니다.

| 순위 | 제안 | 근거 수준 | 예상 범위·비용 | 주요 리스크 |
|---|---|---|---|---|
| P0 | trace JSONL의 불완전 분석·scanner 오류 표시 | 실제 CLI 대조 재현 | 작음: decoder·응답 경고·회귀 검증 | 기존 malformed-line 복구와 상한 유지 |
| P0 | 성공 probe duration 보존, 재사용·표본 수 구분 | 직접 코드 확인 | 작음: 측정 경계와 deterministic 테스트 | 과거 baseline과 새 값의 의미 차이 |
| P1 | host별 설치·링크·발견·실행 지원 상태 분리 | 코드·인접 테스트 | 중간: inspect DTO·CLI/MCP 출력 | 파일 존재를 실제 capability로 오인 |
| P1 | 대표 도구의 구조화 결과·정확한 annotation | 현재 SDK와 변환 경계 확인 | 중간: 공유 계약·golden·host round-trip | payload 증가, 잘못된 행동 hint |
| P1 | MCP protocol·SDK·호스트 지원 matrix | 2025/2026 명세, pin과 최신 v1.8.0 | 작음~중간: SDK minor 갱신·compatibility fixtures | HTTP 취소·세션 계약 혼동 |
| P2 | preflight history 조회와 next의 동일-root 관측 통합 | 정적 호출 경로 | 작음~중간: request-local snapshot | fetch·동시 변경 뒤 stale 결과 |
| P2 | catalog metadata 읽기와 capped selection 개선 | 코드의 읽기·정렬·상한 확인 | 중간: bounded reader·선택·diagnostics | H1 fallback·symlink 안전성 손상 |
| P2 | 기존 span wait/hold와 단계 지연의 원인 분리 | 현재 observer 존재 확인 | 중간: 선택적 관측 필드 | 계측 overhead와 식별자 노출 |
| P2 | 호스트 usage·TRACEPARENT의 읽기 전용 연결 | 공식 호스트 계약·로컬 공백 | 중간: 정규화 adapter | beta·host 차이, 추정 비용을 청구액으로 오인 |
| P3 | Streamable HTTP·Tasks·ACP·추가 scheduler | 표준·타 제품 기능 | 큼: 인증·수명주기·호환성·운영 | 기존 native actor authority를 잃을 위험 |

로컬 근거는
[설치·capability](agent-updates-2026-10-02/issueops-03.md),
[catalog 읽기](agent-updates-2026-10-02/issueops-04.md),
[trace·metrics](agent-updates-2026-10-02/issueops-05.md),
[subprocess](agent-updates-2026-10-02/issueops-06.md),
[next/readiness](agent-updates-2026-10-02/issueops-07.md),
[SQLite](agent-updates-2026-10-02/issueops-02.md)에 있다.

### 중복 구현하지 않을 현재 기능

- readiness의 branch/status는 이미 operation-local cache를 쓰고 fetch 경계에서
  비운다. 전역 cache를 새로 추가하는 제안은 하지 않는다.
- Omo MCP catalog SHA를 설정 env에 넣어 schema 변경을 무효화하는 installer
  경로가 이미 있다.
- SQLite span wait/hold/contention observer와 WAL 유지보수 결과가 이미 있다.
- review-metrics는 phase duration·review round·verdict·read_errors를 제공한다.
- self-verify는 같은 pass의 성공 suite 근거를 재사용한다. 무조건 테스트를
  더 생략하는 제안은 아니다.
- native install은 staging·activation·digest receipt를 사용한다. 현재
  진단용 `+dirty` 표기의 한계를 activation 검증 부재로 해석하지 않는다.

반면 과거 ADR의 external-LLM usage writer 경로는 현재 checkout에서 찾지 못했다.
그 문서만으로 지금도 per-call token 비용을 기록한다고 주장하지 않는다.
([MCP](agent-updates-2026-10-02/issueops-01.md),
[state](agent-updates-2026-10-02/issueops-02.md),
[설치](agent-updates-2026-10-02/issueops-03.md),
[관측](agent-updates-2026-10-02/issueops-05.md),
[검증](agent-updates-2026-10-02/issueops-09.md))

### 최소 실험과 수용 기준

먼저 할 세 작업은 다음처럼 작게 나눌 수 있다. 이는 구현 승인이 아니라
조사 결과에 따른 제안이다.

1. **관측 정확성:** JSONL oversize/malformed/후속 정상 행의 입력별 결과를
   고정하고, 불완전 분석이 성공으로 숨지 않게 한다. duration 검증은 sleep
   대신 주입된 command 결과의 시간을 보존하는지 확인한다.
2. **호스트·MCP 상태 표현:** 설치 경로 존재, source/link 일치, client 발견,
   실제 연결, protocol 지원을 각각 다른 상태로 표현한다. 네트워크가 없는
   점검에서 live-ready라고 표시하지 않는 것이 수용 기준이다.
3. **대표 읽기 도구의 결과 계약:** 기존 text 응답을 유지한 채 structured
   결과와 schema를 시험하고, 세 호스트의 성공·오류 round-trip을 비교한다.
   공유 DTO·CLI JSON·MCP 의미가 달라지면 채택하지 않는다.

| 후보 | 관측할 값 | 통제할 조건 | 채택 전 확인 |
|---|---|---|---|
| MCP 지연 로딩 | 초기 context, tool schema 비용, 첫 도구 선택 지연, 선택 성공률 | 같은 호스트·모델·작업·서버 목록 | 절감과 추가 검색 지연을 함께 비교 |
| 정적 catalog 주입 | SessionStart 지연, 주입 bytes, compact 뒤 필수 지침 접근 | startup/resume/compact 각각 구분 | hook의 context-only 계약 유지 |
| 실행 가시성 | queued/running/approval-wait/completed 구분, 누락·중복 이벤트 | 동일한 실행 ID·generation | 호스트 상태와 core authority 혼동 방지 |
| Streamable HTTP | 연결 비용, 재연결·취소·중복 실행, 자원 사용 | 동일 도구·payload·인증 조건 | actor 검증과 운영 비용까지 포함 |

최적화 후보의 구체적인 합격 조건도 분리한다.

- **반복 조회 통합:** 같은 요청의 history 조회와 동일-root 관측 횟수는 줄되,
  다른 base ref·fetch 후·동시 변경에서는 새 관측을 사용해야 한다. 전역
  first-root cache나 cached authorization은 제외한다.
- **metadata-only discovery:** 큰 문서에서도 읽은 bytes·allocation이 줄어야
  하고 title·description·manifest 필드는 같아야 한다. 정렬 뒤 cap 선택,
  필수 문서 우선순위, 생략 수 표시와 symlink/root 경계도 함께 검증한다.
- **span 관측:** existing wait/hold/contended에 lock 대기·callback·commit·총
  경과시간을 구분하되 계측 자체의 overhead를 측정한다. actor/generation
  검증을 잠금 밖의 오래된 snapshot으로 옮기는 방식은 제외한다.
- **host usage 연결:** token·cache read/write·재시도·미실행·unknown을 분리한다.
  API 단가 환산과 subscription 비용을 섞지 않고, 누락한 값에 0을 넣지 않는다.

성능 비교는 같은 commit·모델·host 버전·입력·권한·동시성·cache 조건에서
독립 실행을 비교한다. 평균 토큰만 줄이고 재시도나 실패율이 늘면 개선으로
채택하지 않는다. 성공한 작업당 시간·사용량과 결과 품질을 함께 기록한다.
반복 수가 작으면 분포를 과장하지 않고 원자료와 표본 수를 제시한다.
([평가 근거](agent-updates-2026-10-02/shared-09.md))

## 반증과 제외한 제안

- **전체 MCP 목록이 도구 하나라는 주장:** 실제 목록은 51개다. 하위 카탈로그를
  전체 서버로 오인한 분석은 교정했다.
- **v2.1.286의 특정 per-turn 성능 개선 문장:** 지정된 공식 릴리스에서 해당
  문장을 확인하지 못했다. 출처 버전이 확인될 때까지 결론에서 제외한다.
- **Streamable HTTP가 stdio를 폐기했다는 해석:** 공식 명세와 다르다.
- **공급자 개선 발표를 issueops 절감 수치로 사용:** 비교 측정이 없으므로 제외한다.

## 출처와 조사 한계

조사 범위와 근거 계약은 [조사 개요](agent-updates-2026-10-02/brief.md)를 따른다.
현재 작성된 내용의 원문 조회일은 2026-10-02다. 공식 문서와 같은 공급자의
GitHub 릴리스는 독립된 두 출처로 계산하지 않았다. 릴리스 자체의 기능·날짜는
단일 1차 출처 예외로 취급하며, 효과에 대한 일반화는 별도 검증 대상으로 남겼다.
총 60개 기본 조사와 Claude.dev·Streamable HTTP 추가 조사를 수행하고,
원문·설치 코드·현재 issueops를 대조했다. 수집 노트의 초기 판단보다 메인의
재현 기록과 교정된 집계 결과를 우선한다.

이번에 직접 관측한 실행 증거는 stdio MCP 목록과 JSONL 입력 대조다.
자식 작업의 모델 영수증에서 Luna·Sol·Astra도 확인했다.
호스트 설정을 바꾸거나 HTTP 서버·mod·collector를 설치한 성능 비교는 하지
않았으므로 개선율과 실제 청구 절감률은 제시하지 않는다.

### 검증 결과와 남은 한계

- 실제 stdio 초기화·도구 목록 호출 성공: 51개 도구, JSON 배열 30,888 bytes.
- JSONL 큰 행 뒤 실패 이벤트 누락: 동일 행 수·이벤트를 둔 대조군으로 재현.
- 조사 변경 범위: `.issueops/research/`의 Markdown뿐이다.
- Markdown LSP는 등록되어 있지 않다. 대신 문서·링크·범위 검사를 수행한다.
- 프로젝트 self-verify는 `Python script tests`에서 `pydantic` 미설치로
  exit 1이었다. 후속 Go test·build까지 통과한 결과가 아니다.
  [정확한 명령과 traceback](agent-updates-2026-10-02/self-verify-result.md)을 남겼다.
- Fable 5.1의 독립 판정은 **수정 후 게재 가능**이었다. 중간 2건·낮음 4건을
  메인이 원문·코드로 확인해 반영했다. 재검토를 다시 받았다는 뜻은 아니다.
  [검토 결과와 반영표](agent-updates-2026-10-02/fable-5.1-review.md)를 남겼다.

### 별도로 발견한 문서 불일치

`configs/upstream.json`은 plugin 4개·Git skill 2개인데
`.issueops/operations/hosts.md:94`와 `.issueops/TECH_STACK.md:45`는 skill
1개라고 적는다. source `SKILL.md`는 52개이며 `TECH_STACK.md:55`의 34개와 다르다.
이는 source inventory와 문서의 불일치이지 host 설치 성공·실패의 증거가 아니다.
이번 조사에서 운영 문서를 변경하지 않았다.

전체 조사 노트와 교정·실행 기록은 [근거 색인](agent-updates-2026-10-02/sources-ledger.md)에서 찾을 수 있다.
