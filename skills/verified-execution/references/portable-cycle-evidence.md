# Portable Cycle Evidence

Moved from SKILL.md "Portable Cycle Evidence". Read when recording domain-change, API, runtime-matrix, feedback, or ready/done evidence in a cycle.

## Portable Cycle Evidence

- Before domain changes record invariant, exact mechanism, equivalent behavior,
  and current source/command evidence. Report missing documented mechanism
  separately from whether another path enforces the invariant.
- API changes require plan/draft evidence of changed endpoints (or none), public
  errors reachable through business logic, available static checks, agent review
  for visible errors, and targeted commands. Prefer repo commands; if absent,
  record that and use the nearest check, without blaming unrelated pre-existing debt.
- Runtime/environment differences need a matrix: Environment | Repo/config
  evidence | Runtime evidence | Failure path | Remediation order. Separate source
  from live DB/config/env/pod/log observations and similar-looking failure paths;
  distinguish local-only probes from same-network/workload proof.
- For feedback record classification (contract_change, defect, question, noise,
  valid_review, stale_review, rollout_evidence_missing, environment_debt), validity
  evidence, original-thread reply status, and resolution (unresolved, fixed,
  resolved, obsolete, follow-up). Apply only verified feedback. Scope/criteria/
  non-goals/verification/labels/link changes require owning-stage issue-body update
  before continuing, within remote-write authorization.
- Before ready/done record actual-worktree diff, source/target branches, issue/PR
  body freshness, copied or explicitly replaced labels, single-commit policy or
  reason for multiple commits, divergence/cleanliness, cleanup status or numbered
  choices. Prepare an inspectable draft completion record before remote writing
  and final reporting: diff, verification, labels, linked children, PR/MR URL,
  review-thread status, cleanup and unresolved follow-ups. Tests alone do not
  satisfy requested remote updates, replies, merge readiness, or cleanup.
