# 원격 쓰기 단독 실행 순서

render-template 골격 → `fluent-korean` 스킬로 한국어 다듬기(issueops와 함께 설치되므로 생략하지 않는다) → secret 제거 →
threshold로 선택한 label·실제 assignee(`@me` 금지) → preview의 readability 확인 →
같은 요청에만 confirm → URL·상태·본문·metadata readback. critical은 수정 후 재preview하고,
warning은 수정하거나 이유를 기록한다. 승인된 원문/범위가 바뀌면 사용자에게 확인한다.
raw provider 쓰기로 우회하지 않는다. 모호한 write는 reconcile하며 중복 생성하지 않는다.
계약 변경 feedback은 소유 단계가 인증된 actor로 기록하고 이슈 본문을 먼저 갱신한 뒤
`feedback mark-issue-updated`를 기록한다. 댓글은 계약이 아니다.
`issueops feedback add`와 `issueops status`는 유효한 root alias다. `quality inspect`는
semantic skill runner가 아니며 `issueops skill-bench`는 구현되지 않았다.
