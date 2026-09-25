---
name: 2026-09-24-claude-role-models-opus-5-plans-and-reviews-sonnet-5-impleme
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Claude role models: Opus 5 plans and reviews, Sonnet 5 implements, Fable 5 is manual-only

- Date: 2026-09-24
- Kind: `adr`
- Source: user feedback 2026-09-24 (#517)
- Summary: Claude host의 역할 모델 기본값을 기록한다. 계획과 리뷰는 claude-opus-5/high, 구현은 claude-sonnet-5/high가 맡고, Fable 5는 사용자가 이름으로 지정할 때만 쓴다. 2026-07-24 planner/implementer 기록이 적은 Claude 기본값(fable5/opus4.8)을 대체한다.
- Context: 2026-07-24 ADR이 적은 Claude 역할 모델 기본값은 이후 코드에서 바뀌었지만 대체 기록이 없었다. 리뷰 스킬의 3라운드 규칙은 '다른 모델 또는 한 단계 높은 effort'만 요구해서, 2026-09-24 #513과 #514의 계획 리뷰 3라운드에서 에이전트가 Fable 5를 골랐다. 사용자는 Fable을 명시적으로 지정할 때만 쓰라고 지적했다.
- Decision: Claude 기본값은 코드가 소유한다(internal/port/orca.go의 IssueOpsPlannerModelClaude, internal/contract/issueopspreparation/prepare.go의 ImplementerModelClaude). Fable 5는 자동 기본값, 폴백, 리뷰 3라운드의 대체 모델로 쓰지 않는다. Claude의 리뷰 3라운드는 next.review.model을 쓰고 effort를 next.review.effort에서 한 단계 올려 claude -p 새 세션으로 실행한다.
- Consequences: issueops-review 스킬의 3라운드 규칙이 이 결정을 따른다. 2026-07-24 기록의 planner/implementer 이원 구조는 그대로다. 이미 기록된 리뷰 판정(#513 계획 리뷰 3라운드)은 바꾸지 않는다.
- Evidence:
  - internal/contract/issueopspreparation/prepare.go:18-19
  - internal/port/orca.go:19
  - skills/issueops-review/SKILL.md
- Alternatives / rejected options:
  - Fable 5를 3라운드 대체 모델로 허용한다: 명시적 수동 지정 규칙과 충돌해 기각
  - 3라운드에서도 같은 effort의 같은 모델을 쓴다: 규칙이 요구하는 관점 변화가 없어 기각
