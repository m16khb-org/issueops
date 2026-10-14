# 2026-10-13 — The project-doc catalog carries a usage line, and project_docs_route returns owning modules

← [ADR index](../ADR.md)

**결정:** 모델용 catalog는 헤더 줄 바로 다음에 사용 안내 한 줄을 둔다. 내용은 두 가지다. 목차 문서의 모듈 표가 가리키는 파일을 열어 세부 규칙을 확인할 것, 그리고 결정에 문서 규칙을 적용하면 그 규칙 문장을 읽은 파일 경로와 함께 그대로 인용할 것이다. 그렇게 인용할 수 없는 규칙은 문서 규칙이 아니다. 문서 줄 형식, discovery, 정렬, 상한은 바뀌지 않는다. 텍스트 소유자는 `internal/adapter/projectdoc/catalog.go`의 `projectDocCatalogUsage`이고, 모든 호스트의 `SessionStart`·`SubagentStart`가 같은 텍스트를 받는다.

`project_docs_route`는 라우팅된 목차 문서(ADR, ARCHITECTURE, CAUTIONS, CONVENTIONS, OPERATIONS, TESTING)가 속한 family의 살아 있는 모듈 파일 가운데 작업과 단어를 두 개 이상 공유하는 것을 최대 3개 돌려준다. 비교 대상은 모듈의 H1, frontmatter description, `## ` 제목이다. 후보의 3분의 1 이상이 가진 단어는 공유 단어로 세지 않는다. dated 기록과 overview는 모듈 후보에서 뺀다.

이 기록은 [2026-10-08 SubagentStart carries the project-doc catalog](2026-10-08-subagent-start-catalog-and-unified-model-text.md)의 "첫 줄 뒤에 문서마다 한 줄" 형식에 사용 안내 한 줄을 더한다. 그 기록의 나머지 결정과 [2026-08-10 default hooks are thin static context only](2026-08-10-default-hooks-thin-static-context.md)의 "hook은 정적 catalog만 렌더하고 IssueOps state를 읽지 않는다"는 그대로다.

**근거 (`scripts/autoresearch/doc_injection_judge.py`, 2026-10-12~13):** Sonnet 5가 샌드박스의 `.issueops` 문서를 도구로 직접 골라 읽으며 단계 24개를 수행하고, Opus 5.5가 정답 목록 없이 기대 문서를 먼저 정한 뒤 루브릭으로 채점했다.

- 기준선에서 지어낸 규칙이 시나리오당 1.9건이었다. 대부분 스킬이나 AGENTS.md의 규칙을 읽지 않은 문서에서 나온 것처럼 적은 사례였다. 목차 문서(TESTING.md 등)에서 멈추고 규칙을 소유한 모듈(testing/unit-and-contract.md 등)을 열지 않는 사례도 반복됐다.
- "연 파일에서만 인용"이라는 금지형 안내는 효과가 노이즈 범위였다(21개 paired, 지어낸 규칙 1.95→1.86건). "규칙 문장을 경로와 함께 그대로 인용"으로 바꾸자 같은 21개에서 이전 binary 대비 +7.6점(표준오차 2.5, 15승 4패), 지어낸 규칙 1.86→0.57건, rule_grounding 2.48→3.24/4가 나왔다.
- 모듈 라우팅은 목차가 가리키는 모듈을 직접 후보로 보여 준다. 정성 평가에서 모듈 파일 읽기는 시나리오당 1.6→2.2건으로 늘었고, 점수 변화는 노이즈 범위였다.

**대안:**

- `issueops next` 결과를 받는 `PostToolUse` 훅으로 단계 문서를 주입하는 방법: Claude·Codex(0.162.0 내장 schema)·omp(`tool_result`의 `additionalContext`) 모두 가능하다는 것을 바이너리로 확인했다. 하지만 스킬 지시만으로 결정적 측정의 timing이 이미 1.0이라 같은 지시를 한 번 더 넣는 셈이고, 2026-08-10 결정과 네 호스트 설치를 바꿔야 해서 보류했다.
- 훅 안에서 LLM으로 문서를 고르는 방법: 훅 제한 시간(Codex 5초), 호출마다 붙는 지연과 비용, 인증·네트워크 의존 때문에 하네스 독립 실행 불변식과 맞지 않아 기각했다.

**결과:** catalog의 모델용 텍스트가 한 줄 늘었다(약 260바이트). route 결과에는 `module "<title>" of the routed index shares: ...` 사유를 가진 모듈 항목이 추가될 수 있다. 결정적 벤치마크(`autoresearch.sh`)는 catalog 블록 전체를 정적 인식으로 보고 점수에서 제외한다.
