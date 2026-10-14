# 2026-10-14 — Completion checks that nothing but the report changed since the verified pr head

← [ADR index](../ADR.md)

**결정:** pr 단계에 들어갈 때 canonical worktree HEAD를 record의 `pr_head`에 봉인한다. `execution complete`는 `git diff --name-only pr_head final_head`에서 검증 보고서 경로를 뺀 경로가 하나라도 있으면 거부한다. 이미 pr인 사이클에서 `issueops phase --to pr`을 다시 실행하면 첫 진입과 같은 strict pr 게이트(upstream fetch, gate 원장, 깨끗한 워크트리, 새 정리·문서·구현 리뷰 봉인)를 다시 통과해야 하고, 통과하면 `pr_head`를 새 HEAD로 바꾼다. 거부되면 `pr_head`는 그대로다. reseed는 `pr_head`를 지운다. `pr_head`가 없는 기존 record는 검사를 건너뛴다.

**근거:** 기존 completion은 `--final-head`가 현재 HEAD와 같은지만 봤다. 그래서 PR 뒤에 코드를 고쳐 커밋해도 검증받지 않은 head가 completion head로 봉인됐다([caution](../cautions/2026-10-14-completion-does-not-check-the-head-was-verified.md)). 스킬 규칙만으로는 진단 6회 중 5회만 멈췄다. 변경 fingerprint 비교도 검토했지만 fingerprint에는 보고서 경로가 들어가서, 보고서를 커밋하는 정상 흐름에서도 `_stale`이 뜬다. pr 게이트는 이미 깨끗하고 upstream과 맞춰진 워크트리와 새 봉인을 요구하므로, 그 순간의 HEAD가 곧 검증된 head다. 보고서 경로 하나만 빼면 되므로 fingerprint 계약은 바꾸지 않는다.

**대안과 기각 이유:**

- fingerprint에서 보고서 경로를 빼고 completion에서 stale을 거부: readiness·리뷰 봉인 계약이 함께 바뀌고, 보고서 경로를 fingerprint 계산 시점에 알아야 한다.
- 코드 변경 뒤 `regress`: grill까지 되돌아가 계획부터 다시 하므로 작은 PR 뒤 수정에 비해 너무 무겁다. 같은 단계 재진입이 봉인을 새로 받는 방식은 `phase --to ai-slop-clean` 재봉인에 이미 있다.

**영향:**

- PR 뒤 수정은 `phase --to ai-slop-clean` → `next`가 가리키는 리뷰·검증 재기록 → `phase --to pr` 순서로 다시 봉인한다. 이전에는 pr에서 pr로 가는 호출이 아무것도 하지 않았다.
- pr 진입 뒤 base를 손으로 병합하면 병합된 경로도 diff에 잡혀 completion이 거부한다. 병합 결과도 검증받지 않은 head이므로 같은 재봉인 경로를 밟는다. post-completion `sync-base`는 completion 뒤에 실행되므로 영향이 없다.
- record에 선택 필드 `pr_head`가 생겼다. 응답 계약 golden에 반영했다.

**검증:** `TestCompleteRefusesHeadThatChangedCodeAfterPREntry`, `TestEnvironmentChangedPathsSinceLeavesOutOnlyTheReport`(실제 git 저장소, macOS `/var` 심볼릭 링크 포함), `TestCLIIssueOpsPRReentryResealsPRHeadOnlyAfterTheStrictGate`(실제 worktree에서 pr 진입 봉인, 낡은 봉인일 때 재진입 거부, 재봉인 뒤 갱신), reseed 정리 테스트.
