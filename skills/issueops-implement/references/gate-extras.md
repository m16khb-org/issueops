# 구현 단계 보충 규칙

## frontend·schema 변경

`next.review.frontend`가 true면 기존 디자인 시스템과 컴포넌트 출처·접근성·반응형·
모션 감소를 확인하고 실제 화면을 검증한다. 설치된 `ui-ux-craft`를 활용하며, 확인 결과는
report의 **`## UI 판단`**에 적는다. 이는 7단계 브라우저 QA의 INTENT이므로 비우지 않는다.
schema 변경은 실제 인덱스·카탈로그 추정 row 수와 출처를 기록한다. 운영 DB 전수
`SELECT COUNT(*)`는 금지하며 관찰 불가면 waive 근거를 남긴다.

## feedback 기록과 원격 쓰기

인증된 feedback 기록도 이 단계가 소유한다(`issueops feedback add`/`status`는 유효 alias).
계약 feedback은 이슈 본문을 먼저 갱신하고 `feedback mark-issue-updated`를 기록한다.
원격 쓰기는 preview → 동일 요청 confirm → readback이며 모호하면 reconcile한다.
원래 승인 범위 밖 commit·push·publication·merge·cleanup은 하지 않는다.
명령 존재는 usage 카탈로그/소스로 확인한다. `skill-bench`는 미구현이며
`quality inspect`는 semantic skill runner가 아니다.

## 추적 사본

- `phase --to implement`와 `phase --to ai-slop-clean`은 `.issueops/issues/<n>/`에 구현 자료의
  추적 사본(plan.md, intent.md, spec.md, plan-review.md)을 쓰고 응답 `tracked_materials`에
  경로를 보고한다. 사본은 변경 집합에 들어가므로 구현 진입 뒤 첫 커밋에 포함한다.

## API/DTO/OpenAPI 변경

- API/DTO/OpenAPI 변경은 `.issueops/OPEN_API_SPEC.md` gate를 적용한다.
  해당 문서가 없는 대상도 변경 endpoint·public error 계약을 조사하고, 제공되는 static check와
  agent review 결과를 기록한다. 관련 없는 legacy 부채로 실패시키지 않는다.
