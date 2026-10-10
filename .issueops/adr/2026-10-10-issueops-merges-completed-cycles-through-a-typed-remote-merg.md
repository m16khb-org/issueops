---
name: 2026-10-10-issueops-merges-completed-cycles-through-a-typed-remote-merg
description: Accepted decision record with rationale, alternatives, and consequences.
---

# IssueOps merges completed cycles through a typed remote merge-pr

- Date: 2026-10-10
- Kind: `adr`
- Source: issueops-merge skill
- Summary: 메인 세션의 issueops-merge 스킬이 완료된 사이클의 PR/MR을 typed 명령 remote merge-pr로 머지하고 cleanup에 넘긴다.
- Context: 워크트리 세션은 execution complete까지만 맡고, 머지는 개발자가 PR을 확인한 뒤 메인 세션에서 gh pr merge를 직접 호출하고 있었다. 이 경로는 미리보기·확인·다시 읽기 계약과 head 고정 밖에 있었고, --delete-branch는 다른 워크트리가 base를 체크아웃하면 실패했다.
- Decision: remote merge-pr는 phase done, released lease, completion이 있는 record만 받는다. PR/MR을 한 번 읽어 draft·head·체크·머지 가능 여부를 판정하고, 차단 사유(checks_failing, checks_pending, head_mismatch, merge_conflict, merge_blocked, pr_not_open)가 하나라도 있으면 --confirm을 거부한다. 기본 방식은 squash다. 머지는 completion final_head에 고정하며 branch 삭제·admin 우회·auto-merge는 쓰지 않는다. 원격 브랜치는 cleanup remote-branch가 지운다. 체크 실패(코드 결함)와 head 불일치는 execution replace --reseed로 사이클을 다시 열고, 충돌은 execution sync-base 뒤 같은 경로를 밟는다.
- Consequences: 머지 단계는 record를 쓰지 않으며 cleanup이 provider readback으로 머지를 다시 확인한다. GitLab에서는 프로젝트가 merge commit과 fast-forward를 정하므로 rebase 방식을 받지 않는다. merge queue나 지연 반영으로 readback이 open이면 merge was not verified로 보고하고 재시도 대신 미리보기를 다시 읽는다. issueops list는 issue_url을 노출해 이슈 번호로 사이클을 찾는다.
