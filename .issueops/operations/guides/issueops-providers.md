---
name: issueops-providers
description: IssueOps execution preparation, provider branch linkage, and publication contracts.
---

# IssueOps Provider Publication and Branch Contracts

This guide owns cycle preparation and provider branch linkage contracts for
IssueOps execution. The execution state machine, recovery, and session
workflow live in
[issueops-execution.md](issueops-execution.md). Canonical index:
[../../OPERATIONS.md](../../OPERATIONS.md).

## Preparation and Provider Publication Contracts

Use the focused skill for the current publication step:

- [`issueops-create-issue`](../../../skills/issueops-create-issue/SKILL.md) owns
  parent issue and provider-native child creation, scoring, examples, and
  reconciliation.
- [`issueops-create-pr`](../../../skills/issueops-create-pr/SKILL.md) owns
  linked PR/MR publication and live artifact verification.
- [`issueops-sync-issue`](../../../skills/issueops-sync-issue/SKILL.md) and
  [`issueops-sync-pr`](../../../skills/issueops-sync-pr/SKILL.md) own refreshing
  a body that has fallen behind the cycle that published it.

### Refreshing a published body

`sync-issue` and `sync-pr` replace the authored body of an artifact this cycle
already published. They never touch the managed blocks
(`issueops:completion`, `issueops:devils-advocate`, `issueops:issue-create`):
those are lifted out of the live body and spliced back after the replacement, so
a full-body rewrite cannot strand the cleanup readback that looks for the
completion marker.

Preview first. It reports `drift` (`in_sync` | `stale` | `remote_edited`), the
sections it will preserve, and the `expected_body_sha256` a confirm has to name.

```bash
issueops remote sync-issue --id ID --body-file BODY.md --json
```

A confirm is a compare-and-swap on the body: without `--expected-body-sha256`,
or when the live body no longer hashes to it, nothing is written. A body edited
outside the harness (`drift=remote_edited`) additionally needs
`--accept-remote-edits`, so an outside edit is folded into the replacement
deliberately rather than erased by accident.

```bash
issueops remote sync-issue \
  --id ID --body-file BODY.md --expected-body-sha256 SHA \
  --host codex --session-id SESSION --cwd WORKER_PATH --confirm --json
```

`sync-issue --url` addresses a provider-native child, and the hierarchy is
verified against the linked parent before anything is written. `sync-pr`
addresses only the artifact this cycle verified, is fenced by
`--expected-generation` like `create-pr`, and refuses a merged or closed
artifact. Every confirmed write is verified by readback and stored as the
record's `body_syncs` baseline for the next staleness check.

### Durable parent issue creation and recovery

`create-issue --confirm` requires a started IssueOps record, a canonical
`origin` remote, and `--template`. The body contract and readability check run
before anything is sealed; a critical finding refuses the confirm. The command stores a sealed intent before invoking `gh` or
`glab`, appends an operation marker to the issue body, performs live
label/assignee verification, then records the canonical issue URL and completed
intent atomically.

```bash
issueops remote create-issue \
  --id ID --provider github --title "Title" \
  --template implementation_task --body-file BODY.md \
  --label bug --assignee USER --confirm --json
```

If the provider process did not start, rerun the exact request; changed title,
body, labels, assignees, provider, or project authority are rejected. If the
process started but the result is unknown, do not rerun create. Search and
adopt through the durable marker:

```bash
issueops remote reconcile-issue --id ID --json
issueops remote reconcile-issue --id ID --confirm --json
```

Zero or multiple marker candidates remain blocked. One candidate is accepted
only when its title and full body digest match the sealed request and live
labels/assignees verify. `issueops list` shows
`[issue-create:<status>]`; `doctor` reports ambiguous, verification-failed, and
receipt-failed states. Normal short-lived `pending` intent is not unhealthy.

### Durable child creation and recovery

`create-child` seals a separate `child_create_operations` entry before the
provider create. It retains the parent `IssueURL`, request digest, child marker,
known URL, labels/assignees and execution authority. A replay without an ID
always selects the first implicit operation for the same normalized payload,
even after completion or a later explicit child with that payload.

For an intentional additional child, generate an ID once, save it and reuse it
on every retry:

```bash
CHILD_OPERATION_ID="$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
issueops remote create-child --id ID --operation-id "$CHILD_OPERATION_ID" \
  --title "Title" --template child_task --body-file BODY.md \
  --label enhancement --assignee USER --host codex --session-id SESSION \
  --cwd WORKER_PATH --json
```

Preview does not write state or call the provider. Only a proven
`not_invoked` result permits the exact create request to be retried.
An unresolved operation blocks additional creates, including fresh IDs.
GitHub reads valid `gh issue create --help` before choosing one create path;
an error after create never selects a second create.

```bash
issueops remote reconcile-child --id ID --operation-id OPERATION \
  --host codex --session-id SESSION --cwd WORKER_PATH --json
# Repeat the verified preview with --confirm to adopt the existing child.
```

Recovery checks the exact known URL, or requires one untruncated marker
candidate. Title/body digest, project, child type, labels/assignees and hierarchy
must verify. Confirm can attach the existing child, then records the child link
and completed receipt atomically; it never creates a replacement. A stale
operation requires explicit reconciliation by the current active holder.
CLI errors retain the operation ID, known URL and recovery command.

`issueops execution prepare --id ID $ACTOR_FLAGS`는 preview로 workspace root·branch·base를
보여 준다. 내용을 확인한 뒤 같은 요청에 `--confirm`을 붙여 다시 실행하면 worktree가 만들어진다.

prepare 전에 승인된 child plan을 source checkout 밖의 coordinator
임시 파일에 작성하고 `issueops artifact stage --id ID --name plan --file PATH --json`로
stage한다. 이 command에는 actor flag가 없다. prepare는 worktree receipt
직후 레코드의 `execution.workspace.artifact_dir` 아래에 권한이 `0600`인 artifact를
materialize한다. 이슈에 연결된 cycle의 canonical 경로는
`.issueops/issues/<provider-issue-number>/artifact/plan.md`다. 이슈 번호를 알 수
없는 연결 URL이면 prepare가 거부한다. Prepare는 이 경로를
durable `plan_path`로 같은 CAS에 기록한다. `parent_plan_path`만으로는 readiness를 충족하지 못한다.

엄브렐라 자식 cycle은 `issueops branch prepare`에 canonical
`--parent-worktree`를 기록하고 preview의 `workspace.parent_worktree`가 부모
통합 worktree를 가리키는지 확인한다. 내부 delegation이 없는 provider-native
자식도 같은 계약을 사용한다.

**GitHub linked branch는 봉인한 base SHA에서 만든다.** `createLinkedBranch`는 `oid`에서 새
브랜치를 만들기 때문에 **원격에** 같은 이름이 있으면 실패한다. 실제 값은 앞 단계 출력에서
그대로 옮겨 적는다 — 아래 꺾쇠 자리는 셸 변수가 아니다.

전체 계약은 `branch prepare` (base SHA only) → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified` 순서다.

```bash
issueops branch prepare --id ID --base-sha <BASE_HEAD> ...    # 기록만, 브랜치 없음
gh api repos/<OWNER>/<REPO>/issues/<NUMBER> --jq .node_id                   # 다음 단계의 issueId
gh api graphql -f 'query=mutation($issueId:ID!,$oid:GitObjectID!,$name:String!){createLinkedBranch(input:{issueId:$issueId,oid:$oid,name:$name}){linkedBranch{ref{name target{oid}}}}}' -F issueId=<NODE_ID> -F oid=<BASE_HEAD> -F name=<BRANCH>
issueops branch prepare --id ID ... --link-verified           # 추적 확인 후 갱신
```

`branch prepare`는 브랜치 존재를 검증하지 않고 이름 규칙과 레코드 일치만 보므로 첫 단계가
성립한다.

**`gh issue develop`을 쓰지 않는 이유는 base가 갈리기 때문이다(#176).** 그 CLI의 `--base`는
브랜치 이름만 받고 GitHub이 **그 시점** 브랜치 HEAD를 `oid`로 쓴다. `execution prepare`는 봉인된 base SHA에서
worktree를 만들므로, 그 사이 base 브랜치가 진행하면 push가 `non-fast-forward`로 거부되고
봉인 가드가 `merge`를, 안전 훅이 force push를, `sync-base`가 completion 이전 실행을 막아 발행
경로가 사라진다. `oid`는 `createLinkedBranch`의 필수 필드이므로 봉인 SHA를 직접 넘기면 그
갈림이 원리적으로 생기지 않는다.

두 가지 형태 제약이 있다:

- **GraphQL 변수는 단일 인용해야 한다.** `$issueId` 같은 토큰이 인용 밖에 있으면 lifecycle
  가드가 셸 파라미터 확장으로 판정해 명령을 거부한다. 같은 이유로 `oid`·`name`·`issueId` 값도
  셸 변수로 넘기지 않고 리터럴로 적는다.
- **node ID 조회는 별도 단계다.** 한 명령에 `$(...)`를 넣으면 가드가 정적으로 분류할 수 없다.
  출력값을 그대로 옮겨 적는다.

`branch prepare`가 이 두 명령을 실행 가능한 형태로 `Steps`에 렌더하므로 `--base-sha`를 준
사이클은 그 출력을 그대로 따르면 된다. 문서와 `Steps`가 어긋나면 `Steps`가 원본이다.

GitLab은 브랜치 이름 규칙이 연결 수단이라 GitHub의 linked-branch 생성 단계가 따로 없다.
그래도 base 못박기는 같은 계약이다. GitLab의 `ref`는 commit SHA를 받으므로
`--base-sha`를 주면 `Steps`가 그 값을 `ref`로 안내한다(`#180`).

GitLab issue 본문을 읽는 provider recipe 탐색과 `.issueops/VCS.md` 기록은
[`gitlab-usecase`](../../../skills/gitlab-usecase/SKILL.md) 스킬이 소유한다.
