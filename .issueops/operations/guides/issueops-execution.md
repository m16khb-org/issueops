---
name: issueops-execution
description: IssueOps execution lifecycle, recovery, sync-base, session handoff, and planner/implementer operations.
---

# IssueOps Execution Lifecycle and Session Workflow

This guide owns the execution state machine, recovery, completion,
sync-base, the execution sequence, and the planner/implementer
operations. Provider publication and branch linkage contracts live in
[issueops-providers.md](issueops-providers.md). Canonical index:
[../../OPERATIONS.md](../../OPERATIONS.md).

## Execution Lifecycle, Recovery, and Completion

Follow `skills/issueops/references/execution.md`. `execution prepare` creates
the canonical sibling worktree and grants generation 1 to the preparing
session. A new session takes over through one chain: the current holder runs
`execution release`, the new session runs `execution replace --id ID
--expected-generation N --preview` under its own native actor, executes the
emitted `--reseed` command, then runs `execution claim --claim-current-token`.
Only the active generation holder implements, verifies, creates the draft
PR/MR, and completes from the canonical worktree. The source main worktree
remains available for unrelated cycles.

Every external mutation stores intent first. Timeout or ambiguous output is not
absence: inspect `issueops execution status` and use preview/confirm
`issueops execution reconcile`; never repeat create after a possible
mutation. Failed-holder recovery uses the ordered generation-CAS
replacement sequence and proves the prior process is quiescent before
creating a new claimable generation.

`issueops next` routes by lease state: a claimable execution returns
`execution claim --claim-current-token`, a released execution returns
`execution replace --preview`, and a revoking execution returns
`execution replace --finalize-preview`. Execute the returned generation-bound
command unchanged. Actor flags are all-or-nothing: pass either the complete
explicit `ACTOR_FLAGS` receipt or none; the actor-free form observes the
current Codex/Claude/Omo/omp/opencode session, native host process ancestry, and
canonical process cwd. Supplying only part of `ACTOR_FLAGS` is rejected.

완료 영수증이 있는 `released` 또는 `claimable` execution을 `--reseed`하면 기존 영수증은
generation, reason, reopen time과 함께 append-only `execution.completion_history`로 이동하고
현재 `completion`은 비워진다. 같은 raw-CAS commit이 phase를 `implement`로 되돌리고 현재
HEAD에 묶인 AI-slop/implementation-review/remote-completion proof를 제거하며, `implement`
이후 ledger entry를 `stale: completed execution reseed (<old> -> <new>)`로 표시한다. Branch,
worktree, PR/MR artifact, feedback, decision, sync-base event와 이전 history는 보존된다. History가
없는 기존 schema v1 record도 계속 읽을 수 있다. 새 completion은 receipt에 lease generation을
직접 기록한다. Generation이 없거나 0인 current completion은 invalid v1 state이며 요청의
`--completion-generation` 값으로 보정하지 않는다. Preview와 reseed는 durable stamped generation만
권위로 사용하고, 누락하거나 충돌하면 artifact prepare와 record CAS 전에 fail-closed한다.
Preview의 `next_command`는 stamped generation을 그대로 보존한다. Status가 반환한 exact generation-bound
`claim`을 실행한 뒤 새 HEAD에서 구현 검증, AI-slop proof, implementation review,
정상 forward phase를 다시 획득하고 새 evidence로 `execution complete`한다. 이전 completion을
재시도하거나 state JSON을 직접 지우지 않는다.

`issueops remote create-pr`와 `issueops execution reconcile`의
`remote_pr_create` 경로는 같은 publication capability handler를 사용한다.
초기 생성은 CLI에만 있고, 복구는 CLI와 MCP `issueops_execution` 양쪽에서 같은
reconcile handler로 흐른다. Handler가 조립되지 않은 compatibility/test wrapper는
provider를 직접 resolve하거나 legacy create/reconcile로 우회하지 않고 기존
provider-unavailable 계약으로 fail closed한다.

`issueops execution complete` requires phase `pr`, the exact active generation,
final HEAD, committed verification report, verification evidence, and the verified
durable remote artifact URL. Entering `pr` records the canonical HEAD as `pr_head`;
completion refuses a final HEAD whose diff from `pr_head` touches anything besides
the verification report, because that change never passed cleanup, docs review, or
verification. After a post-PR fix, refresh cleanup (`issueops phase --to ai-slop-clean`),
re-record the reviews `next` reports, and run `issueops phase --to pr` again: re-entry
passes the same strict pr gate and reseals `pr_head`. Records without `pr_head` skip the
check; reseed clears it. Completion records `done` and releases the lease
atomically. It never merges or deletes a worktree, branch, or remote
resource. Operational release evidence includes fake-provider recovery
matrices and installed Codex/Claude hook smokes.

`issueops execution sync-base` is the post-completion conflict-resolution
surface (#114, #318). Completed replacement preview fetches and checks the
recorded parent first. If the parent is not an ancestor of the completed work
HEAD, it returns `post_completion_sync_base_required` and the exact
`--preview --completion-generation N` command instead of reseed. Run that
preview from the canonical worktree, then execute its generation- and
fingerprint-fenced `--apply --confirm --fingerprint` command. Conflicts stop as
a merge-in-progress; the emitted exact generation-bound `--finalize` commits
and pushes, while `abort_command` withdraws. Apply/finalize/abort accept the
existing active holder path or a released current completion with matching
stamped generation and live native actor. They append only `sync_base_events`;
current completion, history, and phase remain immutable. After successful sync,
run completed replacement preview, reseed/claim, verify, and re-complete.
Rebase and force-push remain rejected. `execution reconcile
--preview` output is a constant, not an inventory observation — do not cite it
as residue evidence.

The #303 provenance boundary covers the typed
`BaseSyncRequiredError.next_command` on both reachable CLI and MCP error paths.
On the CLI sync-base success path, finalize and abort commands from one response
share one canonical executable/hash/generation observation. If observation or
binding fails, the adapter returns the structured provenance error without an
unbound command fallback. Per #326, the public MCP action enum has no sync-base
success action/result: AC-06 host parity is the exact CLI command plus the
Codex/Claude hook classifier, while MCP binds reachable replace
`BaseSyncRequiredError` output only.

Do not run `execution replace --revoke` against your own live lease. That leaves
the cycle `revoking` with a live holder and closes every exit; `release
--generation N` is the correct command and the guard now refuses the self-revoke
with that instruction (#170). Note the actor identity too: `--session-id` is the
host session identifier, not a value you compose — it survives a session restart
while the PID changes, so `holder_identity_mismatch` after a restart means the
wrong id was recorded, not that the holder died.

## Execution sequence

Use this order:

```text
provider branch + base SHA
-> approved child plan in coordinator temp file + artifact stage
-> execution prepare preview/confirm
-> worktree receipt + durable plan_path + generation 1 to the preparing session
-> optional session handoff: release -> replace --preview -> --reseed -> claim --claim-current-token
-> compatibility/devil's-advocate gates + phase=implement
-> TDD/verification in the canonical worktree
-> AI-slop evidence + phase=ai-slop-clean
-> implementation review + atomic commit/push + phase=pr
-> generation-fenced draft PR/MR + readback
-> execution complete
-> separate human merge and cleanup choice
```

The write fence is the exact lifecycle ID, generation, native process receipt,
and canonical worktree. The source main worktree remains available for
unrelated work throughout the sequence. Plan 연결과 세 phase 전이는 `issueops next`가
돌려준 exact command를 사용한다. 세션이 phase를 추론해 건너뛰거나
cleanup evidence 기록이 phase를 자동 전이한다고 가정하지 않는다.

## IssueOps 10단계 운영

단계 판별은 `issueops next --json`이 소유한다. 읽기 전용이며 현재 단계,
미충족 게이트, 다음 명령, 탈출 경로를 돌려준다. 세션 경계는 하나뿐이다: 1~3단계는
source checkout의 준비 세션이, 4단계 이후는 canonical worktree의 구현 세션이 수행하며,
그 경계를 만드는 것이 3단계 끝의 worktree 준비와 세션 인계다. 일반 경로는
`execution prepare --id ID $ACTOR_FLAGS`를 preview로 확인한 뒤 `--confirm`으로 만들고,
`skills/issueops/references/session-choice.md`를 따른다. 준비 세션이 Herdr 세션이고 실행 중인
서버·호환성·현재 native host 실행 가능 여부가 확인되면 같은 worktree의 새 Herdr 세션으로
인계한다. Herdr를 쓸 수 없으면 현재 세션에서 이어가거나 사용자가 직접 연 두 번째
터미널에서 수동으로 인계한다. Claude Code·Codex·Omo·omp·opencode host를 유지하며
Herdr에서는 `worktree create` 대신 `worktree open`으로 준비된 checkout을 등록한다.
세션 인계는 기존 세션이 `release`로 lease를 해제하고, 새 세션이 자기 native actor로
`execution replace --id ID --expected-generation N --preview`를 실행해 반환된 `--reseed`
명령을 따른 뒤 `claim --claim-current-token`을 실행하는 체인으로만 이루어진다. 어느 단계에서든
빠져나오는 길은 `issueops-abandon`이고, 미머지 사이클의 원격 정리는 `cleanup abandon`의
`--close-pr`·`--close-issue`·`--delete-remote-branch`가 소유한다.

- child 스폰 준비: 승인된 child plan을 source checkout 밖의 임시 파일에 작성 → `issueops artifact stage --id ID --name plan --file PATH --json` → `issueops execution prepare --id ID $ACTOR_FLAGS`. `spec|verified-execution-loop`도 prepare 전에 stage할 수 있고 잘못 올렸으면 `artifact unstage`한다. (`implement` 역할(child cycle은 `child-implement`)은 `issueops model` 설정으로 해석한다. 현재 값은 `issueops model show --json`으로 읽는다. 리뷰·조사·독자 검토 모델도 같은 해석기에서 나오고, 조사 모델은 허용된 읽기 전용 조사에만 사용하며 구현·리뷰 게이트 판정을 맡기지 않는다. Fable 5는 명시적 지정만 허용).
- 다중 사이클 조망: `issueops list [--repo PATH] --json` — one-query SQLite snapshot에서 물리 `scanned_records`, `read_errors`/`unreadable_ids`/bounded `diagnostics`와 claimable/cleanup/unreflected/pending/failure/cleanup-failure 상태를 함께 노출한다. invalid row의 raw payload나 free-form 오류는 출력하지 않는다.
- 하위 세션 publication: `issueops implementation-review record --id ID --verdict pass --finding ... --evidence ... --reviewer-model <planner급>` 기록 후에만 `remote create-pr`가 통과한다. diff가 바뀌면 stale로 다시 막힌다.
- 머지(`issueops-merge`, 메인 세션): `issueops list --json`의 `issue_url`/`remote_artifact_url`로 ID를 찾는다 → `remote merge-pr --id ID --json`(기본 squash) → 차단 사유가 없을 때 `remote merge-pr --id ID --confirm --json` → `cleanup remote-branch --preview` → `--apply --confirm --fingerprint FP`. `checks_failing`(코드 결함)·`head_mismatch`는 `execution replace --preview` → `--reseed`로 사이클을 다시 열어 워크트리 세션에 넘기고, `merge_conflict`는 `execution sync-base --completion-generation N` 뒤 같은 reseed 경로를 밟는다.
- 머지 후 정리(순서 고정): `cleanup status --merged` → `cleanup close-children --merged --confirm` → `remote reflect-completion --body-file RESULT.md --confirm` → `remote close-issue --confirm` → `cleanup finish --preview` → `cleanup finish --apply --confirm --fingerprint FP`. finish 재실행 전에는 preview로 fingerprint를 재발급한다.
- 검증 보고서는 **원격 아티팩트(PR/MR) 생성 이전에** 커밋한다. `execution complete`가 리포트를 요구하는 시점이 머지 이후면 그 커밋이 어느 아티팩트에도 실리지 못해 두 번째 PR/MR이 필요해진다(GitHub PR에서 실측, #153). 원인은 provider가 아니라 `execution complete`의 리포트 요구 시점이므로 GitLab MR에서도 같을 것으로 보이나 실환경에서 확인하지 않았다. #153이 그 상황을 회복 가능하게 고쳤지만(원격 tip이 base에 도달했으면 통과) 애초에 만들지 않는 편이 낫다.
- 하네스 코드를 고친 뒤에는 **설치본을 재빌드한다**: `go build -o bin/issueops ./cmd/issueops`. `~/.local/bin/issueops`는 그 경로를 가리키는 심볼릭 링크이므로 머지만 하고 재빌드를 빠뜨리면 고친 동작이 나오지 않는다 — #153 cleanup에서 로컬 `main`이 두 머지 뒤처져 #154의 진단 필드가 출력되지 않았고 원인을 찾는 데 시간이 들었다.
- 생성된 IssueOps `next_command`의 absolute 첫 token과 끝의 `--generated-by-executable`, `--generated-by-sha256`, `--generated-for-generation`은 제거하거나 수동 보정하지 않는다. 첫 token이 생성 바이너리를 정확히 선택하므로 stale PATH에서도 동일 바이너리를 실행하며, 현재 바이너리는 subcommand 실행 전에 envelope를 canonical executable·SHA-256·durable generation과 비교한다. Hook은 durable worktree/source의 canonical binary 밖 absolute target, wrapper, substitution을 먼저 거부한다. 관측 실패나 mismatch가 나면 PATH를 바꾸거나 다른 바이너리로 우회하지 말고 structured error를 그대로 복구 근거로 사용한다. 사람이 직접 실행하는 일반 `issueops …` 명령은 계속 PATH를 사용한다.

## 역할별 모델 기본값과 설정

README의 「모델과 세션 인계」가 이 절을 가리킨다. 아래 표는 `internal/domain/agentmodel/defaults.go`의
내장 기본값이다. `child-implement`는 따로 설정하지 않으면 `implement` 값을 따른다. `review-escalate`는
리뷰 3라운드부터 쓰며, 따로 설정하지 않으면 리뷰 모델을 그대로 쓰고 effort를 한 단계 올린다.

| 호스트 | 구현(`implement`) | 계획·리뷰(`plan-review`·`diff-review`) | 조사(`research`) | 독자 검토(`reader-check`) |
|---|---|---|---|---|
| Codex | `gpt-6.1-sol` / `high` | `gpt-6-astra` / `high` | `gpt-6-luna` / `medium` | `gpt-6-luna` / `low` |
| Claude Code | `claude-opus-5-5` / `high` | `claude-opus-5-5` / `high` | `claude-sonnet-5-5` / `medium` | `claude-haiku-5-5` / `medium` |
| Omo native | `chatgpt-subscription/gpt-6-sol` / `max` | `chatgpt-subscription/gpt-6-astra` / `max` | `chatgpt-subscription/gpt-6-luna` / `medium` | 없음 |
| omp | `anthropic/claude-opus-5-5` / `high` | `anthropic/claude-opus-5-5` / `high` | `anthropic/claude-sonnet-5-5` / `medium` | `anthropic/claude-haiku-5-5` / `medium` |
| opencode | `opencode/deepseek-v4.1-flash-free` / `max` | `opencode/deepseek-v4.1-flash-free` / `max` | `opencode/deepseek-v4.1-flash-free` / `high` | `opencode/deepseek-v4.1-flash-free` / `low` |

다섯 호스트 모두 역할별 model·effort를 사용자 전체(global)나 저장소 하나(local) 단위로 바꿀 수 있다.
우선순위는 필드마다 명시 플래그 > local > global > 기본값이다. local 설정은 메인 워크트리의
`.issueops/agent-models.local.json` 하나이며 커밋하지 않는다. 연결 워크트리도 같은 파일을 읽는다.
global은 `$XDG_CONFIG_HOME/issueops/agent-models.json`(변수가 비면 `~/.config/issueops/`)이다.
대화로 바꾸려면 [`io-model`](../../../skills/io-model/SKILL.md) 스킬을 쓴다.

IssueOps가 cmux로 띄우는 Claude Code와 Codex 세션에는 리뷰·조사·독자 검토 역할을
서브에이전트로 넣는다. Omo native, omp, opencode 세션에는 넣지 않는다. 리뷰 3라운드부터는
`issueops model resolve --round N`이 돌려주는 상향 값을 쓴다. opencode effort는 모델마다 다른 variant
이름이므로 문서 전용 리뷰의 `medium` 하향과 3라운드 상향을 적용하지 않는다. 대화형 opencode 세션은
`--model` 플래그가 없어 `OPENCODE_CONFIG_CONTENT`의 모델만 받고 effort는 사용자 `model.json`의 variant를 쓴다.
Fable은 기본값이나 상향에 쓰지 않고
이름으로 지정할 때만 쓴다. `chatgpt-subscription/`은 Omo의 provider 식별자이며 Codex 호스트 이름을
바꾸는 설정이 아니다.
