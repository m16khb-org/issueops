# 검토 기록과 되돌림 세부 절차

`SKILL.md`의 `## 아키텍처 작성과 스테이징`과 `## 검토 루프`가 가리키는 세부 절차다. 본문은 `regress`로
돌아가는 조건을 소유하고, 이 문서는 되돌린 뒤의 순서를 소유한다.

## 스테이징한 아키텍처 문서 내리기

스테이징한 문서는 `artifact unstage --name architecture`로 내리거나 같은 이름으로 다시 올려
교체한다. 승인한 뒤 문서를 고치면 승인이 `architecture_review_stale`이 되므로 다시 검토한다.
아키텍처 검토의 판정별 처리는 [승인 기록](architecture-review-record.md)에 있다.

## 계획 검토 stop 판정 뒤의 되돌림

`regress`는 사이클을 grill로 되돌리고 아키텍처 승인도 지운다. grill에서 다시 조사한다. 요구가 바뀌면
`feedback add --classification contract_change` → 이슈 본문 갱신(`issueops-sync-issue`) →
`feedback mark-issue-updated` → 새 기준으로 `intent record` 순서로 반영한다. 그 뒤
`phase --to plan`으로 올라와 아키텍처와 계획을 다시 쓰고 다시 검토한다. 판정 뒤 계획을 고치면
`devils_advocate_review_stale`이 되어 `next`가 plan.review로 돌아간다.

## plan.handoff에서 사용자가 계획을 바꾸라고 할 때

`execution prepare`로 가지 않는다. 바뀐 내용이 아키텍처에 닿으면 아키텍처 문서를 다시 스테이징하고
리뷰를 다시 기록한다. 그다음 계획을 다시 스테이징하고 계획 검토를 다시 기록한다. 이슈 본문의 사실이
바뀌면 `contract_change`도 기록한다.
