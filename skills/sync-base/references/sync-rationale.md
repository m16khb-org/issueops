# Sync Base Background

Optional background only. `../SKILL.md` owns all execution gates and decisions.

## Why the decision table prefers merge

- Row 2: rebasing commits already taken by another branch creates new hashes for
  the same changes. Later merges may duplicate history and conflict on edits.
- Row 3: collaborators still have the old commits; rebase forces manual recovery.
- Row 4: review comments anchor to commits; rewriting detaches review context.
- Row 5: plain rebase drops merge commits and may repeat prior resolutions.
- Unclear evidence: a wrong merge costs linearity; a wrong rebase disrupts
  someone else's branch or review.

## Previously Recorded Observations

Moved from the original skill's “Verified facts” section, not rerun by the
contract refactor. The original recorded these as scratch-repository reproductions.

| Claim | Recorded evidence |
|---|---|
| `git switch -c` records `branch: Created from HEAD`, not the base name | new branch reflog |
| Divergence lookup distinguishes immediate base from older ancestor | base merge-base equalled fork with base-ahead=2; main was earlier with base-ahead=0 |
| Plain rebase after base amendment can conflict in an untouched file | exit 1; base-owned file in `--diff-filter=U` |
| `--onto <base> <old-merge-base>` handles that case | exit 0; branch commit and base content preserved |
| Fork-point and creation reflog can recover the old divergence | same SHA; ordinary merge-base returned an older SHA |
| Wrong expected lease SHA rejects push | stale-info or non-fast-forward rejection, depending on object resolution |
| Correct old/new `range-diff` proves patch preservation | all rows `=` |
| `origin/HEAD` can be missing | symbolic-ref exit 1 before set-head --auto, exit 0 after |
| `--contains` finds dependent branches | empty before merge into qa; qa and origin/qa afterward |
| Rebase after another branch took commits duplicates history | new SHA, same patch-id; qa logged the change twice after merge |
| Author check finds collaborator commits | two addresses from sort -u |
| Plain rebase drops merge commits | merge count 1 before, 0 afterward |
| Merge preserves branch tip and brings base files | both ancestor checks exit 0; file lists match |
| Merge push is unforced | plain push fast-forwarded, exit 0 |
| Concurrent remote push is protected | fetch-first before fetch; non-fast-forward afterward |
| Merge stages 2/3 mean branch/base | stage reads printed feature/release; abort restored clean original tip |
| GitLab review/base fields are available | open MR returned user_notes_count, target_branch, approved_by |
