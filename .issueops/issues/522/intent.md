# 요청자 의도 계약

- lifecycle: io-814b092d660e
- issue: https://github.com/m16khb-org/issueops/issues/522
- intent_class: standard

## 원문 요청
3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘

## 해석
리뷰 effort 개선 항목을 독립 사이클로 진행하여 owner가 구현 검증 시점의 next.review 모델과 effort를 따르도록 통일한다.

## 성공 기준
- 실제 생성 owner prompt가 리뷰 직전 exact ID의 next.review를 읽고 review skill 정책을 따르며 docs-only/default/contract/schema-auth와 라운드 상승을 구분한다. 기본값과 명시적 override 및 기존 봉인 자료는 보존하고 회귀 테스트로 검증한다.

## 비목표
- 품질 파일 범위 및 최신 상태 표시 수정은 다른 독립 사이클이 맡는다.

## 제약
- 승인 범위는 독립 이슈, 작업 공간, 구현과 검증, commit/push, Draft PR, execution complete까지다. merge와 cleanup은 제외한다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
