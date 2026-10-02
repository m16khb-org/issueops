# Execution leases, resume cancellation, and orchestration boundaries

Retrieval date: **2026-10-02**. Model: **gpt-6.1-sol**, reported by `PI_MODEL`. Scope: current local implementation and neighboring tests, plus official Go documentation. Tests were inspected, not executed; no benchmarks, installation, external mutations, or production edits were performed.

Version context: `go.mod:1-14` declares Go **1.26.3** and modernc SQLite **1.53.0**. These are checkout requirements, not verified installed versions or IssueOps release dates. `WithoutCancel` arrived in **Go 1.21**, per https://go.dev/doc/go1.21#context; the official announcement at https://go.dev/blog/go1.21 is dated **2023-08-08** in its HTML. No IssueOps release date is inferred from fixture timestamps. The Go sources are one publisher, not independent corroboration; local implementation and tests are same-repository evidence.

## Findings

### 1. Lease release and owner launch resume are different transitions

**High certainty; directly applicable to recovery visibility.** Release validates the active generation, exact native actor, and canonical CWD (`internal/domain/issueopslease/release.go:57-85`). The application then clears holder and claim-token digest and sets `released` (`internal/application/issueopslease/release.go:36-57`). Conversely, Orca resume requires a holderless **claimable** lease with a token digest, matching generation, canonical CWD, and no pending intent (`internal/domain/issueopslease/resume.go:83-95`). Therefore releasing ownership is not sufficient to make this resume operation admissible. A terminal handle is not lease authority.

**Counterevidence:** runtime-rollover validation accepts holderless released *or* claimable states (`internal/domain/issueopslease/resume.go:63-81`), but the enclosing `PlanResume` remains stricter. Applicability: status/help should distinguish ownership release, preparation of a claimable generation, and resumption of owner launch rather than presenting them as interchangeable.

### 2. Resume avoids duplicate launch and bounds ambiguous retries

**High certainty; directly applicable to orchestration correctness.** A live same-generation task and terminal yield `existing_binding`; a live prior-generation task is denied. Matching idle terminals can be reused; settled ghost terminals require complete inventory (`internal/domain/issueopslease/resume.go:104-140`). The application returns an existing binding before allocating an operation ID (`internal/application/issueopslease/resume.go:79-90`); neighboring tests explicitly assert zero new launch operations (`internal/application/issueopslease/resume_test.go:49-87`).

Launch proceeds through at most five stage iterations (`internal/application/issueopslease/resume.go:91-136`). Stage planning adopts a unique candidate, rejects multiple candidates, and requires proven non-invocation or exact replay before invoking; attempts are capped at two (`internal/domain/issueopslease/resume.go:189-212`). **Counterevidence:** exact replay intentionally permits invocation despite unknown prior outcome; this is not a blanket prohibition on retries. Application ordering tests cover terminal/run/bind/task/dispatch (`internal/application/issueopslease/resume_test.go:92-139`). Preserve these distinctions in any optimization; parallelizing dependent stages would change the contract.

### 3. Cancellation can obscure resume failure diagnostics

**High certainty about code; medium certainty about operational impact.** Resume attempts `RecordFailure` using the same cancellable fence context and discards its error (`internal/application/issueopslease/resume.go:99-132`). Failure recording uses context-aware raw CAS (`internal/adapter/outbound/issueopslease/orca_failure.go:66-79`); SQL writer acquisition and `BeginTx` receive that context (`internal/adapter/outbound/sqlstore/sqlstore.go:747-780`). Thus cancellation can prevent the diagnostic write after an external effect. This is a reason to reproduce the boundary, not proof of duplicate execution or lost intent.

**Counterevidence:** intent and invocation markers precede invocation, preserving reconciliation evidence; the repository's failure test verifies redaction, request IDs, and stale-snapshot rejection, but uses `context.Background()` (`internal/adapter/outbound/issueopslease/resume_repository_test.go:195-227`). Cleanup already detaches finalization from cancellation and drains before recording failure (`internal/application/issueopscleanup/abandon_executor.go:96-124`), with a cancellation regression (`internal/adapter/issueops/cleanup_abandon_ownership_test.go:89-121`). Official https://pkg.go.dev/context#WithoutCancel says detachment also removes the deadline and Done signal: any proposed finalization must have a separate bounded timeout.

### 4. Per-cycle exclusion and raw CAS are separate boundaries

**High certainty; optimization benefit unmeasured.** Production resume wires a lifecycle fence and a separate main-state repository (`cmd/issueops/issueopsapp/issueops_resume_wiring.go:36-56`). The fence hashes lifecycle ID into a dedicated root (`internal/adapter/outbound/issueopslease/reseed_fence.go:32-38`); its test demonstrates distinct cycles entering concurrently (`internal/adapter/outbound/issueopslease/reseed_fence_test.go:12-32`). The entire resume callback, including external inventory and invocation, runs inside that fence (`internal/application/issueopslease/resume.go:52-132`).

**Counterevidence:** shared record storage still serializes writer acquisition; per-cycle fencing does not imply unlimited throughput. Existing span observation exposes wait, hold, contention, and outcome (`internal/adapter/outbound/sqlstore/sqlstore.go:347-373`). Applicability: measure these boundaries before shortening locks. Cleanup-finish exclusion is independently asserted against resume/reconcile intent writers (`internal/adapter/outbound/issueopslease/finish_intent_fence_test.go:16-70`); retain it.

## EXPAND

- **EXPAND-CORRECTNESS:** event-driven cancellation reproduction after invocation but before failure CAS; inspect pending intent, invocation marker, diagnostic receipt, and reconciliation outcome.
- **EXPAND-VISIBILITY:** expose lifecycle/generation, stage, invocation attempts, disposition, and failure-recording outcome without token values.
- **EXPAND-MEASUREMENT:** instrument existing span observers and stage durations before proposing lock reduction. No performance percentage is supported.

Source access: all cited public URLs accessible. Tooling failures: one search skipped nonexistent `internal/adapter/sqlstore`, corrected to `internal/adapter/outbound/sqlstore`; one eval raised `lines is not defined`, recovered with a persistent reader. No test or benchmark result is claimed.

Verification: cited code ranges and public URLs were reopened. `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-08.md` exited **0**. Because the report is untracked, an additional `git diff --no-index --check /dev/null .issueops/research/agent-updates-2026-10-02/issueops-08.md` returned **1**, with empty stdout/stderr; it detected a new-file difference without whitespace diagnostics. An instruction-file probe, `ls .issueops/AGENTS.md .issueops/research/AGENTS.md`, found neither file. Diagnostics, tests, and builds were not run for this bounded evidence-only lane.
