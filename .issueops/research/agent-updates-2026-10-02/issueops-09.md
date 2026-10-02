# IssueOps self-verification: existing latency evidence

Retrieval date: **2026-10-02**. Scope: implementation and neighboring tests, not a new benchmark. Model reported by session environment: `gpt-6.1-sol`.

Version context: current summary contract is **v6** (`internal/domain/selfverify/contract.go:10-14`); persisted summary schema remains **v1** (`internal/domain/selfaugment/summary_snapshot.go:11-24`). Their release dates are not established by these declarations. The repository declares Go **1.26.3** (`go.mod:1-3`), released **2026-05-07** according to https://go.dev/doc/devel/release. This declaration does not establish the installed toolchain version.

## Findings

### 1. Self-verification is a sequential evidence pass, not repeated sampling

`internal/application/selfverify/loop.go:65-78,98-123` fixes iterations to one, invokes each planned closure sequentially, records elapsed milliseconds, and stops after failure unless collect-all is enabled. This bounds duplicate work but also means later labels disappear from a fail-fast result.

**Certainty:** high for control flow; no runtime latency measured. **Applicability:** compare equivalent completed runs, scopes, and modes before proposing scheduling changes. **Counterevidence:** sequential top-level execution does not prove every child operation is serial; nor does a large timeout prove a slow step. The neighboring timeout test asserts a ten-minute allowance, not ten minutes of observed execution (`cmd/issueops/selfworkflow/steps/self_verify_steps_test.go:121-140`).

### 2. Existing deduplication already avoids redundant suite execution

`internal/application/selfverify/steps.go:74-95` reuses successful full-suite risk-QA coverage instead of invoking ordinary `go test`; successful ordinary-suite evidence then covers the golden step. Neighboring tests explicitly assert zero ordinary-suite calls after successful race coverage and identify golden reuse (`cmd/issueops/selfworkflow/steps/self_verify_steps_test.go:59-102`).

The ordinary command uses `-count=1`. Official Go 1.10 notes explain that this bypasses test-result caching: https://go.dev/doc/go1.10, “Test.” Go 1.10 was released **2018-02-16** (https://go.dev/doc/devel/release).

**Certainty:** high for implemented reuse. **Applicability:** attribute reused zero-duration labels to the originating evidence rather than treating them as faster executions. **Counterevidence:** `-count=1` does not establish a cold compilation cache; reuse requires successful full-suite coverage. No savings magnitude is measured here. The Go references are official single-publisher evidence, not independent corroboration.

### 3. Per-label budgets exist, but their fixtures are not benchmarks

`internal/domain/selfaugment/summary.go:29-66` collects all label durations while limiting `slowest_steps` to five. `internal/domain/selfaugment/step_stats.go:34-59` computes count, minimum, maximum, mean, and nearest-rank p95. With one observation per label in the normal single pass, p95 equals that observation; it is not an empirical tail distribution across repeated runs.

`internal/domain/selfaugment/history_regressions.go:45-74` compares p95 values, excluding increases below the 25-ms floor declared at line 9 and increases at or below the supplied percentage threshold. Tests inject docs-smoke values of 100 and 130 ms and verify a regression outside the slowest list; another injects 76 and 83 ms and expects noise suppression (`cmd/issueops/selfworkflow/historycompare/self_augment_compare_test.go:158-229`).

**Certainty:** high. **Applicability:** preserve per-label coverage and sample counts when designing visibility. **Counterevidence:** these hand-authored values prove comparator behavior, not actual docs or state latency. Legacy summaries fall back to slowest-only observations (`internal/domain/selfaugment/step_stats.go:62-74`).

### 4. Four successful probes can discard the latency they just measured

`internal/adapter/verification/probe/contractauditworker/validation_contract_check.go:16-47` executes a subprocess, then returns an assertion using `time.Since(time.Now()).Milliseconds()`. That expression starts timing at the return point, excluding the subprocess and parsing work. Failed subprocess results are returned directly.

Final Fable review and the lead's direct code reads confirmed the same successful-path pattern in four files in this package: `validation_contract_check.go:47`, `validation_tool_conformance.go:47`, `validation_worker_lifecycle.go:46`, and `validation_command_audit.go:42`. The affected labels are contract check, tool contract conformance, worker lifecycle smoke, and command audit smoke. This is a four-site accounting issue, not evidence that the commands themselves run slowly.

The neighboring success fixture supplies command output but no duration and asserts only success (`internal/adapter/verification/probe/contractauditworker/validation_contract_audit_worker_test.go:133-149`).

**Certainty:** high for the accounting defect; impact unmeasured. **Applicability:** prioritize duration propagation or a start-to-finish clock contract before optimizing this probe. **Counterevidence:** failure passthrough retains the child result; this is not evidence that the underlying command is slow.

### 5. Progress events already expose useful failure context

`cmd/issueops/selfworkflow/progress/self_verify_progress.go:46-70` emits step duration, position, success, last successful label, and error. Its neighboring test verifies that a build failure preserves the preceding successful test label (`cmd/issueops/selfworkflow/progress/self_verify_progress_step_test.go:13-28`).

**Certainty:** high for output construction, not live delivery. **Applicability:** consume existing JSONL before adding another telemetry system. **Counterevidence:** these tests do not establish streaming latency, and events inherit inaccurate probe durations.

## EXPAND

- **EXPAND-CORRECTNESS:** Add deterministic duration-preservation tests around successful contract/audit/worker probes; assess clock propagation separately from performance.
- **EXPAND-MEASUREMENT:** In an authorized later lane, capture equivalent complete runs with revision, toolchain, scope, cache conditions, and sample counts. No optimization benefit is established here.
- **EXPAND-VISIBILITY:** Distinguish reused evidence, missing fail-fast labels, and single-observation “p95” in presentation.

Source independence: local implementation and tests are same-repository corroboration, not independent measurements. Public citations were re-opened; cited code ranges were re-read. No inaccessible public source. One failed inspection: guessed path `internal/contract/selfaugment/self_augment_summary_types.go` returned ENOENT; actual version declarations above were read instead. No tests, builds, benchmarks, installations, or external mutations ran.
