# Readiness and next-state evaluation: I/O and cache evidence

Retrieval date: 2026-10-02. Scope: the current local IssueOps implementation and neighboring tests. Version context: `go.mod:1-3` declares module `issueops`, Go 1.26.3; this is a source requirement, not proof of the running toolchain. Release date/version for these readiness changes: not established by this checkout inspection; no release or anticipated-release claim is made.

All findings are first-party repository evidence. Neighboring tests corroborate intended contracts within the same source, not independently. No public-source claim or external URL is needed for this local lane. Tests were read, not executed; no benchmarks or broad verification runs were performed. Structural certainty is high; performance impact remains an unmeasured hypothesis.

## Findings

### 1. Branch/status reuse already exists, with operation-scoped freshness

`internal/adapter/issueops/readiness_git.go:16-53` memoizes only exact two-argument `branch --show-current` and `status --porcelain=v1` calls, keyed by literal root and arguments. It preserves exit code, stdout, and stderr, including failures. Production wiring shares this observer between readiness and cleanup (`cmd/issueops/issueopsapp/cycle_readiness_wiring.go:23-36`). `scoped()` clears the factory from the operation's copy, so nested calls reuse its scope (`internal/application/issueopscycle/readiness_service.go:26-31`).

Applicability: optimize remaining probes rather than introduce another global cache. The real-Git wrapper test expects each shared command once per operation (`cmd/issueops/issueopsapp/cycle_readiness_wiring_test.go:60-93`). Counterevidence to cross-request caching: the same-instance test explicitly expects changed HEAD and cleanup observations on the next call (`internal/application/issueopscycle/readiness_scope_test.go:70-88`). These are inspected assertions, not a passing run.

### 2. Next already shares its local change snapshot with review-tier classification

The next handler creates `nextLocalReadinessObservation` inside each request. Local readiness retains paths and their verification flag; review-tier observation returns a defensive copy only for the matching record key. That key includes repository, worktree, execution root, branch, and prepared base (`cmd/issueops/issueopsapp/issueops_next_wiring.go:35-118`). Thus this reuse is request-local, not persistent.

Applicability: preserve one observation for readiness and review visibility. Counterevidence to blindly retrying or trusting cached paths: tests require repository changes to fall back, prevent caller mutation, and retain an unverified snapshot without promoting it through another observation (`cmd/issueops/issueopsapp/issueops_next_observation_test.go:11-75`). Earlier phases without retained evidence use a fresh fallback (`:79-94`). Certainty: high.

### 3. Some next-state I/O precedes branches that do not need full readiness

`buildInput()` observes staged artifacts, worktree state, holder liveness, and phase-eligible local readiness before classification (`internal/application/issueopsnext/service.go:140-185`). The classifier can then return immediately for another live holder or a claimable/released lease (`internal/domain/issueopsnext/classify.go:115-133`). Local readiness therefore can run even when the eventual decision is a lease action.

There is also a precise repeated-probe candidate: repository detection calls `WorktreeState(cwd)` (`internal/application/issueopsnext/service.go:329-336`); execution observation calls it again for the workspace root. Each successful production call reads top-level, branch, and HEAD (`cmd/issueops/issueopsapp/issueops_next_wiring.go:147-153`). When cwd equals that root, these observations repeat outside the readiness cache.

Applicability: investigate lazy readiness and same-request worktree observation reuse. Counterevidence: roots can differ, and warning visibility uses worktree facts. Existing phase gating already limits local readiness to ai-slop-clean/feedback. Certainty: high for call ordering; no latency claim.

### 4. Filesystem readiness repeats checks outside the Git cache

Baseline PR readiness checks plan existence and containment (`internal/application/issueopscycle/pr_readiness.go:31-40`). Observed PR readiness obtains those facts again (`internal/application/issueopscycle/readiness_service.go:147-155`). The concrete adapter uses `os.Stat` for existence and, for absolute contained plans, `filepath.EvalSymlinks` on both paths (`internal/adapter/issueops/readinesspaths/paths.go:19-68`).

Applicability: consider operation-scoped path facts after counting actual calls. Counterevidence: relative paths skip symlink resolution; existence roots can differ; containment is a correctness boundary. A cache must not equate superficially similar paths or survive external filesystem changes. Certainty: high for repeated observer calls, conditional for identical underlying filesystem targets.

### 5. Strict readiness intentionally refreshes after fetch

Readiness clears Git observations around fetch before upstream counts (`internal/application/issueopscycle/readiness_service.go:134-148`). Strict schema checking re-observes changed paths, whereas local checking uses the supplied snapshot (`internal/application/issueopscycle/schema_readiness.go:21-28`). Tests specify event order for internal, callback, and prefetch modes (`internal/application/issueopscycle/readiness_fetch_scope_test.go:11-54`).

Applicability: any visibility counters should distinguish local reuse from strict refresh. Counterevidence to eliminating every repeated read: these repetitions encode a mutation boundary. Certainty: high; their runtime cost is unknown.

## EXPAND

- **EXPAND-MEASURE:** Count subprocesses, path probes, and elapsed observation stages for matched-root versus different-root next requests, including blocked leases.
- **EXPAND-DESIGN:** Evaluate lazy readiness while preserving warning and review-tier output.
- **EXPAND-SAFETY:** Evaluate request-local filesystem reuse with symlink, root-identity, fetch, and concurrent-edit cases. No optimization is justified before measurement.

Verification: cited key source ranges were re-read. Inaccessible sources: none. Failed commands: none. One tool-discovery batch failed because `tool.glob` was unavailable; supported searches and `find` replaced it. The requested report-only whitespace check is recorded in the completion response.
