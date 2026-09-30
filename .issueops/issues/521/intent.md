# 요청자 의도 계약

- lifecycle: io-dd786760d2a9
- issue: https://github.com/m16khb-org/issueops/issues/521
- intent_class: standard

## 원문 요청
3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘

## 해석
품질 지표가 임시 증거와 재현 스크립트를 제품 코드로 집계하지 않도록 분석 대상을 바로잡고 독립 IssueOps 세션으로 구현한다.

## 성공 기준
- ignored evidence/temp 및 생성 Go 코드는 제외하고 정상 tracked/untracked 제품 코드는 포함한다. Git 없는 디렉터리 동작을 정의하고 기존 품질 응답 및 실패 표시 계약을 유지한다. focused RED/GREEN과 실제 scanner 표면으로 검증하고 draft PR과 execution complete까지 수행한다.

## 비목표
- 리뷰 effort 정책, 최신 self-verify 선택, merge 및 cleanup은 범위 밖이다.

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
