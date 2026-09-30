# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/513
- intent_class: architecture

## 원문 요청
issueops의 아키텍처를 개선할 수 있는게 있을까? 이슈와 pr을 사람이 이해하기 쉽게 만드는 걸 더 강화했으면 좋겠어 /explain 같은 설명도 좋고 /fluent-korean 같은 것도 확인해보고 잘만들어진 이슈들도 좀 참고해보고
이슈와 pr은 사람이 읽는거야 쓸데없는 해시 값이런거 넣지말고 사람이 읽고 전체적인 흐름을 파악하고 이해하기 쉬운 내용들을 넣어야해
하네스가 사람이 읽을수 있게 잘정리된 이슈와 pr을 만들고 구현을 위한 자료는 .issueops 하위에 가지고 있는게 의도야
승인, 커밋하고 이슈옵스 사이클 시작해

## 해석
issueops가 게시하는 이슈·PR 본문을 사람이 읽는 문서로 바꾼다. 승인된 설계 docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md를 구현한다. (1) 요약이 먼저 오는 짧은 본문 계약(PR 필수 4절, 구현 이슈 필수 5절)을 artifacttemplate 한 곳에서 정의한다. (2) 해시, 커밋 SHA, 라벨 점수, 정리 감사, plan 원문 같은 기계용 데이터는 본문에서 빼고 .issueops/issues/<n>/와 record로 옮긴다. (3) 가독성 검사(critical 4개, warning 9개)를 issueops remote의 게시·동기화 명령 안에서 항상 실행한다. (4) 완료 구간은 사람이 쓴 진행 결과로, 계획 검토 구간은 흐름 한 줄로 바꾼다. (5) explain 구조, fluent-korean, 용어 변환표, 공개 모범 사례를 작성 지침 한 문서로 모은다. PR 하나에서 두 작업 묶음(사람이 쓰는 본문, 하네스가 붙이는 구간과 구현 자료)을 순서대로 구현하고 묶음마다 커밋을 나눈다.

## 성공 기준
- 새 계약으로 게시한 이슈와 PR의 첫 절은 ## 요약이다.
- PR의 필수 절은 요약, 변경 내용, 확인한 것, 리뷰 포인트 4개이고, 구현 이슈의 필수 절은 요약, 배경, 완료 기준, 범위, 검증 5개다.
- remote create-issue, create-child, create-pr, sync-issue, sync-pr, reflect-completion은 가독성 검사를 항상 실행하고 critical이 있으면 --confirm을 거부한다.
- 완료 구간과 계획 검토 구간에는 64자리 hex, 커밋 SHA 전문, 로컬 절대 경로, plan·spec 원문이 없다.
- plan, spec, intent, gates 같은 구현 자료는 .issueops/issues/<n>/에 커밋되고 이슈 본문에 복사되지 않는다.
- go test ./... -count=1, 응답 계약 golden, 바뀐 스킬의 validate-skill.py와 verify-skill-shell.py가 통과한다.

## 비목표
- 문장의 자연스러움이나 이해 여부를 기계로 판정하지 않는다.
- 독자 검토를 게시 조건으로 묶지 않는다.
- provider MCP 도구로 직접 게시하는 것을 기술적으로 막지 않는다.
- 이미 게시된 이슈와 PR을 한꺼번에 고치지 않는다.
- record 스키마, provider 어댑터 API, lease·generation·CAS 절차를 바꾸지 않는다.

## 제약
- 이 저장소는 공개 저장소이므로 사내 비공개 저장소의 문장과 수치를 작성 지침과 테스트 fixture에 쓰지 않는다.
- record 디코더가 모르는 필드를 거부하므로(DisallowUnknownFields) record 필드를 추가하지 않는다.
- 구현 자료는 .issueops/issues/<n>/ 아래에 둔다(2026-09-24 사용자 의도).

## 모호함
- resolved: plan·spec 원문의 위치는 .issueops/issues/<n>/이며 PR 커밋에 포함한다(사용자 의도와 설계 4절).
- resolved: sync의 템플릿 종류는 record에 저장하지 않고 제안 본문의 절 제목으로 추론한다(설계 5절).
- deferred: 독자 검토를 게시 조건으로 묶을지는 새 형식의 효과를 측정한 뒤 정한다.
- resolved: 작업 분할은 child 위임 대신 PR 하나, 작업 묶음별 커밋으로 한다(계획 리뷰 1라운드 D4, 2026-09-24 사용자 선택).

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
