---
name: 2026-09-24-sync-base-stale
description: Caution record for a solved false case or recurring risk.
---

# 구현 진입 sync-base 뒤 계획을 고치면 계획 리뷰가 stale이 된다

- Date: 2026-09-24
- Kind: `caution`
- Source: issueops-docs; #517 io-312a238fdcfe 구현 진입
- Summary: issueops-implement는 4단계 진입의 sync-base apply 뒤 그 사실을 계획의 `## 하위 호환성과 side effect` 절에 적으라고 하지만, 계획 리뷰 판정은 계획 sha256에 묶여 있어 그 편집이 devils_advocate_review_stale을 만든다. 계획은 그대로 두고 compatibility review의 side effect와 verified-execution report에 적는다.
- Context: 2026-09-24 #517 구현 세션은 claim 뒤 `execution sync-base --preview`에서 merge_needed=true, conflict 0을 보고 origin/main 844e69c8(#513 머지)을 merge commit 5a49595c로 반영했다. skills/issueops-implement/SKILL.md의 "base가 앞서 나갔는지 본다" 절은 이때 계획의 `## 하위 호환성과 side effect` 절에 그 사실을 적으라고 한다. 그런데 이 사이클의 계획 리뷰 3라운드 pass는 reviewed_plan_digest=310a3ec6…(계획 sha256)에 묶여 있었고, issueOpsDevilsAdvocateReviewMissing(internal/adapter/issueops/issueops_readiness.go)은 링크된 계획의 digest가 판정 digest와 다르면 devils_advocate_review_stale을 돌려준다. 계획을 한 줄이라도 고치면 계획 리뷰를 다시 받아야 하고, 이미 3라운드를 쓴 사이클은 revise 상한 때문에 탈출 경로(stop, regress, waive)만 남는다. sync-base 반영은 봉인된 BaseSHA 기준 diff에 base 쪽 변경을 넣기 때문에, docs-only 사이클이라도 next의 review.tier가 contract로 올라간다(#517에서 116파일, .go 69개가 diff에 들어왔다).
- Resolution: 계획 파일은 고치지 않는다. sync-base 사실(base oid, merge commit, conflict 수, diff와 review tier에 주는 영향)을 `issueops compatibility review --side-effect`와 verified-execution report의 base_sync 항목에 적는다. link-plan은 봉인된 계획 그대로 실행한다. 스킬 문구를 이 순서에 맞게 고치는 일은 별도 이슈로 다룬다.
- Evidence:
  - skills/issueops-implement/SKILL.md "base가 앞서 나갔는지 본다" 절
  - internal/adapter/issueops/issueops_readiness.go issueOpsDevilsAdvocateReviewMissing: 링크된 계획 digest가 reviewed_plan_digest와 다르면 devils_advocate_review_stale
  - issueops status --id io-312a238fdcfe --json: execution.sync_base_events[0] merge_commit 5a49595c, conflict_files 0
  - issueops next --id io-312a238fdcfe --json (clean 단계): review.tier contract
  - git diff 92bbbdda --stat: 116 files, .go 69개(이 사이클 자체는 .md만 변경)
