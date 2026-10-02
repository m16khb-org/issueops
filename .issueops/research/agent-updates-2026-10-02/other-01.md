# Gemini CLI: checkpointing, telemetry, and recent releases

**Retrieved:** 2026-10-02
**Scope:** Public Gemini CLI release notes and documentation, checked against the current issueops repository. This is one bounded evidence lane, not an implementation or a performance benchmark.

## Release snapshot

The official GitHub Releases page showed `v0.64.0-nightly.20261002.gc9096a847` as a prerelease on the retrieval date. The official release-notes pages date stable `v0.61.0` to 2026-09-23 and preview `v0.63.0-preview.0` to 2026-09-29. The preview is explicitly described as less stable than the weekly stable channel. The release index also listed `v0.62.0`, while the stable notes still called `v0.61.0` the latest stable release; the fetched `v0.62.0` release body did not expose its publication date. I therefore do not resolve that discrepancy or label `.62.0` the latest stable version.

Sources: [GitHub Releases](https://github.com/google-gemini/gemini-cli/releases), [stable notes](https://github.com/google-gemini/gemini-cli/blob/main/docs/changelogs/latest.md), [preview notes](https://github.com/google-gemini/gemini-cli/blob/main/docs/changelogs/preview.md).

## Findings

### 1. Checkpointing is an opt-in rollback mechanism, not a demonstrated optimization

The official checkpointing guide says Gemini CLI snapshots the project before approved AI file modifications. It stores a Git snapshot in a shadow repository under `~/.gemini/history/<project_hash>` and stores conversation history and the proposed tool call locally under `~/.gemini/tmp/<project_hash>/checkpoints`. `/restore` restores both files and conversation state. The feature is disabled by default and enabled in `settings.json`.

**Certainty:** High for documented behavior; a single official product guide, with no independent corroboration. The guide describes a rollback and recovery capability, not runtime or memory savings. **Applicability:** Treat recoverability and performance as separate design questions. IssueOps already has state checkpoints, integrity inspection, and WAL maintenance (`.issueops/architecture/runtime.md:54-58`; `internal/adapter/outbound/sqlstore/maintain.go:10-43`). Any added snapshot mechanism should first be justified against those existing boundaries and measured for filesystem/time cost. **Counterevidence:** Gemini’s snapshots include project state plus conversation data, while IssueOps state checkpoints serve a different purpose; they are not interchangeable.

Source: [Checkpointing guide](https://geminicli.com/docs/cli/checkpointing/).

### 2. Recent memory work is a preview claim without quantified results

`v0.63.0-preview.0`, dated 2026-09-29, announces bounded tool output and an optimized memory lifecycle in long-running agent loops to prevent memory leaks. Its release list identifies PR #29451 as the tool-output/memory change. This is the most recent directly relevant release evidence found; the stable `v0.61.0` notes dated 2026-09-23 instead emphasize security hardening and AgentLoopContext reliability.

**Certainty:** High that the project announced this change in preview; single-source release evidence, not independent verification that a leak is eliminated. **Applicability:** For issueops performance work, use this as a test hypothesis: bound or release large transient outputs where ownership permits, then benchmark representative long runs and inspect peak memory and output sizes. No percentage, measured gain, or corresponding issueops defect is established here. **Counterevidence:** Preview code may change before stable release; `v0.61.0` stable notes do not claim this optimization.

Source: [v0.63.0 preview notes](https://github.com/google-gemini/gemini-cli/blob/main/docs/changelogs/preview.md).

### 3. Gemini exposes useful telemetry, but prompt-bearing fields make defaults material

The official OpenTelemetry guide documents telemetry and detailed tracing as disabled by default, with local-file or OTLP export. It lists measurements for tool/API latency, token usage (including cache tokens), context-compression token counts, startup phases, CPU/memory, tool queue depth, and execution phase breakdowns. However, its settings table gives `logPrompts` a default of `true`; prompt and detailed trace attributes may contain messages, tool arguments/results, or system instructions. The guide notes detailed trace attributes are disabled by default, but does not quantify telemetry overhead.

**Certainty:** High for the guide’s stated configuration and event fields; one official documentation source, no independent corroboration. **Applicability:** The metric categories can inform issueops measurement plans, not justify adopting a new collector. Prefer low-cardinality durations, counts, queue depth, and bytes; make content-bearing telemetry opt-in and explicitly redacted. **Counterevidence:** Gemini’s measurements describe its own CLI/runtime. They do not establish that equivalent fields exist in IssueOps or that exporting them would improve it.

Source: [Telemetry guide](https://geminicli.com/docs/cli/telemetry/).

### 4. IssueOps already has narrower, purpose-built visibility surfaces

The repository provides read-only `issueops review-metrics` aggregates (`cmd/issueops/issueopscli/issueops_subcommands.go:542-568`), reports scanned-record counts for cycle listing (`cmd/issueops/issueopscli/issueops_subcommands.go:570-588`), trace analysis with evidence redaction (`.issueops/architecture/hexagonal-core.md:127-130`), and `state doctor`/maintenance/checkpoint surfaces (`.issueops/architecture/runtime.md:54-58`). These are concrete evidence against treating Gemini’s broader telemetry inventory as proof that IssueOps lacks visibility.

**Certainty:** High, direct repository source/documentation. **Applicability:** Extend or benchmark these existing read-only surfaces before considering general telemetry plumbing; measure command latency and records scanned in representative repositories. **Counterevidence:** These surfaces do not document a general per-phase latency, memory, or token dashboard, so a specific visibility gap could remain after measurement.

## EXPAND

- **EXPAND — release-date reconciliation:** Resolve why the release index lists `v0.62.0` while the stable changelog identifies `v0.61.0` as latest. The GitHub Releases API returned a rate-limit response during retrieval; retry the official tag metadata later. Do not infer a publication date from the tag name.
- **EXPAND — local performance evidence:** Profile issueops on a representative long-running workload before proposing output-retention or telemetry changes. Record wall time, peak memory, and bytes scanned/emitted; compare against an unchanged baseline. No local performance experiment was run for this research lane.
- No independent third-party corroboration was located or counted. Official release notes, product docs, and repository docs are not independent sources of implementation-performance evidence.

**Access/verification note:** The official GitHub Releases page, Gemini checkpointing guide, telemetry guide, stable notes, and preview notes were fetched. GitHub API metadata requests for `v0.62.0` and `v0.63.0-preview.0` were inaccessible due to API rate limiting. No code tests or build were run because this change is report-only.
