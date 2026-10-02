# Omo DAG execution, recovery, and persistence

Retrieval date: **2026-10-02**. Model: `gpt-6.1-sol` (`PI_MODEL`).

## Provenance and method

The canonical project is **code-yeongyu/oh-my-openagent**, established by `/Users/habin/node_modules/omo-ai/package.json:2-24`: installed `omo-ai` is **5.1.8**, depending on senpi **2026.10.1-2**, with that repository URL. Official metadata dates [5.1.8](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.8) to **2026-10-01 15:00:33 UTC** and [5.1.9](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.9) to **2026-10-02 01:13:10 UTC**. These are actual releases, not forecasts.

Source fan-out: official release metadata/notes, release-tagged implementation, installed SDK, and IssueOps architecture. Claim verification: implementation facts below have high source-reading certainty; all public evidence is **official single-source**, not independently corroborated or runtime-benchmarked. Local package/SDK agreement is another artifact from the same producer, not source independence. Release-tagged links avoid conflating development-branch documentation with installed behavior.

## Findings

### 1. Dependency frontiers replace execution barriers

The [5.1.8 scheduler](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/scheduler.ts), lines 530-605 and 927-931, admits nodes when every dependency has completed; wave events are grouping metadata. An unrelated slow sibling does not impose a wave barrier. Residency-denied nodes receive FIFO preference, and a session-wide residency-change signal is armed **before** admission probes. This avoids missing capacity released during admission.

Applicability: investigate dependency-ready admission and capacity-wait visibility rather than increasing concurrency blindly. Counterevidence: session-wide resident limits still constrain admission; failed dependencies can produce skipped descendants. The [manager](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/manager.ts), lines 46-53, records the barrier-to-frontier change as **2026-08-25**, a source-comment date, not a verified release date. No throughput percentage is established.

### 2. Retry preserves the run but creates fresh execution identity

[Node retry](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/node-retry.ts), lines 16-119 and 153-188, permits failed, cancelled, or skipped nodes, refuses completed nodes, increments `execAttempt`, and restores cascaded skipped dependents. Re-entry creates a **new scheduler**, dropping previous `preAttachedTasks`; spent cancellation/admission latches are not reused. Prompt overrides require exactly one explicit node.

Applicability: recovery controls should expose the retained run ID, new execution attempt, and descendant restoration. Counterevidence: retry refuses active pending/running runs; a skipped node cannot be retried alone while its blocking ancestors remain failed. Installed `/Users/habin/node_modules/omo-ai/plugin/runtime/dag/sdk.js:121-150` confirms the wrapper forwards these controls, but does not independently prove engine correctness.

### 3. Amendment is selective invalidation, not unrestricted live editing

The manager, lines 453-578 and 641-672, compares node fingerprints and invalidates changed/added nodes plus transitive dependents while preserving unaffected nodes. It journals amendment history and updates the definition fingerprint. It refuses run-key changes, scheduled/running-node edits, running runs, and paused runs with live leases.

Applicability: report affected and reused node IDs before recovery, preserving completed independent work. Counterevidence: lines 166-168 explicitly exclude skill-only changes from fingerprint invalidation; changing `load_skills` alone does not force re-execution. A caller must not equate any definition edit with a fresh result.

### 4. Persistence is durable but deliberately conservative about lost launches

[Journal](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/journal.ts), lines 95-125, serializes under a run lock, appends sequenced WAL events before checkpoint replacement, and stages completed results before the WAL. [Store](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/store.ts), lines 153, 368-395, and 419-455, enables fsync by default, uses temporary-file rename, and discards malformed trailing fragments rather than accepting corrupt records.

Applicability: distinguish checkpoint, journal, result, and launch evidence in diagnostics. Counterevidence: Windows skips parent-directory fsync. [Recovery](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/recovery.ts), lines 35 and 344-365, automatically readmits only scheduled lost tasks lacking `started_at`, capped at three attempts. Stamped lost work becomes `task_lost`; this is not an exactly-once external-effects guarantee.

### 5. Visibility has concrete parse-cost optimization and a newer retry-settings fix

Manager lines 236-321 cache summaries using inode, size, and modification time while retaining authoritative directory membership and evicting vanished entries. Applicability: measure repeated state parsing before adding more refresh polling. Counterevidence: directory scans, stat calls, and sorting remain; producer comments are not independent benchmarks.

The fetched [5.1.9 release notes](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.9) state that in-process children now inherit timeout/retry settings and name senpi **2026.10.1-3**. Certainty: high that this official claim was published, not locally execution-verified. Applicability: check settings inheritance when diagnosing stalled DAG children. Counterevidence: installed 5.1.8 does not establish that fix locally, and provider retries are distinct from DAG node retries.

## EXPAND

- **EXPAND: actionable / performance** — measure frontier admission latency, capacity waits, and summary-cache hit/miss costs on representative IssueOps workloads.
- **EXPAND: actionable / correctness** — validate amendment reuse, crash-after-launch behavior, and inherited retry settings in an authorized disposable integration lane.
- **EXPAND: actionable / visibility** — evaluate run/node/attempt and invalidation fields through host-neutral contracts. `.issueops/ARCHITECTURE.md:11-15,59-67` places shared logic in Go and state in SQLite; do not copy Omo filesystem persistence wholesale.

## Access and verification boundaries

No auth files, transcripts, installs, or external mutations were used. Inaccessible source: guessed official `packages/senpi-task/src/dag/amend.ts` returned **404**; actual amendment code is in `manager.ts`. Failed command: runtime `find .../node_modules/@oh-my-opencode` exited **1**, directory absent. API directory JSON parsing also failed because webfetch truncated its response; targeted raw files supplied complete evidence instead. Tests, build, and LSP diagnostics are not applicable to this prose-only evidence lane; validation is source rereading, manual report review, and the requested scoped diff check.

Verification: all nine cited public URLs were reopened; local anchors were reread. `git diff --check -- .issueops/research/agent-updates-2026-10-02/omo-02.md` exited **0**. Additional `git diff --no-index --check -- /dev/null <report>` exited **1**, with no whitespace diagnostics, comparing the untracked report against an empty file; it is not claimed as a passing command.
