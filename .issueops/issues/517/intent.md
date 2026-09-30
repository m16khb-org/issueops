# 요청자 의도 계약

- lifecycle: io-312a238fdcfe
- issue: https://github.com/m16khb-org/issueops/issues/517
- intent_class: standard

## 원문 요청
fable은 명시적으로 사용하지않으면 사용하지않아야하는데 왜사용하는거지? 코드베이스에 그렇게 되어있다면 고쳐

## 해석
코드와 운영 문서는 Claude의 Fable 5를 명시적 수동 지정 전용으로 정했지만(internal/contract/issueopspreparation/prepare.go:19, .issueops/architecture/issueops.md:42), issueops-review 스킬의 3라운드 규칙('next.review.model과 다른 모델 또는 한 단계 높은 effort')이 Fable을 막지 않아 에이전트가 #513·#514 계획 리뷰 3라운드에 fable을 골랐다. 스킬의 3라운드 규칙에 Fable 5 제외를 명시하고, Claude에서는 next.review.model을 effort 한 단계 올려 claude -p 새 세션으로 띄우는 방법을 적는다. 2026-07-24 ADR에 남은 옛 Claude 역할 모델 기본값(fable5/opus4.8)은 ADR 규칙대로 새 날짜의 ADR로 대체하고 README의 대체 목록에 올린다. 전체 사이클을 draft PR 발행과 execution complete까지 진행한다.

## 성공 기준
- skills/issueops-review/SKILL.md의 3라운드 규칙이 Fable 5를 자동 선택하지 않는다고 명시하고, Claude에서 effort를 올리는 실행 방법(claude -p --model --effort)을 적는다.
- 새 ADR이 Claude 역할 모델 기본값(planner claude-opus-5/high, implementer claude-sonnet-5/high, Fable 5는 명시적 수동 지정 전용)을 기록하고 2026-07-24 기록의 옛 기본값을 대체한다고 밝힌다.
- .issueops/adr/README.md의 대체 목록과 ADR 색인이 새 결정을 가리킨다.
- python3 scripts/validate-skill.py skills/issueops-review, python3 scripts/verify-skill-shell.py skills/issueops-review, go test ./... -count=1이 통과한다.

## 비목표
- 코드의 모델 기본값(internal/port/orca.go, internal/contract/issueopspreparation/prepare.go)은 바꾸지 않는다.
- 과거 기록(.issueops/issues/*, verified-execution JSON, 이미 기록된 리뷰 판정)은 고치지 않는다.
- codex와 omo host의 3라운드 모델 선택 규칙은 바꾸지 않는다.

## 제약
- ADR은 추가만 한다. 옛 기록은 새 날짜의 기록으로 대체한다(.issueops/adr/README.md).
- #513 브랜치도 skills/issueops-review/SKILL.md를 고친다(128행 근처와 177행 근처). 이 사이클은 3라운드 규칙(144~146행)만 바꾼다.

## 모호함
- resolved: 코드와 운영 문서는 이미 Fable 5를 명시적 수동 지정 전용으로 정했다. 고칠 곳은 이 규칙을 반영하지 않은 리뷰 스킬과 옛 ADR의 기본값 표기다.
- resolved: Claude의 서브에이전트 도구는 effort를 받지 않으므로 3라운드 effort 상향은 claude -p 새 세션으로 한다(claude --help의 --model, --effort, -p).

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
