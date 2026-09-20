# IssueOps 병렬 세션 인계 도그푸딩 A 보고서

## 범위와 기준점

- Lifecycle: `io-75f917ecd7a1`
- Issue: `https://github.com/m16khb-org/issueops/issues/509`
- Branch: `509-installed-cycle-dogfood-a`
- Canonical worktree: `/Users/m16khb/Workspace/issueops.worktrees/509-installed-cycle-dogfood-a`
- 봉인된 base와 최초 HEAD: `f441a2aa432a51868c855741a8d0ad54f6ff3385`
- Plan SHA-256: `7eec1748b91fb066046734f44c0946c2d819e2adf52a6e1ea982424cac95043f`
- 갱신 인계물 SHA-256: `894e56755e3d48829c163710414f42362e2a4061fe0422581f7eb8c72cd8fd1f`

이 사이클은 설치된 IssueOps의 실제 direct lease 인계와 두 독립 native session의 동시 활성 상태를 검증한다. 런타임 코드, API, MCP schema, record schema는 변경하지 않으며 추적 대상은 이 보고서와 같은 이슈의 `gates.md`뿐이다.

## 설치본 identity

갱신 인계물의 `installed_identity`와 로컬 관측을 다음과 같이 대조했다.

- Source HEAD: `d746ee7c04b58dd2fc2d01661823ae27b73d9195`
- 설치 바이너리: `/Users/m16khb/Workspace/issueops/bin/issueops`
- 바이너리 SHA-256: `6e5cbb0e2e54631b297d8cb55d5736b1d2f844c7d6531b441f903d084c6c96dd`
- Source cwd에서 읽은 `inspect.issueops_root`: `/Users/m16khb/Workspace/issueops`
- 최종 update transition: `9d3e29a2be3377f66edfdf5d1e754db0`, `committed=true`
- Host activation: Codex, Claude, Omo, agy의 `hosts[].ok`가 모두 `true`
- Activation receipt: `native activation receipt sealed after strict Codex/Claude/Omo MCP and lifecycle readback`

CLI 증거는 `shasum -a 256`, source checkout의 `git rev-parse HEAD`, 설치 바이너리의 `inspect --json`, `/tmp/issueops-dogfood-fixes/handoff-fix-update.json` readback이다.

연결된 `daemon_status` MCP는 별도로 한 번 호출했다. 응답은 `ok=true`, `running=true`, `reachable=true`, `identity_verified=true`였고, instance executable은 같은 설치 바이너리이며 `instance.build_sha`도 `6e5cbb0e2e54631b297d8cb55d5736b1d2f844c7d6531b441f903d084c6c96dd`였다. 따라서 MCP daemon identity와 CLI에서 계산한 바이너리 해시가 일치했다.

## 실제 인계와 병렬 관측

generation 1의 released direct lease에 대해 설치 바이너리가 렌더한 `execution replace --preview` 명령부터 정확히 따랐다. inventory fingerprint를 포함한 reseed가 generation 2를 만들었고, `execution claim --claim-current-token` 뒤 현재 Codex actor가 `active(self)`가 되었다.

- 현재 actor host: `codex`
- 현재 session ID: `01a0bf27-1dc8-7631-918c-2c619b1cc48b`
- 현재 session process: PID `50921`, start `2026-09-20T14:10:06Z`
- Claim generation: `2`
- Active evidence: `/tmp/issueops-dogfood-fixes/cycle/active-a.json`

sender가 발행한 `/tmp/issueops-dogfood-fixes/cycle/parallel-observed.json`을 5초 간격의 bounded wait 안에서 읽었다. 관측 구간은 `2026-09-20T14:35:29.316369Z`부터 `2026-09-20T14:35:39.357669Z`까지다.

- A: `io-75f917ecd7a1`, generation 2, session `01a0bf27-1dc8-7631-918c-2c619b1cc48b`, PID `50921`
- B: `io-73e95250e1ad`, generation 2, session `01a0bf26-3124-7ad3-ba91-852c646ce0f4`, PID `49354`
- 두 snapshot 모두 lease가 `active`, `pending=null`, `process_verified=true`였다.
- ID, branch, worktree, session ID, PID는 두 cycle 사이에서 각각 달랐다.
- 이전 owner의 mutation은 두 cycle 모두 exit code 1과 `IssueOps execution mutation requires the current write lease holder`로 거부됐다.
- A의 stale-owner 검사 전후 record SHA-256은 모두 `b2d6c189a6bcc59c6158fc23da6b8daf99adbcce7e908f7499a8ec926bcd6224`였다.
- B의 stale-owner 검사 전후 record SHA-256은 모두 `67124044d8c002e5fd3d32fd44fee8b0a6bbb2fc5c53894ac488d3a5c6d9636f`였다.

이 증거는 두 native owner가 동시에 활성 상태였고, 이전 owner가 durable state를 바꾸지 못했음을 확인한다.

## 이전 source 수정 검증과 이번 cycle의 구분

다음 파일은 이번 cycle을 시작하기 전에 source 수정과 설치본을 검증한 증거다. 이 결과를 이번 lifecycle의 PR 발행이나 completion 증거로 해석하지 않는다.

- `/tmp/issueops-dogfood-fixes/self-verify-final.json`: 26/26 step 통과, minimum goal score 100, full risk QA에 race와 vet 포함
- `/tmp/issueops-dogfood-fixes/live-final.json`: Codex, Claude, Omo 각각 3건씩 총 9/9 live case 완료
- `/tmp/issueops-dogfood-fixes/update.json`: transition `1500796940cc793d7935a20c189d788e`, committed update와 host activation
- `/tmp/issueops-dogfood-fixes/start-fix-update.json`: transition `9ebb694c3d5f47f8b111320d45b35866`, branchless start 수정 이후 committed update
- `/tmp/issueops-dogfood-fixes/handoff-fix-self-verify.json`: 현재 source revision에서 26/26 step 통과, minimum goal score 100
- `/tmp/issueops-dogfood-fixes/handoff-fix-update.json`: transition `9d3e29a2be3377f66edfdf5d1e754db0`, 현재 설치 identity를 봉인한 committed update

이번 cycle의 고유 증거는 plan/material digest 대조, generation 1에서 2로의 실제 replace/claim, sender가 관찰한 병렬 active 구간, stale-owner 거부, 이 이슈의 gate ledger와 단계별 lifecycle receipt다.

## 범위 보정과 제한

`execution sync-base --preview`는 `main`이 `d746ee7c04b58dd2fc2d01661823ae27b73d9195`로 전진해 `merge_needed=true`라고 보고했다. 갱신 인계물은 original base `f441a2aa432a51868c855741a8d0ad54f6ff3385`와 두 문서만 허용하는 범위를 명시적으로 보존한다. Base merge를 적용하면 source 수정 파일이 이 lane의 diff에 포함되므로 적용하지 않았다.

Gate 초기화의 첫 시도에서는 G4 Python 식의 집합 합집합 연산자 `|`를 ledger 구분자로 해석해 파일을 만들지 않고 거부했다. 한 번의 허용된 문서 workflow 보정으로 같은 완전 일치 판정을 `set.union(...)`으로 표현했다. G4는 tracked diff와 untracked 파일의 합집합이 허용된 두 경로와 정확히 같아야 통과하며 substring 비교를 사용하지 않는다.

이번 변경은 보고서 전용이므로 새 runtime 코드나 테스트를 추가하지 않았다. 전체 race/vet와 9-case host 검증은 위의 이전 source 증거로만 명시했고, 실제 cycle 판단은 G1-G4와 lifecycle readback에 둔다. Merge, issue close, worktree·branch 삭제, 다른 lifecycle mutation은 이 보고서 범위에 없다.

## 품질과 성능 측정

- 정리 전 inventory는 Markdown 2개, 총 98줄과 9,560바이트였고 source file은 0개였다.
- 정리 후 inventory는 같은 Markdown 2개, 총 106줄이며 source file은 0개다.
- Code SNR, cyclomatic entropy, redundancy, channel overhead는 측정할 source 입력이 없어 N/A다. 0을 품질 점수로 꾸미지 않고 no-input guard를 적용했다.
- Runtime hot path 변경 파일과 executable artifact 변경은 각각 0개다. 실행 코드가 바뀌지 않았으므로 runtime benchmark는 적용 대상이 아니다.
- Durable side effect는 generation 2 lease와 lifecycle 단계 기록, 두 추적 문서, 승인 범위의 draft PR 및 completion receipt다. Merge와 cleanup side effect는 없다.

## Verified Execution evidence

- Success criteria: 설치 바이너리 identity 일치, genuine actor의 generation 2 인수, 두 독립 active owner 관측, stale-owner mutation 거부와 record 불변, 정확한 두 파일 변경 경계
- Evidence artifact: `/tmp/issueops-dogfood-fixes/cycle/refreshed-handoff-a.json`, `active-a.json`, `parallel-observed.json`, `.issueops/issues/509/gates.md`
- Cleanup receipt: runtime 또는 임시 service를 새로 띄우지 않았으며 canonical worktree와 lease는 승인된 lifecycle 완료를 위해 보존한다.
- Verification mode: report-only 위험도에 맞춘 proportionate mode와 실제 lifecycle/parallel observation
- Skipped checks: 이번 cycle에서 runtime 코드가 바뀌지 않아 새 full benchmark와 새 runtime test를 반복하지 않았다.

Draft PR URL과 execution completion receipt는 최종 HEAD를 바꾸지 않는 별도 결과 영수증 `/tmp/issueops-dogfood-fixes/cycle/result-a.json`이 소유한다.
