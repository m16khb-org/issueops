---
name: sync-base
description: Bring the current branch up to date with the base branch it was created from after that base advanced, choosing merge or rebase by an evidence-based decision table, with evidence-based base resolution, a verified backup ref, proof that no commit was lost, and a confirmed push (plain for merge, --force-with-lease for rebase). Use when a release or other base branch moved ahead and the feature branch must catch up or pre-resolve conflicts, or when the user says "베이스 브랜치 최신화", "release 브랜치 반영", "브랜치 최신으로 맞춰줘", "부모 브랜치 rebase", "base 브랜치 merge", "sync with base branch", "catch up with base", "rebase onto parent", "refresh a stacked branch". Git Operations sub-skill; an issueops-owned branch routes to `issueops execution sync-base` instead.
---

# Sync Base

> **Git Operations sub-skill.** Git has no notion of a base branch, so this skill
> resolves one from recorded evidence, proves the base actually advanced, picks
> merge or rebase from evidence, and applies it with a verified recovery path.
> Interactive rebase, bisect, and reflog archaeology belong to the parent
> **`git-operations`** skill. Ordinary commit and push belong to
> **`atomic-commit-push`**.

**User's request:** $ARGUMENTS

This skill owns exactly five effects:

1. resolve the base branch of the current branch and record the decision;
2. determine whether that base has advanced past the divergence point;
3. choose merge or rebase with the decision table and report the deciding row;
4. merge the base in, or rebase onto it, with a verified backup ref;
5. optionally push after explicit confirmation: a plain push after a merge,
   `--force-with-lease` after a rebase.

It never resolves conflicts on its own, never uses bare `--force`, and never
syncs a branch owned by an active issueops cycle.

## Routing gate (run first)

1. **IssueOps-owned branch?** Check `ISSUEOPS_ID` in the environment and run
   `issueops list --repo "$PWD" --json`. If a cycle owns this
   branch, stop and route to `issueops execution sync-base`. That
   surface is merge-based and bound to the execution lease generation; syncing
   here bypasses the lease and the recorded PR head.
2. **Protected branch?** Refuse to sync `main`, `master`, `develop`, or
   `release/*` itself unless the user names the branch and confirms in the same
   message. Syncing a feature branch from one of them is the normal case.
3. **Orca is optional.** Orca worktree lineage is one evidence source among
   several. Never require an Orca runtime, and skip the source when
   `orca status --json` does not report a ready runtime.

## Safety rules

- The working tree must be clean and no rebase or merge may be in progress.
- Create and verify a backup ref before the merge or rebase starts. No backup,
  no sync.
- Never resolve a conflict automatically. Report both sides and hand the choice
  to the user.
- After a rebase, push only with
  `--force-with-lease=<branch>:<pre-sync remote SHA>`. Refuse bare `--force`
  and refuse a lease without an explicit expected value. After a merge, push
  without any force flag.
- Every claim in the final report must cite a command output, not an assumption.

## Step 1 — Resolve the base branch

Collect evidence in this order and stop at the first hit. Report which source
answered so the user can judge the confidence.

| Rank | Source | Command | Confidence |
|---|---|---|---|
| 1 | This skill's own record | `git config --get branch.<branch>.parent` | exact |
| 2 | Open PR base / MR target | `gh pr view --json baseRefName` / `glab mr view -F json` (`target_branch`) | exact |
| 3 | `gh` CLI merge base | `git config --get branch.<branch>.gh-merge-base` | exact |
| 4 | IssueOps pinned base | `git config --get branch.<branch>.base` | SHA, needs lookup |
| 5 | Orca worktree lineage | `orca worktree show --worktree path:<path> --json` | exact when ready |
| 6 | VS Code merge base | `git config --get branch.<branch>.vscode-merge-base` | usually the default branch |
| 7 | Divergence-point lookup | reflog SHA plus candidate comparison (below) | inferred |
| 8 | Repository default branch | `git symbolic-ref refs/remotes/origin/HEAD` | weak, announce it |

The record key stays `branch.<branch>.parent` so records written under this
skill's former name, `rebase-onto-parent`, keep answering rank 1.
`branch.<branch>.base` belongs to IssueOps and holds a SHA; never write it here.

```bash
BRANCH=$(git branch --show-current)
git config --get "branch.$BRANCH.parent"
git config --get "branch.$BRANCH.gh-merge-base"
git config --get "branch.$BRANCH.base"
git config --get "branch.$BRANCH.vscode-merge-base"
```

Each `git config --get` exits 1 when the key is absent. Treat exit 1 as "no
evidence from this source" and continue down the cascade; do not treat it as a
failure of the skill.

### Rank 7: divergence-point lookup

`git switch -c` records only `branch: Created from HEAD`, so the reflog gives a
SHA and not a name. Recover the divergence SHA, then rank the candidate branches
by how recent their merge base is. The immediate base is the candidate whose
merge base equals the divergence point.

```bash
BRANCH=$(git branch --show-current)
FORK=$(git reflog show "$BRANCH" --format='%H %gs' | awk '/branch: Created from/ {print $1}' | tail -1)
git for-each-ref --format='%(refname:short)' refs/remotes/origin | while IFS= read -r REF; do
  if [ "$REF" != "origin/$BRANCH" ]; then
    MERGE_BASE=$(git merge-base "$REF" HEAD)
    printf '%s merge-base=%s base-ahead=%s\n' \
      "$REF" \
      "$(git rev-parse --short "$MERGE_BASE")" \
      "$(git rev-list --count "$MERGE_BASE..$REF")"
  fi
done
```

An older ancestor such as `origin/main` shows a merge base further back and
`base-ahead=0`; the real base shows a merge base equal to `$FORK`. When two
candidates tie, ask the user instead of guessing.

### Rank 8: repository default branch

```bash
git symbolic-ref --quiet refs/remotes/origin/HEAD
git remote set-head origin --auto
git symbolic-ref --quiet refs/remotes/origin/HEAD
```

The first call exits 1 in a repository where `origin/HEAD` was never written.
`git remote set-head origin --auto` creates it. Reaching rank 8 means the base
was inferred rather than recorded, so say so before asking for confirmation.

### Confirm and record

Present the resolved base with its evidence and the divergence figures, then
ask for confirmation. After the user confirms, record the decision so later runs
skip the cascade:

```bash
git config "branch.$BRANCH.parent" "<base-ref>"
```

## Step 2 — Has the base advanced?

```bash
git fetch --prune origin
MERGE_BASE=$(git merge-base "<base>" HEAD)
git rev-list --left-right --count "<base>...HEAD"
```

The left number counts commits the base has and this branch does not; the
right number counts this branch's own commits. A left value of `0` means the
branch is already current: report that and stop without touching anything.

Prefer the remote-tracking ref `origin/<base>` over the local branch. The
local copy of the base may be stale or checked out in another worktree.

## Step 3 — Choose merge or rebase

Rebase only when the evidence proves nobody else depends on this branch's
current commits. Walk the table top to bottom and stop at the first row that
matches. Report the row number and its command output with the decision.

| Row | Condition | Check | Mode |
|---|---|---|---|
| 1 | The user named a mode | the request | that mode |
| 2 | Another branch already contains this branch's commits | `git for-each-ref --contains "$OLDEST"` (below) prints any ref | merge |
| 3 | Someone else authored a commit on this branch | `git log --no-merges --format='%ae' "<base>..HEAD" \| sort -u` shows an address other than `git config user.email` | merge |
| 4 | An open PR or MR has review activity | `gh pr view --json reviews,comments` has a non-empty array, or `glab mr view -F json` has `user_notes_count > 0`, or `glab api "projects/:id/merge_requests/<iid>/approvals"` has a non-empty `approved_by` | merge |
| 5 | The branch already contains a merge commit | `git rev-list --merges --count "<base>..HEAD"` is above 0 | merge |
| 6 | None of the above | — | rebase |
| — | A check failed or its answer is unclear | — | merge |

```bash
BRANCH=$(git branch --show-current)
OLDEST=$(git rev-list --reverse "<base>..HEAD" | head -1)
git for-each-ref --format='%(refname:short)' --contains "$OLDEST" refs/heads refs/remotes \
  | grep -v -x -e "$BRANCH" -e "origin/$BRANCH"
```

Why each row picks merge:

- **Row 2.** Rebasing rewrites the hashes, so the branch that already took the
  old commits (a QA or staging branch, or a branch stacked on this one) ends up
  with a second copy of every change. Merging the two later records each change
  twice, and any later edit to one copy conflicts with the other.
- **Row 3.** A collaborator's local branch still points at the old commits; a
  rebase forces them to recover by hand.
- **Row 4.** Review comments anchor to commits. A rewritten history detaches
  them and hides what changed since the last review.
- **Row 5.** A plain rebase linearizes the branch and drops its merge commits,
  so every earlier conflict resolution must be redone. Once a branch has been
  synced by merge, this row keeps it on merge.
- **Unclear.** A wrong merge costs a less linear history. A wrong rebase breaks
  someone else's branch or review.

For row 4, `gh pr view` exiting with `no pull requests found` or an empty
`glab mr list --source-branch "$BRANCH"` means there is no PR or MR, so the row
does not match; it is not an unclear answer.

When row 1 names rebase and rows 2–5 would have chosen merge, show the matching
evidence and ask once before rebasing.

## Step 4 — Preflight

```bash
git status --porcelain
test -d "$(git rev-parse --git-path rebase-merge)" && echo "rebase in progress"
test -d "$(git rev-parse --git-path rebase-apply)" && echo "rebase in progress"
test -f "$(git rev-parse --git-path MERGE_HEAD)" && echo "merge in progress"
git rev-parse --verify --quiet "<base>^{commit}"
```

Non-empty `git status --porcelain` output means the tree is dirty: stop and ask
whether to stash. Any in-progress rebase or merge means stop. A missing base
commit means the resolution in step 1 was wrong; go back rather than continue.

Then create and verify the backup, and record the remote tip for the later push:

```bash
BRANCH=$(git branch --show-current)
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP="refs/backup/sync-base/$BRANCH/$STAMP"
git update-ref "$BACKUP" HEAD
git show-ref --verify --quiet "$BACKUP" && echo "backup verified: $BACKUP"
git rev-parse "origin/$BRANCH"
```

Report the backup ref name and the pre-sync remote SHA to the user before
proceeding. Both are needed for recovery, and the rebase push lease needs the
SHA.

## Step 5M — Merge

<!-- skill-shell: destructive recovery="restore from the verified refs/backup/sync-base ref recorded in step 4, or run git merge --abort while the merge is in progress" -->
```bash
git merge --no-edit "<base>"
```

The merge leaves this branch's commits untouched and adds one merge commit, so
no hash that anyone else holds changes. Continue with step 6 on a conflict,
otherwise with step 7M.

## Step 5R — Rebase

### Choose the rebase form

The plain form is correct only while the recorded divergence point is still an
ancestor of the base. When the base was itself rebased or amended, replaying
the old range makes Git re-apply a commit the base already carries in a
modified form, which produces a conflict in a file this branch never touched.

```bash
git merge-base --is-ancestor "<old-merge-base>" "<base>"
```

Exit 0 selects the plain form. Exit 1 selects `--onto`. Recover
`<old-merge-base>` from `branch.<branch>.base`, or from
`git merge-base --fork-point "<base>" "<branch>"`, or from the reflog
divergence SHA of step 1. `git merge-base "<base>" "<branch>"` is the wrong
input here: after the base moves it returns a much older commit, and using it
reintroduces the same duplicate replay.

`--fork-point` reads the base's local reflog, so it is unavailable in a fresh
clone. When no source yields the old divergence point and the plain form is not
provably safe, say so and ask the user rather than guessing.

### Run it

<!-- skill-shell: destructive recovery="restore from the verified refs/backup/sync-base ref recorded in step 4, or run git rebase --abort while the rebase is in progress" -->
```bash
git rebase --no-fork-point "<base>"
```

<!-- skill-shell: destructive recovery="restore from the verified refs/backup/sync-base ref recorded in step 4, or run git rebase --abort while the rebase is in progress" -->
```bash
git rebase --onto "<base>" "<old-merge-base>"
```

`--no-fork-point` is deliberate. Without it Git may silently drop commits it
believes the upstream already discarded, which is the opposite of the guarantee
this skill makes. The `--onto` form takes the base as the new base and the old
divergence point as the start of the range to replay.

## Step 6 — Conflicts

Stop at the first conflict. Do not choose a side and do not continue.

```bash
git diff --name-only --diff-filter=U
git show :2:<path>
git show :3:<path>
```

**The stage numbers mean opposite things in the two modes.**

| Mode | `:2:` (ours) | `:3:` (theirs) |
|---|---|---|
| merge | this branch | the base |
| rebase | the base, because it is checked out as the target being replayed onto | this branch's commit being replayed |

Label the sides by meaning in the report, never as "ours" and "theirs". During
a rebase also name the commit being replayed (`git rev-parse --short REBASE_HEAD`).
Then offer three options and wait:

1. the user resolves, then `git add <path>` and `git commit --no-edit` (merge)
   or `git rebase --continue` (rebase);
2. hand the file to **`git-operations`** for its conflict-resolution protocol;
3. abort and return to the pre-sync state.

<!-- skill-shell: destructive recovery="abort returns the branch to the pre-merge tip; confirm afterwards against the backup ref recorded in step 4" -->
```bash
git merge --abort
```

<!-- skill-shell: destructive recovery="abort returns the branch to the pre-rebase tip; confirm afterwards against the backup ref recorded in step 4" -->
```bash
git rebase --abort
```

If a rebase conflicts across more than three commits, abort, report the
pattern, and offer the merge mode instead: a merge resolves each file once
rather than once per replayed commit.

## Step 7M — Verify a merge

```bash
git merge-base --is-ancestor "<base>" HEAD
git merge-base --is-ancestor "<backup-ref>" HEAD
git diff --name-only "<backup-ref>" HEAD
git diff --name-only "<old-merge-base>" "<base>"
```

1. **The base is in.** The first command exits 0.
2. **No commit of this branch was lost.** The second command exits 0: the
   pre-sync tip is an ancestor of the new tip.
3. **The only new content is the base's.** The files from
   `<backup-ref>..HEAD` must be the files the base brought in. A file outside
   that list is a conflict resolution; name it and that resolution in the report.

## Step 7R — Verify a rebase

Two invariants must both hold before the result is reported as successful.

```bash
git range-diff "<old-merge-base>..<backup-ref>" "<base>..HEAD"
git rev-list --count "<old-merge-base>..<backup-ref>"
git rev-list --count "<base>..HEAD"
git diff --stat "<backup-ref>" HEAD
```

1. **Every commit survived unchanged.** Each `range-diff` row must read `=`.
   A `!` row means the patch changed, which is expected only where a conflict was
   resolved; name that file and that resolution in the report. A `<` or `>` row
   means a commit was dropped or added, which is a failure.
2. **The only new content is the base's.** `git diff --stat <backup-ref> HEAD`
   must show exactly the files the base brought in.

Use `<old-merge-base>..<backup-ref>` on the left and `<base>..HEAD` on the
right. Passing the base's tip on both sides mixes the base's own commits into
the comparison and produces rows that look like losses but are not.

## Step 8 — Push

Ask before pushing. Show the branch, the mode and its deciding row, the backup
ref, the pre-sync remote SHA, the exact command, and any open PR or MR with its
base branch.

After a merge the push is a fast-forward:

```bash
git push origin "<branch>"
```

A `! [rejected] ... (fetch first)` or `(non-fast-forward)` reply means
someone pushed to the branch meanwhile. Do not force it: fetch, then run this
skill again from step 2.

After a rebase the push needs the lease:

<!-- skill-shell: destructive recovery="the pre-sync remote SHA recorded in step 4 restores the remote ref; the lease rejects the push when the remote moved" -->
```bash
git push --force-with-lease="<branch>:<pre-sync remote SHA>" origin "<branch>"
```

The explicit expected value is what makes the lease meaningful. A value that no
longer matches the remote is rejected as `! [rejected] ... (stale info)`, and an
expected value Git cannot resolve to a known object degrades to an unforced push
and is rejected as `! [rejected] ... (non-fast-forward)`. Both mean the same
thing operationally: do not retry, re-read the remote and find out who moved it.

Refuse `git push --force`. Refuse `--force-with-lease` without `=<branch>:<sha>`,
because the bare form trusts the local remote-tracking ref, and a background
fetch can have already updated it.

## Recovery

The backup ref is the recovery path for every failure after step 4.

```bash
git for-each-ref "refs/backup/sync-base/<branch>" "refs/backup/rebase-onto-parent/<branch>"
git log --oneline -1 "<backup-ref>"
git diff --stat "<backup-ref>" HEAD
```

The second namespace holds backups written under this skill's former name.
Show the user what would be discarded, then ask for confirmation naming the
exact command before running it.

<!-- skill-shell: destructive recovery="the backup ref is verified to exist and its diff against HEAD is shown to the user, who confirms the exact command before it runs" -->
```bash
git reset --hard "<backup-ref>"
```

Backup refs under `refs/backup/` are not pruned by ordinary garbage collection
while they exist. Delete one only after the user confirms the sync result.

## Never

- Sync a branch an active issueops cycle owns; route to
  `issueops execution sync-base`.
- Rebase when a row from 2 to 5 of the decision table matched, unless the user
  confirmed after seeing that evidence.
- Sync without a verified backup ref.
- Resolve a conflict by picking a side without the user's decision.
- Push with bare `--force`, with a lease that carries no expected SHA, or with
  any force flag after a merge.
- Report success without every invariant of step 7M or 7R shown.
- Delete a backup ref in the same run that created it.

## Verified facts

Each row was reproduced in a scratch repository before being written here.

| Claim | Evidence |
|---|---|
| `git switch -c` records `branch: Created from HEAD`, never a branch name | reflog of a branch created with `switch -c` |
| The divergence-point lookup separates the immediate base from an older ancestor | the base showed `merge-base` equal to the fork point with `base-ahead=2`; `origin/main` showed an earlier merge base with `base-ahead=0` |
| Plain `git rebase <base>` conflicts in an untouched file after the base is amended | exit 1 with the base's own file listed by `--diff-filter=U` |
| `git rebase --onto <base> <old-merge-base>` handles the same case cleanly | exit 0, the branch commit preserved, the base's content intact |
| `git merge-base --fork-point` and the reflog divergence SHA both recover the old merge base | both returned the same SHA that `git merge-base` alone did not |
| `--force-with-lease` with a wrong expected SHA is rejected | `! [rejected] ... (stale info)` in one run and `! [rejected] ... (non-fast-forward)` in another, depending on whether Git could resolve the expected value |
| `range-diff` over the old and new ranges proves commit preservation | every row read `=` after a successful rebase |
| `origin/HEAD` may be absent and is restored by `git remote set-head origin --auto` | `symbolic-ref` exit 1 before, exit 0 after |
| Row 2's `for-each-ref --contains` finds a branch that took this branch's commits | printed nothing before the feature was merged into `qa`, then `qa` and `origin/qa` |
| Rebasing after another branch took the commits leaves two copies there | the rebased commit had a new hash and the same patch-id; merging it into `qa` exited 0 and `qa` then logged the change twice |
| Row 3's author check sees a collaborator's commit | `sort -u` printed both addresses after a commit made with another `user.email` |
| A plain rebase drops the branch's merge commits | `git rev-list --merges --count` was 1 before the rebase and 0 after |
| A merge keeps the pre-sync tip as an ancestor and brings only the base's files | both `--is-ancestor` checks exited 0; `diff --name-only <backup>..HEAD` equalled the base's own file list |
| The push after a merge needs no force flag | plain `git push origin feature` fast-forwarded and exited 0 |
| A plain push after someone else pushed is rejected, not overwritten | `(fetch first)` before fetching and `(non-fast-forward)` after |
| During a merge `:2:` is this branch and `:3:` is the base | `git show :2:c` printed the feature line and `:3:c` the release line; `git merge --abort` restored the branch tip with a clean tree |
| `glab mr view -F json` exposes `user_notes_count` and `target_branch`, and the approvals API exposes `approved_by` | read from an open MR on a GitLab project |
