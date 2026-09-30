# 요청자 의도 계약

- lifecycle: io-14e119b8fce9
- issue: https://github.com/m16khb-org/issueops/issues/523
- intent_class: standard

## 원문 요청
3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘

## 해석
세 개선 중 상태 요약의 최신 자기 검증 기록 선택을 독립 이슈로 구현·검증하고 새 Codex 세션에 인계한다.

## 성공 기준
- 생성 시각 기준 최신 적격 실행을 순서 독립적으로 선택하고 최근 실패를 숨기지 않는다. 후보 자료 제외, 시각 오류·동률, 기존 응답 계약, 조회 무변이 검증을 통과한다.

## 비목표
- 품질 scanner 및 reviewer effort 변경

## 제약
- 독립 canonical worktree에서만 구현. Draft PR 발행과 execution complete까지 승인. merge 및 cleanup 제외.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
