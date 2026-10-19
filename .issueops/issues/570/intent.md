# 요청자 의도 계약

- lifecycle: io-75f917ecd7a1
- issue: https://github.com/m16khb-org/issueops/issues/570
- intent_class: architecture

## 원문 요청
issueops를 사용할 때도 이렇게 시니어 엔지니어처럼 소프트웨어 설계하는 방식으로 설계 해줄 수 있어? / 애초에 계획 단계 앞에 설계 단계가 있어야 하지않을까?? / design이라는 워딩은 변경하면 좋겠어 / (선택 답변) 새 이름: architecture (아키텍처), 바꿀 범위: 전체

## 해석
IssueOps 계획 단계에서 아키텍처를 계획보다 먼저 정하도록 순서를 바꾼다(plan.architecture → plan.write → plan.review → plan.handoff). 아키텍처 문서는 요구사항과 규모, 데이터와 API 계약, 구조와 흐름, 병목과 장애, 절충점과 기각한 대안을 다루고, 독립 리뷰 승인이 그 문서 digest에 묶인다. 같은 변경에서 이 단계와 리뷰를 가리키는 design 용어(stage 키, CLI 명령, record 필드, 게이트 키, Go 타입, 공용 스킬 design-review)를 architecture로 바꾼다.

## 성공 기준
- issueops next가 plan 단계에서 plan.architecture → plan.write → plan.review → plan.handoff 순서를 돌려준다
- 필수 절이 빠진 아키텍처 문서는 아키텍처 리뷰 기록이 거부되고 빠진 절 제목이 오류에 나온다
- 승인 뒤 아키텍처 문서를 고치면 승인이 stale이 되어 plan.write로 넘어가지 않는다
- 현행 코드·스킬·문서·golden에 범위 안의 옛 이름(plan.design, design review 명령, design_review*, design-review 스킬)이 남지 않는다
- 임시 state와 임시 저장소에서 새 바이너리로 계획 단계를 처음부터 끝까지 실행해 순서를 확인한다

## 비목표
- 과거 기록 문서(.issueops/adr, cautions, issues, plans, research, verified-execution)와 과거 리뷰 라운드를 가리키는 코드 주석은 고치지 않는다
- database-design, ui-ux-craft 등 다른 뜻의 design은 바꾸지 않는다
- durable phase를 추가하지 않는다
- 계획 필수 절 네 개는 바꾸지 않는다
- 이전 필드 이름을 읽는 호환 경로를 두지 않는다

## 제약
- record schema_version 1 정책상 자동 변환 없음: design_review 필드가 있는 옛 record는 invalid state가 된다(현재 로컬 기록 0건)

## 모호함
- resolved: 이름은 architecture, 범위는 전체(사용자 선택)
- resolved: 새 durable phase 대신 plan 단계 안에서 순서를 바꾼다(추천안 적용, 발행 전 확인)
- deferred: 아키텍처 문서를 새 artifact 이름으로 둘지 기존 spec artifact를 재사용할지는 계획에서 정한다

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
