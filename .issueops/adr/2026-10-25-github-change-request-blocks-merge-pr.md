---
name: 2026-10-25-github-change-request-blocks-merge-pr
description: Accepted decision record with rationale, alternatives, and consequences.
---

# GitHub change request blocks merge-pr

- Date: 2026-10-25
- Kind: `adr`
- Source: autoresearch QA /tmp/qa-fb
- Summary: remote merge-pr treats a standing GitHub CHANGES_REQUESTED review decision as merge_blocked, matching GitLab's requested_changes status.
- Context: In a feedback QA run (2026-10-25) a human reviewer requested changes on a completed cycle's PR. The agent addressed them through a reseed and merged; merge-pr preview showed no blocker because GitHub without a protection rule merges over a change request, while the GitLab adapter already blocks requested_changes.
- Decision: The GitHub merge state reads reviewDecision with the other merge fields in one gh pr view. CHANGES_REQUESTED sets Blocked, which the domain maps to the existing merge_blocked code with the reason 'a reviewer requested changes; ask for a re-review or a dismissal'. The merge skill's merge_blocked row names the code-defect reseed route for addressing the comments and says the reviewer must re-approve or dismiss.
- Consequences: An agent cannot merge over a standing change request; a human re-approves or dismisses on the provider. REVIEW_REQUIRED without protection still does not block (GitHub's own BLOCKED status covers protected branches).
- Evidence:
  - internal/adapter/provider/github/merge_pull_request.go reviewDecision
  - TestParseGhMergeStateBlocksOnChangesRequested
  - internal/adapter/provider/gitlab/merge_merge_request.go requested_changes
  - commit d1e594e8
- Alternatives / rejected options:
  - Warning only: rejected, agents followed exact next_command merge-pr over the change request
  - Let the user's merge approval override: rejected, a reviewer's change request is a team gate the session cannot clear
