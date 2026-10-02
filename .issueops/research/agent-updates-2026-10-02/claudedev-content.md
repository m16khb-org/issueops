# Claude.dev 기술 주장과 issueops 적용 검토

조회일: **2026-10-02**. 결론: 단계별 문맥 로딩, 안정적인 도구 계약, 성공한 작업 단위의 비용 측정은 적용 후보다. Claude 전용 workflow·메모리·HTTP transport를 공용 core에 복제하거나, 내부 평가 수치를 issueops 성능 개선치로 사용하는 근거는 없다.

## 방법과 출처 계보

- **Source fan-out:** 지정 글 6개, `claude.com` 대응 주소 6개, API·Claude Code·Agent Skills·MCP 명세, 독립 Chroma 실험, 저장소 구현을 직접 조회했다. 에이전트나 조사 오케스트레이션은 사용하지 않았다.
- **Source index:** 아래 URL은 모두 조회일에 본문을 가져왔다. 글 날짜는 HTML의 `article:published_time`과 JSON-LD `datePublished`를 대조했다. 변경 이력이 없는 현재 문서는 역사적 동작의 증거가 아니다.
- **Claim verification:** 계약·코드는 직접 확인했다. Anthropic 글과 Anthropic 문서의 일치는 같은 공급자의 계약 확인이지 독립 재현이 아니다. 내부 평가·사용량 통계는 단일 출처 주장으로 분리했다.
- **Access boundary:** 인용한 공개 자료에서 접근 차단은 없었다. secret·인증·세션 전사는 읽지 않았다. 설치·설정·state·Git 변경은 수행하지 않았다.

| ID | canonical URL | 게시일·저자 |
|---|---|---|
| A | https://claude.dev/blog/a-harness-for-every-task-dynamic-workflows-in-claude-code/ | 2026-06-02, Thariq Shihipar·Sid Bidasaria |
| B | https://claude.dev/blog/seeing-like-an-agent/ | 2026-04-10, Thariq Shihipar |
| C | https://claude.dev/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models/ | 2026-07-24, Thariq Shihipar |
| D | https://claude.dev/blog/lessons-from-building-claude-code-how-we-use-skills/ | 2026-06-03, Thariq Shihipar |
| E | https://claude.dev/blog/lessons-from-building-claude-code-prompt-caching-is-everything/ | 2026-04-30, Thariq Shihipar |
| F | https://claude.dev/blog/what-a-task-costs-on-opus-5-5/ | 2026-09-25, Addy Osmani |

`https://claude.com/blog/<동일 slug>` 6개를 실제 요청한 결과, 모두 위 `claude.dev` 주소로 이동했다. 최종 HTTP 200, self-canonical, 날짜, 추출 Markdown의 완전 일치를 확인했다. 별도 재게시물 6개가 아니라 같은 문서의 별칭이다. A–E는 작성자가 서로 겹치는 경험담이며 F도 공급자 글이다. 글 수를 독립 증거 수로 세면 안 된다. 소셜 게시물의 최초 발표 시각까지 확정한 것은 아니다.

## 글별 메커니즘·한계·적용

### A: 동적 workflow

JavaScript가 `agent()`로 별도 문맥의 하위 에이전트를 생성하고, `parallel()`·`pipeline()`으로 조합한다. 모델·격리·출력 schema를 선택해 분류, 병렬 조사, 적대 검증, 토너먼트를 구성한다. 깨끗한 문맥은 목표 드리프트·자기 선호를 줄이려는 설계이며, 감소 효과의 독립 측정은 제시하지 않는다.

[현재 workflow 계약](https://code.claude.com/docs/en/workflows)은 스크립트의 직접 파일·shell 접근과 `import()`를 금지한다. 기본 동시성은 최대 16이며 CPU에 따라 낮아지고, 호출별 목록은 4,096개, 전체 run은 1,000 agents까지다. 중단·복구 불가능 API 오류의 결과는 `null`이다. 이를 단순 필터링하면 미완료 항목이 보고서에서 사라질 수 있다.

재개는 같은 세션의 저장 결과를 순서대로 replay한다. 중간 실패 뒤에 시작한 성공 agent도 재실행될 수 있으므로 exactly-once 외부 쓰기가 아니다. 같은 모델·effort·agent type·tools·schema·cwd의 fan-out은 첫 응답 시작까지 나머지를 최대 5초 보류해 prefix cache를 공유한다.

**반증·제한:** A의 “10k token cap”은 조회한 계약에서 강제 token budget으로 확인되지 않았다. size guideline과 대규모 경고는 advisory이고 agent 수 상한과 다르다. 무한 “loop until done”도 issueops 원칙과 충돌한다.

**적용:** 제한된 독립 검증 작업의 템플릿만 후보로 삼는다. 실행 권위는 workflow가 아니라 core lease에 남긴다. `internal/application/issueopslease/claim_transaction.go:24-55`는 generation·actor·canonical cwd·token 검증 경로를 갖고 있다. 임의 하위 에이전트 생성은 이 계약을 대체하지 못한다.

### B: 에이전트가 문맥을 직접 찾는 도구

질문을 계획 제출 도구에 붙이면 답에 따라 계획을 다시 작성해야 한다. 전용 AskUserQuestion schema·UI로 분리하면 질문 시점과 응답 구조를 명확히 할 수 있다. Markdown 파싱과 달리 도구 계약으로 구조를 받는다. 이는 구현 경험담이며 질문 시간 감소의 수치는 없다.

초기 vector RAG를 grep·파일 탐색으로 바꾼 이유는 indexing/setup 취약성과 주어진 문맥에 대한 의존이다. 문서 검색 전용 agent는 상세 검색 결과를 자체 문맥에 두고 답만 반환한다. Todo에서 dependency task로의 전환은 다중 agent 소통 요구에 따른 변화다.

**적용:** 상태·수용 기준을 자연어 기억 대신 구조화한다. 그러나 기존 코드 탐색 도구나 검색 인덱스를 일괄 폐기할 근거는 없다. [2025-09-29의 Anthropic context 글](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)은 선행 retrieval과 자율 탐색의 hybrid도 인정한다.

### C: 지침 축소와 점진적 공개

중복 규칙·상충 지침·장황한 예시를 줄이고 enum·schema·도구 설명에 계약을 둔다. 검증·리뷰 지침은 필요한 skill로 이동하고, 재사용 문맥은 가벼운 경로로 연결한다. “system prompt 80% 이상 삭제, coding eval 손실 없음”은 공개 평가 데이터·분산·실패 유형이 없는 내부 주장이다.

**반증:** [Chroma Context Rot](https://research.trychroma.com/context-rot)은 18 LLM의 통제 실험에서 문맥 길이와 distractor에 따른 비균일 성능 저하를 측정한다. 짧고 관련 있는 문맥의 필요성을 뒷받침하지만 Claude 5 평가나 안전 규칙 80% 삭제를 검증하지 않는다. 해당 자료에는 2025-07-16 수정 주석이 있으며 이를 최초 게시일로 해석하지 않았다.

**적용:** `.issueops/ARCHITECTURE.md:17-29`의 module map과 `skills/issueops/SKILL.md:65-83`의 단계별 로딩은 이미 같은 방향이다. `.issueops/CONSTITUTION.md`의 host parity·workspace 경계·유한 반복은 모델 판단에 맡길 스타일 지침이 아니다.

### D: skill은 파일 하나가 아닌 폴더

설명은 호출 조건을 담고, 본문은 gotcha·경로·도구 사용 지식을 제공하며 scripts/assets/references는 필요할 때 읽는다. PreToolUse 계측은 popularity와 undertriggering을 볼 수 있지만 성공률을 직접 증명하지 않는다. skill 간 의존성을 이름으로 참조하는 것은 dependency manager가 아니다.

[Agent Skills 명세](https://agentskills.io/specification)는 metadata → 본문 → resources의 점진적 로딩을 정의한다. [Claude Code skills 계약](https://code.claude.com/docs/en/skills)은 `hooks`가 호출 뒤 세션 동안 유지된다고 명시한다. `${CLAUDE_PLUGIN_DATA}`는 plugin skill에서만 치환된다. Claude frontmatter 확장을 다른 host가 이해한다고 가정하면 안 된다.

**적용:** 공용 skill 원본과 host-neutral 경로를 유지한다. Claude용 임시 hooks·plugin 데이터 경로·repo-local 배포 예시는 공용 설치 정책으로 옮기지 않는다.

### E: prefix cache 중심의 요청 설계

tools → system → messages의 안정적인 prefix를 보존하고 변동 정보는 뒤쪽 메시지로 전달한다. 모델 변경은 별도 cache를 만들고 도구 정의 변경은 후속 cache를 무효화한다. compaction 요약 호출은 부모의 system·tools·history를 공유해야 기존 읽기 cache를 사용한다. 요약 이후 짧아진 대화는 새 cache 쓰기가 필요하므로 “compaction 전체가 cache-safe”는 아니다.

[API cache 계약](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)은 최대 4 breakpoints, 20-position lookback, 모델별 최소 길이와 5분/1시간 TTL을 명시한다. Opus 5.5 최소 길이는 현재 512 tokens다. `defer_loading`은 Claude API/host 기능이지 MCP transport 기능이 아니다.

**적용:** `internal/adapter/inbound/catalog/mcp/catalog.go:13-44`는 이미 고정 순서 catalog를 정의한다. `cmd/issueops/mcpcli/mcp_sdk_server.go:135-153`는 일반 MCP schema를 등록한다. 여기에서 host의 prompt 조립·cache hit까지 보장하지는 않는다.

### F: token 가격이 아닌 완료 작업 비용

[현재 가격 계약](https://platform.claude.com/docs/en/about-claude/pricing)은 Opus 5.5의 MTok당 fresh input $4, output $20, cache read $0.20, 5분 write $5, 1시간 write $8을 확인한다. F의 예시도 같은 가격이지만 cache write를 제외한다. 2M read + 200K fresh + 60K output은 $2.40이며, 예시의 Opus 5 $3.50 대비 약 31% 감소다.

“전형적 작업 40% 절감”, prompt-audit의 44-ticket 내부 benchmark, 팀의 약 7배 token은 issueops 측정값이 아니다. API 계약상 `input_tokens`는 전체 입력이 아니다. 총입력은 fresh + cache creation + cache read이고 비용에는 thinking output·write·하위 agent도 포함해야 한다.

**적용:** 작업 성공 여부, turns, retry, output, cache read/write, wall time을 함께 기록하는 실험 후보로 삼는다. effort 변경의 cache 보존은 모델·provider·전달 방식에 따라 달라 보편 규칙으로 채택하지 않는다.

## Streamable MCP와 EXPAND

**버전 교정:** 아래 단락은 인용 글과 비교하기 위해 확인한 2025-06-18의 역사적
계약이다. 최신 정식 2026-07-28은 handshake·session ID·GET stream·Last-Event-ID를
제거하고 HTTP SSE 응답 stream 종료를 취소로 처리한다.
[현재 명세와의 대조](mcp-revision-correction.md)를 최종 판단에 우선한다.

[MCP 2025-06-18 transport 명세](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)는 Streamable HTTP의 단일 endpoint POST/GET, 선택적 SSE·session ID·재전송을 정의한다. 스트리밍은 성능 개선 보장이 아니다. Origin 검증은 MUST, localhost bind·인증은 SHOULD다. 연결 단절은 자동 취소와 같지 않다.

`cmd/issueops/mcpcli/mcp_sdk_server.go:255-274`는 stdio·daemon connection에 **IOTransport**를 사용한다. 조사한 MCP·adapter Go 코드에서 Streamable HTTP handler는 발견하지 못했다. HTTP session ID는 native actor·generation 증명이 아니다.

- **EXPAND-CACHE:** host별 실제 요청에서 catalog 순서·prefix hash·read/write 계측을 비교한다. core에 LLM cache를 새로 만들지 않는다.
- **EXPAND-CONTEXT:** 한정된 반복 작업으로 지침 축소 전후 성공률·누락·권한 위반·비용을 비교한다.
- **EXPAND-WORKFLOW:** null 항목 보존, 부분 실패 replay, token budget 강제 여부와 lease 호환성을 테스트한다.
- **EXPAND-MCP-HTTP:** 원격 연결 필요성이 확인되면 actor binding·Origin·인증·취소·재전송 중복을 먼저 설계하고 stdio와 측정 비교한다.

## 검증 기록

본문·canonical·날짜·가격·transport 계약과 위 코드 앵커 6개를 직접 읽었다. 초기 코드 탐색의 `internal/application/issueops` 경로는 존재하지 않아 실패했고 실제 `issueopslease` 경로로 수정했다. 공개 성능 실험 재실행과 설치 host 호환성 실행은 하지 않았다. 문서 한 파일만 작성하는 범위이며 build·test·self-verify는 state/산출물 변경 제한 때문에 실행하지 않는다. `git diff --check -- .issueops/research/agent-updates-2026-10-02/claudedev-content.md`는 exit 0이었다. 새 untracked 파일은 이 명령의 내용 검사 대상이 아니므로 본문도 별도로 검토했다. 단어 상한은 Unicode word regex로 계수하고 공백·충돌 표지를 별도 검사했다.
