---
name: 2026-11-12-nested-team-structure-is-allowed-under-fixed-conditions
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Nested team structure is allowed under fixed conditions

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session
- Summary: 메인 → 팀장 여럿 → 팀원 구조를 깊이 2 고정, 갈래별 worktree, 커밋 금지, 이중 재검증, 독립 리뷰 조건으로 허용하고, 조건 없는 중첩은 계속 금지한다.
- Context: SUB_AGENT_PATTERNS.md의 금지 패턴 표는 sub-agent 중첩을 금지하고 근거를 "Claude Code: Plan subagent prevents nesting"으로 적었다. 2026-11-11에 현재 호스트에서 서브에이전트가 Agent 도구로 손자 에이전트를 띄우는 것을 확인했으므로 그 근거는 사실이 아니다. 같은 날 사용자가 메인 → 팀장 여럿 → 팀원 구조를 직접 지시했고, 과제 18개(대시보드, 운영 규칙, 개선)를 갈래 8개로 나눠 그 구조로 구현해 main에 넣었다.
- Decision: 1) 금지 패턴 표의 행을 "조건 없는 Sub-agent 중첩"으로 바꾼다. 2) SUB_AGENT_PATTERNS.md에 "중첩 팀 구조" 절을 두고 아홉 조건을 적는다: 깊이 2 고정, 팀장 4명·팀장당 팀원 3명 이하, 팀장별 격리 worktree와 파일이 겹치지 않는 갈래, 팀장·팀원 커밋 금지, 팀장과 메인의 수용 기준 이중 재실행, 그 팀장이 아닌 새 컨텍스트의 diff 리뷰, 병합마다 메인이 실행하는 lint와 전체 battery, 의도·안전·범위·accept 판단은 메인, 계획의 사전 리뷰 게이트. 3) 쓰는 경우를 파일이 겹치지 않는 큰 갈래가 둘 이상인 작업으로 한정한다. 읽기 전용 조사 fan-out과 작은 편집은 평면 구조로 한다. 4) slug는 기존 task-fan-out-coordination과 isolated-worktree-work를 쓴다. 새 slug는 만들지 않는다.
- Consequences: 메인은 팀원의 기록과 토큰 사용량을 볼 수 없으므로 팀장 보고의 명령과 결과를 그대로 재현할 수 있어야 한다. 세션이 다시 시작되면 팀장과 팀원이 알림 없이 끊기고, 재개 때 팀장이 worktree 상태를 다시 검증해야 한다. 평면 구조와 같은 과제로 비교한 측정이 없어 속도 이득은 추정이다. 조건 6(독립 리뷰)과 7(메인의 battery)을 생략하면 금지 패턴이다.
- Evidence:
  - capability probe 2026-11-11: a sub-agent called the Agent tool and received the nested reply
  - commits b511f53d4..49dc09970 on main (24 commits, 9 conflict-free workstream merges)
  - independent reviews found 5 major defects after lead acceptance: completion not clearing failure cause, 64KiB retained row bound, fail-open stage list, sentinel creation-site test gap, lint ordering
  - session restart stalled one workstream for about 8 hours
- Alternatives / rejected options:
  - 금지를 유지하고 쓸 때마다 사용자가 지시: 근거 문장이 사실과 다르고, 조건이 문서에 없으면 다음 세션이 조건 없이 쓸 수 있다
  - 낡은 근거 문장만 정정: 실제로 쓴 구조와 문서가 계속 어긋난다
  - 조건 없이 허용: 팀장이 accept한 갈래 5개에서 독립 리뷰가 major 5건을 찾았다. 팀장의 accept만으로는 부족하다
  - 깊이 3 이상 허용: 메인이 손자 에이전트의 기록과 비용을 이미 볼 수 없다
