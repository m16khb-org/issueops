---
name: 2026-10-14-completion-does-not-check-the-head-was-verified
description: Caution record for a solved false case or recurring risk.
---

# Completion does not check that the sealed head is the verified one

- Date: 2026-10-14
- Kind: `caution`
- Source: procedure autoresearch session (complete-stage diagnosis)
- Summary: `execution complete`는 `--final-head`가 canonical worktree의 현재 HEAD와 같은지만 검사한다. 그 head가 검증 단계가 봉인한 변경 집합인지는 보지 않는다. 그래서 PR을 만든 뒤 코드를 한 줄 고쳐 커밋하고 완료하면, 검증받지 않은 head가 completion head로 봉인된다. 머지 단계의 `head_mismatch`는 completion head와 비교하므로 이 경우를 잡지 못한다.
- Context:
  - `internal/application/issueopscompletion/complete.go`의 `completeRecord`는 phase, artifact, lease, `ValidateFinalHead`, 보고서 경로만 확인한다.
  - 변경 fingerprint(`internal/adapter/issueops/implementation/evidence.go` `ChangeFingerprint`)는 base 이후 바뀐 모든 경로의 내용이다. verified-execution report도 여기에 들어간다. 그래서 보고서를 커밋하면 `implementation_review_stale`·`project_docs_review_stale`이 함께 뜨고, 코어가 completion에서 stale을 거부 조건으로 쓰면 정상 흐름도 막힌다.
  - 진단에서 Sonnet 5.5 완료 세션은 "PR 뒤 코드 한 줄 수정 + readiness `_stale`" 상황 6회 중 4회를 그대로 완료했다.
- Resolution:
  - `skills/issueops-complete/SKILL.md` 증거 순서 0단계: 보고서를 쓰기 **전에** `issueops next`를 읽는다. 이때 `_stale` 항목이 있거나 PR 뒤 코드 커밋이 있으면 보고서 탓일 수 없으므로 complete하지 않는다. 정리·문서·검증부터 새 head로 다시 받는다. 진단 6회 기준 2/6→5/6이었고, 변경이 없는 대조군은 그대로 완료했다.
  - 코어 검사(같은 날 추가): pr 진입(그리고 같은 strict 게이트를 다시 통과하는 pr 재진입)이 canonical HEAD를 `pr_head`로 봉인한다. `execution complete`는 `git diff --name-only pr_head final_head`에서 검증 보고서를 뺀 경로가 하나라도 있으면 거부한다. fingerprint 대신 head를 쓴 이유는 pr 게이트가 이미 깨끗한 워크트리와 새 봉인을 요구하므로 그 head가 곧 검증된 head이고, 보고서 경로만 빼면 되기 때문이다. 결정은 `.issueops/adr/2026-10-14-completion-checks-the-verified-pr-head.md`.
- Evidence: 스킬 커밋 d6cc69c7의 측정 기록(측정 밖 진단 시나리오 complete-stale·complete-clean, 각 6회). 코어 검사는 `TestCompleteRefusesHeadThatChangedCodeAfterPREntry`, `TestEnvironmentChangedPathsSinceLeavesOutOnlyTheReport`, `TestCLIIssueOpsPRReentryResealsPRHeadOnlyAfterTheStrictGate`.
