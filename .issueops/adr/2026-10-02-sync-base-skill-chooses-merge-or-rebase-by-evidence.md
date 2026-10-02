---
name: 2026-10-02-sync-base-skill-chooses-merge-or-rebase-by-evidence
description: Accepted decision record with rationale, alternatives, and consequences.
---

# sync-base skill chooses merge or rebase by evidence

- Date: 2026-10-02
- Kind: `adr`
- Source: claude-code session: skill rename requested by user
- Summary: The rebase-onto-parent skill is renamed sync-base and now picks merge or rebase from a first-match decision table, defaulting to merge whenever the branch may be shared or the evidence is unclear.
- Context: Feature branches are cut from release branches that keep advancing, and the user catches them up by merge as often as by rebase. The skill only rebased, so its name described one mechanism and merge-based catch-up had no owner outside an IssueOps cycle. "Parent" also has no meaning in Git and collides with commit parents, while GitHub calls the branch the PR base and GitLab the MR target. The 2026-09-08 pipeline skill routing ADR names the skill by its former name.
- Decision: Rename skills/rebase-onto-parent to skills/sync-base and give it a merge mode. Step 3 walks a first-match table: an explicit user choice, then merge when another branch already contains this branch's commits, when another author committed on it, when an open PR or MR has review activity, or when it already contains a merge commit; rebase only when none match; merge when a check fails or is unclear. A merge keeps the backup ref, verifies that both the base and the pre-sync tip are ancestors of the new tip, and pushes without any force flag. The rebase path keeps its --onto selection, range-diff proof, and explicit-SHA lease. The record key branch.<branch>.parent and the legacy refs/backup/rebase-onto-parent namespace stay readable. IssueOps-owned branches still route to issueops execution sync-base.
- Consequences: A branch synced once by merge stays on merge through the merge-commit row. Users invoke /sync-base instead of /rebase-onto-parent after issueops update relinks the host skill paths. Historical ADR and plan text keeps the former name.
- Evidence:
  - skills/sync-base/SKILL.md Step 3 decision table and Verified facts rows reproduced in a scratch repository
  - Scratch repo: for-each-ref --contains printed qa and origin/qa only after the feature was merged into qa; rebasing then left the change twice in qa
  - Scratch repo: rev-list --merges --count was 1 before a plain rebase and 0 after
  - Scratch repo: during a merge :2: was the feature side and :3: the release side; git merge --abort restored a clean tree
  - glab mr view -F json exposed user_notes_count and target_branch on an open MR; the approvals API exposed approved_by
- Alternatives / rejected options:
  - Keep the name and leave merge to git-operations: catch-up by merge would stay ownerless and the name would keep naming a mechanism instead of the intent.
  - Name it catch-up-base: accurate intent, but sync-base matches the existing issueops execution sync-base verb now that both cover the same intent.
  - Default to rebase when evidence is unclear: a wrong rebase breaks collaborators' branches and review anchors, while a wrong merge only costs a less linear history.
