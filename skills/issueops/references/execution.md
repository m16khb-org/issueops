# IssueOps Execution v1

IssueOps v1 stores one `Execution` in each lifecycle record. That execution
owns one canonical worktree and one generation-fenced write lease. At most one
native holder may mutate the record and worktree. The exact lifecycle ID,
generation, native process receipt, and canonical cwd are the authority; a
source checkout, branch name, or host session alone is not.

Preparation provisions the sibling worktree and grants generation 1 to the
calling Codex, Claude, Omo, omp, or opencode session. A different session takes over only
through the release, replace, and claim chain described below.

## Prepare

Run the preview first and inspect the workspace root, branch, and base SHA it
reports, then create the workspace with `--confirm`:

```bash
issueops execution prepare --id "$ISSUEOPS_ID" $ACTOR_FLAGS --json
issueops execution prepare --id "$ISSUEOPS_ID" $ACTOR_FLAGS --confirm --json
```

`ACTOR_FLAGS` are the exact native process identity and cwd:

```text
--host codex|claude|omo|omp|opencode --session-id ID [--agent-id ID]
--session-pid PID --session-started-at RFC3339
--session-executable PATH --cwd PATH
```

Omo native sessions use `PI_SESSION_ID` and a live Omo runtime receipt.
In Omo 5.x RPC mode, the `omo` launcher detaches its persistent host and the
observed host process is named `senpi`; that `senpi` receipt is the runtime
equivalent of the launcher receipt and is accepted only for `--host omo`.
`issueops execution whoami --json` resolves the runtime receipt
from the local ancestry automatically. It never falls back to the session ID
alone.
omp sessions use `ISSUEOPS_OMP_SESSION_ID`, which the issueops omp extension
exports from the main session, and a live receipt from the nearest `omp` or
`bun` ancestor (omp runs as a bun script); that receipt is accepted only for
`--host omp`. Omo and omp session ids together are ambiguous and fail closed.
opencode sessions use `OPENCODE_SESSION_ID`, which opencode exports to each
session's shell tools, and a live receipt from the nearest `opencode`
ancestor, accepted only for `--host opencode`. Lease ownership additionally
requires the receipt process to be the standalone server: claim, prepare,
mutation authority, and `mcp authorize` read its live arguments and accept only
`opencode serve --stdio`, which `opencode --standalone` runs. A session of the
shared background service (`opencode serve --service`) is refused, so open
the owner session with `opencode --standalone`.

The provisioned path is the fixed sibling
`${repo}.worktrees/<branch-with-slashes-replaced>`. Preparation creates or
reuses only that exact branch/worktree pair and records its base SHA. Do not
create or link another worktree manually: `execution prepare` is the only owner
of local workspace provisioning. Earlier stages record the branch identity and
never run `git worktree add`.

Reuse is conditional, not automatic. `execution prepare` adopts a worktree that
already sits at that path only when all three hold: the path resolves to that
Git top level, its current branch equals the recorded branch, and its **HEAD
equals the recorded base SHA**. Any mismatch fails with `existing canonical
worktree identity does not match branch and base_head`. A worktree created
before the cycle is therefore adoptable only while it is still untouched at the
base commit; once it carries a commit, either record that commit as the base SHA
or remove the worktree and let `execution prepare` create it.

Root every patch, edit, search, and code-intelligence tool at that path. Use
`rg`, CodeGraph when the repository is indexed, and native file tools rooted at
the canonical worktree; any separately installed code-intelligence tool must use
the same root. After edits, combine exact-string search with the relevant tests
so stale references and wrong-root changes are observable.

엄브렐라 자식은 branch prepare에서
`--parent-worktree ${repo}.worktrees/<base-branch-with-slashes-replaced>`를
명시한다. IssueOps는 이를 `workspace.parent_worktree`에 함께 봉인하고 canonical
부모 경로와 다르면 실행 전에 거부한다. 기존 delegation cycle은 이 값이 없어도
같은 경로를 계산한다. 독립 cycle은 `--parent-worktree`를 생략해 top-level을 유지한다.

## Status And Claim

Status is the read-only orientation command:

```bash
issueops execution status --id "$ISSUEOPS_ID" --json
```

The holder that ran preparation does not claim again. A new session claims only
through holderless recovery: `replace --reseed|--finalize` returns an exact
`claim` command for the new claimable generation. That command intentionally
omits `ACTOR_FLAGS`: the CLI observes the invoking native session and its
PID-reuse-safe ancestry receipt. Supplying actor flags remains supported, but
they must be supplied as one complete set; a partial set fails closed. The new
session runs the exact current-generation claim command rendered by
replacement/status:

```bash
issueops execution claim \
  --id "$ISSUEOPS_ID" \
  --generation "$GENERATION" \
  --claim-current-token \
  $ACTOR_FLAGS \
  --json
```

The CLI resolves the token from its private deterministic path. Its value is
never printed, and the path is never copied into claim commands. The token is
consumed exactly once.

## Release, Replacement, And Reconciliation

The active holder may voluntarily release its exact generation:

```bash
issueops execution release \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
```

Handing a cycle to another session, whether a Herdr session or a second terminal
the user opened, uses only this chain: the existing session releases the lease,
the new session runs `replace --preview` with its own native actor, executes the
returned `--reseed` command, and claims the new generation with
`--claim-current-token`.

Replacement is a fail-closed sequence. There is no unsafe override:

1. `issueops execution replace --preview` returns the exact generation and
   inventory fingerprint.
2. `issueops execution replace --revoke --expected-generation N
   --inventory-fingerprint HEX --reason TEXT --confirm` revokes that generation.
3. `issueops execution replace --finalize-preview --expected-generation N`
   proves the old process is quiescent and returns a quiescence fingerprint.
4. `issueops execution replace --finalize --expected-generation N
   --quiescence-fingerprint HEX --confirm` makes that generation claimable.
5. The new session claims with `--claim-current-token`. A failed generation
   change preserves the previous durable lease and removes uncommitted
   generation token files. A retry first recovers exact harness-owned residue
   for that still-uncommitted generation.

Every replacement step requires `ACTOR_FLAGS`. `--reseed` is limited to the
documented holderless recovery case and still uses generation CAS and confirm.
`replace --finalize|--reseed` returns the exact token-backed claim command:

```bash
issueops execution claim \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --claim-current-token \
  $ACTOR_FLAGS --json
```

A released cycle is recovered through the finite `next_command` chain
returned by each read/mutation result:

```text
execution status $ACTOR_FLAGS
  -> execution replace --preview $ACTOR_FLAGS
  -> execution replace --reseed --inventory-fingerprint <preview fingerprint> --confirm $ACTOR_FLAGS
  -> execution claim --claim-current-token $ACTOR_FLAGS
```

`next` routes by lease state: a claimable generation goes to
`claim --claim-current-token`, a released one to `replace --preview`, and a
revoking one to `replace --finalize-preview`.

Do not skip the preview or invent the fingerprint. For a completed cycle,
`execution status` does not render this chain; start at `execution replace
--preview`, which still renders the reseed command (after any base sync it
requires). `issueops-merge` uses this entry to reopen a completed cycle whose
PR/MR cannot merge.

When workspace provisioning or remote publication may have mutated external
state but the result is ambiguous, inspect and then confirm the exact
reconciliation:

```bash
issueops execution reconcile --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json
issueops execution reconcile --id "$ISSUEOPS_ID" --confirm $ACTOR_FLAGS --json
```

Do not retry the external create operation before reconciliation reports one
unambiguous result.

## Draft PR/MR And Completion

Only the active generation may create the draft PR/MR. The request carries the
expected generation, exact head/base branches, native actor, canonical cwd,
labels, and assignee, and uses preview then `--confirm`:

```bash
issueops remote create-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --title "$TITLE" --head "$BRANCH" --base "$BASE_BRANCH" \
  --body "$BODY" --label "$LABEL" --assignee "$ASSIGNEE" \
  $ACTOR_FLAGS --confirm --json
```

Completion is allowed only from phase `pr`, with the verified durable remote
artifact at the exact URL, a full final Git SHA, a verification report, and repeatable
verification evidence:

```bash
issueops execution complete \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --final-head "$FINAL_HEAD" \
  --verification-report "$VERIFICATION_REPORT" \
  --remote-artifact-url "$PR_URL" \
  --verification "$VERIFICATION" \
  $ACTOR_FLAGS --confirm --json
```

Successful completion records the receipt, moves the lifecycle to `done`, and
releases the lease atomically. It does not merge the PR/MR or delete the
worktree or branch.

## Host-Aware Model Defaults

`issueops model resolve --host "$HOST" --role implement --agents --json`이 `implement` 역할
(delegated child cycle은 `child-implement`)을 `issueops model` 설정으로 해석해 model·effort·
`agent_args`를 돌려준다. 현재 값은 표로 옮겨 적지 않고 `issueops model show --json`으로 읽는다.

리뷰(`diff-review`)·조사(`research`)·독자 검토(`reader-check`) 모델도 같은 해석기에서 나온다.
Herdr·cmux가 띄우는 세션에는 이 역할이 서브에이전트로 주입된다(Claude `--agents`,
Codex `-c agents.issueops-<role>.config_file=…`). 조사 모델은 위임이 이미 허용된 범위의
읽기 전용 탐색·자료 요약에만 쓰고, 구현·계획 확정·리뷰 게이트 판정에는 쓰지 않는다.
모델·effort 해석 규칙의 정규 소유자는 `internal/domain/agentmodel`이다.

Fable 5는 자동 기본값·폴백·리뷰 상향에 쓰지 않는다. 필요한 경우에만
`issueops model set`으로 명시한다.

## Artifact Staging And Sealing

메인(planner) 세션은 prepare **이전에** 승인된 child plan을 source checkout 밖의
임시 파일에 쓰고 계획/스펙/verified-execution loop를 스테이징한다. Delegation의
`parent_plan_path`는 child plan readiness 입력이 아니다:

```bash
issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file plan.md --json
issueops artifact stage --id "$ISSUEOPS_ID" --name spec --file spec.md --json
issueops artifact stage --id "$ISSUEOPS_ID" --name verified-execution-loop --file verified-execution-loop.md --json
```

- 이름은 `plan|spec|verified-execution-loop` 고정, 파일당 1MiB 상한, secret 패턴은 거부된다(스크럽 없음).
- `intent`는 staging 대상이 아니다. prepare가 `record.intent`(원문 요청·해석·성공 기준·비목표·
  제약·모호함·intent_class)를 렌더해 같은 디렉터리에 `intent.md`로 봉인하고 manifest에
  `intent`로 넣는다. 문서에 기록 시각이 없어 같은 내용의 재기록은 reseed를 그대로
  통과하고, 내용이 달라진 재봉인은 plan과 같이 불변 writer가 거부한다. 봉인 뒤
  `intent.md`를 다시 만들 CLI 경로가 없으므로 읽는 쪽
  (review·verify·create-pr)은 `status --json`의 `.intent`를 fallback으로 본다. 자격 증명
  형태는 `intent record`가 기록 시점에 거부하고, 그 이전에 기록된 record나 delegation이
  만든 child intent는 prepare의 materialize가 거부하며 그때는 `intent record`를 고친 뒤 같은
  prepare를 다시 실행한다(child record는 `--intent-class delegated-child`를 유지한다).
- After the worktree receipt, prepare materializes each artifact as a `0600` file under
  `execution.workspace.artifact_dir`. For an issue-linked cycle, the canonical path is
  `<worktree>/.issueops/issues/<provider-issue-number>/artifact/<name>.md`. Prepare
  rejects a linked issue URL without an issue number, and a record with an empty
  `artifact_dir` is rejected instead of guessing a sealed path. The temporary source
  may be deleted after successful readback.
- prepare 전 잘못 스테이징했으면 `issueops artifact unstage --id ID --name NAME`.
- `execution replace --finalize|--reseed` 재봉인은 staged 원본과 기존 durable
  `plan_path`를 다시 읽어 exact digest identity를 요구한다. Replacement는
  `plan_path`를 새로 만들거나 바꾸지 않으며, immutable artifact writer는 동일
  바이트만 허용한다.
- 이 디렉토리는 gitignore 대상이다 — 보존은 completion 섹션이 담당한다.

## Cross-Project Cycles

이슈가 있는 프로젝트와 코드가 있는 프로젝트가 다를 수 있다(기획 프로젝트의
이슈, 서비스 프로젝트의 코드). `branch prepare`는 checkout의 `origin`을 관찰해
이슈 프로젝트와 다르면 `code_project_key`를 봉인한다. 관찰이 안 되거나 값을
고정해야 하면 `--code-project-key HOST/GROUP/PROJECT`로 선언한다.

- 봉인된 키가 있으면 `remote create-pr`이 그 프로젝트에 PR/MR을 만들고,
  `remote verify-artifact`와 `execution complete`가 같은 키와 대조한다.
- 봉인이 없으면 이슈 프로젝트가 곧 코드 프로젝트다 — 기존 사이클의 동작.
- 검증이 느슨해지는 것이 아니라 대조 대상이 정확해질 뿐이다. 봉인된 키와 다른
  프로젝트의 아티팩트는 여전히 거부된다.
- 이 봉인이 없으면 cross-project 사이클은 `remote_artifact`를 끝내 채우지 못하고
  cleanup이 `remote_artifact` 미충족으로 멈춘다.

## Publication Evidence Gates

구현 diff가 확정된 뒤 implementation review **전에** 두 게이트를 통과한다.
두 기록 모두 변경 집합 fingerprint를 봉인하므로 문서 수정이 기록보다 먼저다.

```bash
issueops project-docs-review record --id "$ISSUEOPS_ID"   --verdict updated --doc "<고친 문서 경로>" --evidence "..."   --host codex --session-id "$SESSION" --cwd "$WORKTREE" --json
```

- 게이트: `project_docs_review`. verdict는 `updated|no-change`, evidence는 항상
  1개 이상이다. `updated`는 `--doc` 경로가 실제 변경 집합에 있어야 통과하고,
  `no-change`는 `--doc`을 받지 않는다.
- 이후 diff가 바뀌면 `project_docs_review_stale`로
  create-pr·strict readiness가 거부한다.

```bash
issueops schema-evidence record --id "$ISSUEOPS_ID"   --measurement "orders: 8.4M rows(reltuples), idx_orders_user_id 없음"   --source "mcp db-bc-prod execute_sql_bc_prod_market"   --host codex --session-id "$SESSION" --cwd "$WORKTREE" --json
```

- 게이트: `schema_evidence`. 변경 집합에 마이그레이션·엔티티·`.sql`·
  `schema.prisma` 경로가 있을 때만 활성화되고, 없으면 요구되지 않는다.
- measurement와 source는 짝이다. 관찰이 불가능하면
  `--waive --waiver-rationale "..."`로 근거를 남긴다.
- 운영 DB에 전수 스캔을 던지지 않는다. 카탈로그 추정 row 수를 쓴다.

## Implementation Review Gate

사이클의 구현 세션은 publication 전에 planner급 모델 fresh 서브에이전트로
구현 diff의 design-review 적대 리뷰를 수행하고 결과를 기록해야 한다:

```bash
issueops implementation-review record --id "$ISSUEOPS_ID"   --verdict pass --finding "..." --evidence "..."   --reviewer-host codex --reviewer-model gpt-6-astra   --host codex --session-id "$SESSION" --cwd "$WORKTREE" --json
```

- 게이트: `verdict==pass` + findings/evidence 실질 내용. 기록은 implement phase
  이후에만 가능하며, 리뷰가 검토한 변경 집합 fingerprint가 봉인되어 이후 diff가
  바뀌면 `implementation_review_stale`로 create-pr·strict readiness가 거부한다.
- `reviewer_*`는 감사 기록이다 — 하네스는 모델 자기신고를 검증하지 않는다.
- execution이 있는 레코드가 대상이다. execution이 없는 레코드만 면제다.
  fingerprint를 계산할 수 없는 사이클은 빈 값으로 봉인되고, fingerprint가
  생기는 순간 stale로 잡혀 재기록을 요구한다.

## Post-Merge Cleanup Order

휴먼이 PR/MR을 머지한 뒤, 순서가 계약이다(모두 원격 readback 기반 fail-closed):

```bash
issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --confirm --json
issueops remote reflect-completion --id "$ISSUEOPS_ID" --body-file "$RESULT_FILE" --confirm --json   # 보존 먼저
issueops remote close-issue --id "$ISSUEOPS_ID" --confirm --json
issueops cleanup finish --id "$ISSUEOPS_ID" --preview --json
issueops cleanup finish --id "$ISSUEOPS_ID" --apply --confirm --fingerprint "$FP" --json
```

- `reflect-completion`이 사람이 쓴 진행 결과(`--body-file`)를 이슈 본문의
  completion 구간(`## 진행 결과`)에 반영한 뒤에만 finish가 통과한다. 최종 head,
  검증 요약, plan·spec은 이슈 본문이 아니라 record와 `.issueops/issues/<n>/`에 있다.
- finish apply는 워크트리 점유 프로세스 종료(`workspace_processes_stop`:
  fingerprinted 프로세스별 HUP+TERM → KILL → 최종 점유 재관측) →
  git worktree 제거 → 로컬 브랜치 CAS 삭제 → 응답에 감사 라인
  보고(이슈 본문은 쓰지 않음) → **레코드 삭제** 순서로 진행하며, 각 단계는 멱등이고 실패 시 레코드를
  보존한 채 실패 지점을 기록한다. 재실행 전에는 `--preview`로 새 fingerprint를
  발급받아야 한다(이전 값 무효). preview는 종료될 프로세스(receipt·자손 수)를 싣고
  fingerprint에 결속하며, 요청자 자신이 워크트리를 점유하면 거부한다(#477). abandon도 같은 단계를
  워크트리 제거 앞에 둔다.
- 원격 브랜치는 건드리지 않는다. 다중 사이클 조망과 정리 후보 발견은
  `issueops list --repo "$PWD" --json`을 사용한다.

## Parallel Independence

There is one active execution per record, not one global execution per source
repository. Fence only the selected exact lifecycle ID, generation, native
holder, and canonical worktree. Unrelated cycles remain independent. The
source main worktree remains available before, during, and after the execution
for unrelated work, but it must not mutate the selected execution or its
canonical worktree.
