---
name: 2026-11-03-no-enforcement-hook-for-out-of-cli-actions
description: Accepted decision record with rationale, alternatives, and consequences.
---

# No enforcement hook for out-of-CLI actions

- Date: 2026-11-03
- Kind: `adr`
- Source: 2026-11-03 hook gap survey of host transcripts
- Summary: CLI 밖 행동(직접 gh pr create·glab mr create, worktree 밖 편집, force push, next 생략)을 막는 PreToolUse·Stop enforcement hook을 다시 넣지 않는다. 실제 우회는 fork PR 같은 CLI 기능 공백에서 나왔으므로 CLI와 스킬로 메운다.
- Context: 2026-08-27 이후 Claude·Codex·omp 전사 1,459개(도구 호출 121,364건, 사이클 세션 358개)를 조사했다. CLI 게이트가 볼 수 없는 위반 중 반복되는 것은 없었다. worktree 밖 편집 0건(Edit·Write 1,069건 중), next 없이 상태 전이 0세션, 사이클 PR raw 생성 2건(둘 다 포크 head·label 권한 없음·영어 템플릿인 외부 기여), 제목 raw 수정과 raw 머지는 remote retitle(2026-10-27)·merge-pr(2026-10-10) 추가 뒤 0건이었다. Claude·Codex·omp·opencode는 pre-tool 차단을 지원하지만 2026-08-10·08-27 ADR이 기록한 비용(hot path 지연, Codex workdir 없는 payload 오판, Stop 재진입 루프)은 그대로다.
- Decision: hook은 project-doc catalog 주입만 맡는다(2026-08-27 결정 유지). 우회가 관측되면 hook을 추가하기 전에 그 우회를 만든 CLI 공백을 먼저 확인한다. remote create-pr이 표현할 수 없는 PR(포크 head, label·assignee 권한 없음, 비한국어 템플릿)은 사용자 허락, decision add --kind scope 기록, issueops-abandon 정리 순서로 처리하고 이를 issueops-create-pr 스킬에 적는다.
- Consequences: 외부 기여 사이클은 publication receipt 없이 abandon으로 끝난다. 포크 head를 create-pr이 지원하게 하는 일은 원격 쓰기 계약을 바꾸므로 별도 설계가 필요한 후속 작업이다. hook 재도입을 검토할 때는 이 조사처럼 전사에서 반복 위반을 먼저 센다.
