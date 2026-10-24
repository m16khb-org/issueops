---
name: 2026-10-25-release-a-child-create-whose-caller-died
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Release a child create whose caller died

- Date: 2026-10-25
- Kind: `adr`
- Source: autoresearch QA /tmp/qa-chcrash
- Summary: reconcile-child releases a pending child-create operation with no URL and no candidate to not_invoked when the issue-create settlement proves nothing was or will be created, and the same create-child re-runs with its operation ID.
- Context: Fault-injection QA (2026-10-25) killed issueops inside remote create-child --confirm (gh issue create --parent) before the child existed. The operation stayed pending; reconcile-child required exactly one candidate and create-child refused unresolved operations, so only abandon remained.
- Decision: ChildReconciler gains Settled (adapter.ChildCreateCrashSettlement = the issue-create settlement with a GitLab task listing). With zero candidates, no canonical URL, and status pending, a settled preview returns ok with the --confirm recovery command; confirm releases through ChildCreateIntents.ReleaseCrashed (CAS on status, attempt, updated_at, no URL, recovery authority) after rebinding the operation to the current holder. Confirmed create-child holds the issue-create lifetime lock through LockedChildCreator (no context reaches the provider, so orphaned provider children are covered by the process scan, which now matches any gh/glab write carrying the operation ID, including glab api graphql workItemCreate, and skips reads). A restarted attempt refreshes StartedAt/UpdatedAt so its quiet period starts anew.
- Consequences: A crashed create-child resumes with the same operation ID. Remaining gaps are those of the issue-create settlement (other PID namespaces/hosts, marker edited away, transferred issues).
- Evidence:
  - internal/application/issueopsremote/child_reconcile.go releaseCrashedChild
  - ChildCreateIntents.ReleaseCrashed
  - internal/adapter/issueops/issue_create_settlement.go ChildCreateCrashSettlement, LockedChildCreator, issueCreateInFlight
  - TestChildReconcileReleasesASettledCrashedCreate
  - security-reviewer accept-with-changes: GitLab graphql in-flight match, restart timestamps, rebind before release, task listing - applied
- Alternatives / rejected options:
  - Keep refusing (dead end: abandon)
  - Search-only zero (eventually consistent)
