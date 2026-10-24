---
name: 2026-10-25-release-an-issue-create-whose-caller-died
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Release an issue create whose caller died

- Date: 2026-10-25
- Kind: `adr`
- Source: autoresearch QA /tmp/qa-icrash
- Summary: reconcile-issue releases a pending issue-create intent (pending → not_invoked) for the same sealed create-issue when a settlement proves nothing was or will be created.
- Context: Fault-injection QA (2026-10-25) killed issueops inside remote create-issue --confirm before the issue existed. The intent stayed pending; reconcile-issue required exactly one candidate and create-issue refused a pending intent, so only abandon remained.
- Decision: Confirmed create-issue holds an issue-create lifetime lock (processlease keyed repo + sealed title, inherited by gh/glab; a missing state root refuses before invoking). reconcile-issue with zero search candidates (not truncated) for a pending intent asks a settlement: intent at least 2 minutes old, lifetime lock free, no gh/glab issue create carrying the operation ID and no issueops remote create-issue for the cycle in ps, and a provider listing (not search; newest first, 8 MiB output) that has no issue with the marker and reaches back before the attempt. The release is a compare-and-swap on the settled attempt (status pending, attempt, updated_at, operation ID). invoked_unknown and other statuses keep the refusal.
- Consequences: A crashed create-issue resumes with the same sealed request; next routes back to create-issue after the release. Remaining gaps: provider processes in another PID namespace or host; a marker removed from the issue body within the quiet period; an issue transferred out of the repository.
- Evidence:
  - internal/application/issueopsremote/issue_reconcile.go releaseCrashedCreate
  - IssueCreateIntents.ReleaseCrashed
  - internal/adapter/issueops/issue_create_settlement.go
  - port.IssueProviderRecentIssueLister (github/gitlab RecentIssuesLackMarker)
  - TestIssueReconcileReleasesASettledCrashedCreate (incl. racing attempt)
  - security-reviewer verdict accept-with-changes; required CAS, ASCII in-flight match, output limit and settlement error changes applied
- Alternatives / rejected options:
  - Keep refusing (dead end: abandon)
  - Trust the search API zero (eventually consistent; can hide a created issue)
  - Release invoked_unknown too (the provider answered ambiguously while the caller was alive; out of scope)
