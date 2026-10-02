# Omo: persistent Bun eval and tool composition

Retrieval date: **2026-10-02**. Question: which implementation facts support IssueOps performance, optimization, and visibility improvements?

## Method and source index

Source fan-out: package provenance, installed implementation, tagged official documentation, and official release notes. Local source and public documentation corroborate mechanisms but are not independent authorities. No independent benchmark was established; no performance percentage is claimed.

Exact local anchor prefix `K` means `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d/node_modules/@code-yeongyu/senpi-codemode`. Installed provenance: `/Users/habin/node_modules/omo-ai/package.json:1-26` identifies `omo-ai` **5.1.8**, canonical repository `code-yeongyu/oh-my-openagent`, and Senpi dependency **2026.10.1-2**. `K/package.json:50-54` identifies the eval implementation's canonical repository as `code-yeongyu/senpi`, directory `packages/senpi-codemode`.

Public sources, all official and fetched on the retrieval date:

- [D: tagged implementation documentation](https://raw.githubusercontent.com/code-yeongyu/senpi/v2026.10.1-2/packages/senpi-codemode/README.md).
- [R1: Senpi v2026.7.26](https://github.com/code-yeongyu/senpi/releases/tag/v2026.7.26).
- [R2: Senpi v2026.10.1-3](https://github.com/code-yeongyu/senpi/releases/tag/v2026.10.1-3), published **2026-10-01T22:13:05Z**, per [official release API](https://api.github.com/repos/code-yeongyu/senpi/releases/latest).
- [R3: Omo v5.1.9](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.9), published **2026-10-02T01:13:10Z**, per [official release API](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/latest).

## Findings

### 1. Persistence is a Senpi facility, not an Omo-only Bun feature

D documents one persistent kernel per enabled language, surviving cells until reset, restart, or disposal; JavaScript follows the host runtime, Bun or Node. `K/src/kernels/js/worker-host.ts:1-36` constructs a worker through `node:worker_threads`. R1 explicitly removes GPT-only `exec`/`wait` in favor of persistent `eval` composition. The inspected eval returned `Bun.version = 1.4.2`; this establishes the current runtime, not Bun's public release date.

**Certainty:** high for installed behavior and the official migration statement. **Applicability:** reuse clients and parsed evidence between related read-only cells rather than rebuilding state. **Counterevidence:** Node is supported too; persistence ends at recovery boundaries and does not establish durable storage or cross-host parity.

### 2. Parallel composition is bounded, ordered, and not fail-fast

`K/src/kernels/js/worker-runtime.js:358-398` converts thunks to an array, limits concurrent workers to configured width, stores results by input index, waits for all workers, then throws the lowest-index error. `pipeline` awaits each parallel stage before the next. The built-in width is **4**, not automatically the workstation's eight cores (`K/src/config/settings.ts:148-159`).

**Certainty:** high, directly read code. **Applicability:** batch independent reads and probes; catch failures inside each thunk when every result must remain inspectable. **Counterevidence:** a rejected thunk does not cancel remaining work. Batching writes could therefore continue side effects after failure; no throughput gain was measured.

### 3. Detachment does not provide same-language execution concurrency

`K/src/kernels/js/context-manager.ts:194-211,311-330` starts a cell only without an active run, advances after settlement, and recycles an over-ceiling worker only after running and queued cells drain. D distinguishes the default **30-second** interactive detach threshold, **60-second** foreground window, **300-second** own-execution budget, **1,800-second** submission-based hard limit, and **15** detached-cell capacity. `K/src/kernels/js/worker-runtime.js:341-355` brackets host calls with timeout pause/resume events.

**Certainty:** high for implementation/defaults; effective session overrides were not inspected. **Applicability:** separate queue delay, host-call waiting, and actual execution before diagnosing slow checks. **Counterevidence:** detached cells still serialize within their language; timeout recovery can discard globals (`K/src/kernels/js/context-manager.ts:224-260`).

### 4. Useful visibility already exists as bounded structured telemetry

`K/src/tool/eval-execution-event.ts:12-61,76-147` defines version-1 `senpi.eval.execution`, wall duration, kernel duration, `queued_ms`, detached status, call counts, pending counts, and per-tool aggregates. RPC projection excludes argument/result previews and reduces oversized detail. `K/src/tool/call-capture.ts:4-14` caps enriched calls at **30**, tool aggregates at **64**, and RPC events at **32 KiB**.

**Certainty:** high for payload construction, corroborated by D. **Applicability:** evaluate consuming metadata in a thin Omo adapter before inventing another profiler. IssueOps's Go-core/thin-adapter boundary remains authoritative (`.issueops/ARCHITECTURE.md:12-16`). **Counterevidence:** aggregated tool durations overlap under parallel execution and are not additive cell elapsed time; capped detail is not a complete trace.

### 5. Recent fixes reveal stale-session hazards

R2 says eval bash approvals now route to the submitting RPC client rather than the kernel's original connection, including detached calls; deferred tools survive hot reload. R3 incorporates Senpi 2026.10.1-3 and describes the approval hang fix.

**Certainty:** official single-source release claims, repeated by the same project's wrapper release, not independently reproduced. **Applicability:** include submitting-client identity and extension generation in future visibility tests. **Counterevidence:** the inspected installation is 2026.10.1-2/5.1.8; newer release fixes cannot be attributed to it.

## Access boundary and EXPAND

The historical release API `https://api.github.com/repos/code-yeongyu/senpi/releases/tags/v2026.7.26` returned an API-rate-limit error; its public release page remained accessible. Both `/releases/latest` URLs yielded publication timestamps initially but returned rate-limit errors on recheck; timestamps above retain that first-fetch evidence. D and R1-R3 were successfully reopened, and cited local code was reread. No auth files or transcripts were read. No shell command failed during investigation. Diagnostics, tests, and builds are inapplicable to this evidence-only Markdown change; manual citation and word-budget checks apply.

**EXPAND-PERFORMANCE:** benchmark sequential versus bounded parallel read-only probes, recording queue, wall, and tool durations; no benefit is presumed.

**EXPAND-VISIBILITY:** verify actual RPC event delivery, truncation, and pending-call interpretation with a controlled consumer.

**EXPAND-LIFECYCLE:** reproduce detached approval routing and reload behavior on separately authorized 5.1.9/2026.10.1-3 installations. These remain actionable because this lane inspected source and release evidence, not external-client integration.
