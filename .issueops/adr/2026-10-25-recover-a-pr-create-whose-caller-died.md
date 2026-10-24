---
name: 2026-10-25-recover-a-pr-create-whose-caller-died
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Recover a PR create whose caller died

- Date: 2026-10-25
- Kind: `adr`
- Source: autoresearch QA /tmp/qa-crash
- Summary: A pending PR/MR create intent passes through the takeover chain to the next holder, and reconcile may take its single retry when an authoritative empty candidate list is backed by crash settlement.
- Context: Fault-injection QA (2026-10-25) killed issueops inside remote create-pr --confirm before the PR existed. Reconcile refused ('authoritative zero cannot clear an invocation whose absence was not proven'), reconcile needs the live holder, and execution replace refused revoke/finalize while an intent was pending, so only abandon remained.
- Decision: execution replace revoke/finalize no longer refuse a pending intent; the intent stays on the record and the next holder reconciles after claim (finalize still requires the old holder dead; with the workspace missing it stays revoking because a released lease cannot reconcile). next routes a pending intent with a dead other holder to takeover and a claimable one to claim. Reconcile gains a CrashSettled fact: with an authoritative empty list, invocation unknown, no known URL, and RetryCount 0, it takes the existing single retry when the create started at least 2 minutes ago (twice the provider mutation timeout) and the local process table shows no gh/glab create for the head and no issueops create-pr for the cycle. Settlement errors fail closed.
- Consequences: A crashed create recovers through takeover, claim, and reconcile without abandoning the cycle (smoke: PR created, next pr.complete). Remaining blind spots: provider processes in another PID namespace or host are not visible to the settlement (the same limit already applies to the finalize holder check); a renamed head branch could hide an existing PR from the list.
- Evidence:
  - internal/application/issueopsreplacement/service.go
  - internal/domain/issueopspublication/decision.go CrashSettled
  - internal/adapter/issueops/publication_settlement.go
  - TestReconcileCrashSettlementUnlocksTheSingleRetry
  - TestFinalizeCarriesAPendingIntentToTheNextHolder
  - TestFinalizeWithAPendingIntentKeepsAMissingWorkspaceRevoking
  - security-reviewer verdict accept-with-changes; changes applied
- Alternatives / rejected options:
  - Keep refusing (dead end: abandon the finished work)
  - Clear the intent without retrying and let the agent re-run create-pr (loses the retry bound)
  - Process-lease (flock) settlement keyed by operation ID (stronger, larger change; recommended follow-up by the security review)
