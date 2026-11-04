# 구현 루프 세부 규칙

`SKILL.md`의 `## 프로젝트 문서 확인`과 `## 구현 루프`가 가리키는 세부 규칙이다. 본문은 RED 시작,
canonical worktree 전용 수정, 검증 기록 원칙을 소유하고, 이 문서는 코드베이스 존중 규칙, 요구사항 변경 순서,
문서 규칙 보고 형식을 소유한다.

## 코드베이스를 존중하는 네 규칙

1. **재사용을 먼저 본다.** 기존 함수·패키지·테스트 헬퍼를 확장하는 쪽이 새 파일·새
   추상화보다 앞선다. 계획의 `## 재사용하는 기존 구현`에 없는 새 추상화는 만들지 않는다.
2. **계약 표면은 이슈와 계획이 명시한 것만 바꾼다.** CLI JSON, MCP schema, golden,
   record schema, provider body 계약이 여기 해당한다. 하위 호환이 깨지는 변경은 계획에
   적힌 것만 한다.
3. **hot path를 건드리면 전후를 측정한다.** 측정값을 evidence로 남긴다. 측정 없이
   성능이 나아졌다고 적지 않는다.
4. **side effect를 목록으로 적는다.** 파일·원격·durable state에 남는 변화를 verified-execution
   report에 적는다.

## 구현 도중 요구사항이 바뀔 때

사용자가 요구사항을 바꾸면(완료 기준·범위 변경) 이 순서로 반영한다:
`feedback add --classification contract_change` → 이슈 본문 갱신(`issueops-sync-issue`) →
`feedback mark-issue-updated` → 새 기준으로 `intent record` → 게이트 원장 수정 → `decision add`로 바뀐
계획 기록(추적 `plan.md`는 승인 시점 사본이라 고치지 않는다) → 구현·검증 뒤
`feedback resolve`.
구현 진입 뒤 `artifact/`는 승인 사본이라 고치지 않는다. 진입 전 revise면 연결한 계획을 고치고 다시 리뷰한다. `regress`는 plan·compatibility-review에서만 되므로 구현 중에는 쓰지 않는다.

## 적용한 문서 규칙 보고

구현에 적용한 문서 규칙은 verified-execution report의 `## 적용한 문서 규칙` 절에 규칙마다
`| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |` 한 행으로 적는다. 게이트 원장 EVIDENCE는
`gates check`가 명령 출력으로 채우므로 여기에 쓰지 않는다. 읽었지만 적용할 규칙이 없으면
그 절에 대조한 문서 목록을 적는다. 코드를 바꾸기 직전에 그 변경을 정한 문서 규칙을 같은 절에 한 행
추가한다.

## link-plan 연결

prepare는 계획을 워크트리의 artifact/plan.md로 풀어 두지만 `plan_path`는 채우지 않는다. `next`의
`missing`에 `plan_path`가 있으면 `next_command`(봉인된 계획 경로가 채워진 `link-plan`)를 실행한다.
계획에 3단계의 네 필수 절(`## 적용되는 결정과 주의사항`, `## 재사용하는 기존 구현`,
`## 성능 영향`, `## 하위 호환성과 side effect`)이 없으면 연결이 거부된다.
