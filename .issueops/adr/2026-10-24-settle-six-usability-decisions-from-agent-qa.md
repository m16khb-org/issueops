---
name: 2026-10-24-settle-six-usability-decisions-from-agent-qa
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Settle six usability decisions from agent QA

- Date: 2026-10-24
- Kind: `adr`
- Source: 사용자 지시("결정이 필요한 사항은 네가 조사해서 최선의 방법을 결정하고 기록하고 진행해"). 자율 연구 루프에서 Codex·Claude·omp·Omo·opencode 에이전트가 샌드박스 provider(`gh`·`ssh` 대역)로 이슈부터 머지·정리까지 돌리며 반복해서 걸린 지점이다.
- Summary: 여섯 가지를 정했다. Omo 5.1의 bun 호스트는 명령줄로 식별해 받는다. MCP에는 사이클 목록 도구를 더하지 않는다. sync-base apply는 사용자 요청이나 `merge_conflict`일 때만 한다. child 사이클은 자기 브랜치 링크를 스스로 검증한다. `await-link`는 읽기 전용으로 두고 정확한 `--link-verified` 기록 명령을 돌려준다. 검증 보고서 파일은 봉인 fingerprint에서 뺀다.
- Context: 아래 여섯은 코드나 스킬이 서로 다른 답을 주던 지점이다. 매번 에이전트가 임의로 판단했고, 일부는 신원·보안 계약과 맞닿아 있어 별도 결정으로 미뤄 두었다.
- Decision:
  1. **Omo 5.1 `whoami`.** Omo의 RPC 호스트는 이제 `/Users/u/.bun/bin/bun .../@code-yeongyu/senpi/dist/bundle/cli.js`로 실행된다(2026-10-24 실측: 도구 셸의 부모 프로세스). OS가 보고하는 실행 파일 이름이 omp와 같은 `bun`이라, 실행 파일만으로는 두 호스트를 가를 수 없다. host가 `omo`일 때 `bun` 후보는 그 PID의 명령줄에 `/senpi/`나 `/omo-ai/`가 있을 때만 받는다(`nativeHostProcess`, `ps -o args=` 한 번). omp 세션의 bun은 명령줄이 omp 스크립트이므로 Omo로 받지 않는다. 기존 테스트 `TestReusableNativeProcessAncestryDoesNotUseBunForAnotherHost`의 의도(다른 호스트의 bun을 Omo로 받지 않는다)는 그대로 지킨다. herdr가 띄운 도구 셸처럼 Omo 조상이 아예 없는 환경은 계속 `unavailable`로 실패한다.
  2. **MCP 사이클 탐색.** MCP 전용 목록이나 `next` 도구는 더하지 않는다. 통합 표면의 1차는 CLI이고(루트 AGENTS.md 결정 표), 다섯 호스트 모두 셸을 쓸 수 있다. 도구 하나는 모든 세션의 `tools/list`를 늘리는 데 비해, MCP만 쓰는 경우는 QA가 일부러 만든 상황뿐이었다. id 오류에는 이미 `IDFormatHint`가 `issueops next --json`·`list`를 안내한다.
  3. **verify 대 sync-base apply.** verify 단계는 `base_advanced` 경고만으로 스스로 apply하지 않는다. apply는 봉인을 통째로 stale로 만들기 때문이다. 예외는 사용자가 base 반영을 명시적으로 요청했거나 `remote merge-pr`가 `merge_conflict`를 보고한 경우다. 그때는 apply한 뒤 `next`의 재봉인 순서를 따른다.
  4. **child의 `branch_link_verified`.** 물려받지 않는다. child 이슈와 child 브랜치의 링크는 parent와 별개의 provider 사실이고, child는 자기 PR(base는 parent 브랜치)을 완료해야 `done`이 된다. 샌드박스 provider로 돌린 opus 실행이 child를 이 순서로 끝까지 진행했다.
  5. **`--link-verified` 근거.** `await-link`는 읽기 전용 관측기로 둔다. lease 없이 coordinator도 실행할 수 있어야 하고, 원래 설계 주석도 그렇다. 대신 `linked: true`일 때의 `next_command`를 봉인값이 채워진 정확한 `branch prepare ... --link-verified $RECORD_ACTOR_FLAGS --json`로 돌려준다(`BranchLinkVerifiedCommand`). 플래그 자체에 실시간 readback을 강제하지는 않는다. provider가 없는 fixture, GitLab, 오프라인 벤치가 막히기 때문이다.
  6. **봉인과 보고서의 stale 반복.** `.issueops/issues/<n>/verified-execution-report.md`는 verify·docs·complete가 정리 봉인 뒤에 쓰는 과정 기록이므로 변경 fingerprint(`fingerprintPaths`)에서 뺀다. 정리·구현 리뷰 봉인과 readiness 관측이 같은 함수를 쓰므로 둘의 판단이 갈리지 않는다. 다른 이름의 파일과 코드 변경은 그대로 fingerprint에 남는다. completion의 `ChangedPathsSince`는 이미 보고서를 따로 다룬다.
- Consequences: 지금의 Omo에서 `whoami`, `claim`, MCP `authorize`가 동작한다(2026-10-24 실측: `host: omo`, receipt는 bun PID). Omo receipt를 고를 때 bun 후보마다 `ps`를 한 번 더 실행한다. 보고서 파일명이 표준화되어 skill(complete·verify)이 그 경로를 지정한다. 업그레이드 전에 보고서를 포함해 찍은 봉인은 한 번 stale로 보일 수 있으며, 다시 봉인하면 된다(fail-safe). sync-base와 child 규칙은 스킬에만 적힌 정책이고 코드 게이트는 없다.
- Evidence:
  - cmd/issueops/issueopscli/executioncmd/execution.go `nativeHostProcess`, execution_whoami_test.go (senpi bun 수용·omp bun 거부)
  - internal/adapter/issueops/implementation/evidence.go `fingerprintPaths`, evidence_test.go `TestFingerprintIgnoresTheVerificationReportOnly`
  - internal/domain/issueops/branch_await_link.go `BranchLinkVerifiedCommand`, internal/application/issueopsbranch/await_link_test.go
  - skills/issueops-verify, issueops-complete, issueops-prepare, issueops-implement/references/child-delegation.md
  - 실측: `omo -p`로 띄운 세션의 `issueops execution whoami --json`이 `host: omo`, `executable: /Users/m16khb/.bun/bin/bun`를 돌려줌(2026-10-24)
- Alternatives / rejected options:
  - Omo에서 모든 `bun`을 받기: omp 세션의 bun을 Omo로 잘못 고를 수 있다
  - 기존 receipt에 argv 필드 추가: 저장된 receipt 계약과 liveness 비교가 바뀐다. 선택 시점에만 읽는 편이 좁다
  - MCP `issueops_list`/`next` 도구 추가: 모든 세션의 tools/list 비용에 비해 실제 수요가 없다
  - `await-link`가 기록까지 하기: 읽기 전용 관측기 계약과 actor 권한 경계가 깨진다
  - `--link-verified`에 실시간 readback 강제: provider 없는 fixture와 GitLab, 오프라인 벤치가 막힌다
  - `.issueops/issues/<n>/` 전체를 fingerprint에서 빼기: 같은 디렉터리의 문서·게이트 원장 변경까지 리뷰에서 빠진다
