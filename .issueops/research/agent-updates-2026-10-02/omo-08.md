# Omo session metrics and trace visibility

Retrieval date: **2026-10-02**. Scope: official public sources, installed implementation, and IssueOps source; no credentials or session transcripts inspected. Actual session model: `gpt-6.1-sol` (`PI_MODEL`).

Canonical project: installed `/Users/habin/node_modules/omo-ai/package.json:2-25` identifies `omo-ai` **5.1.8**, repository `code-yeongyu/oh-my-openagent`, and engine `@code-yeongyu/senpi` **2026.10.1-2**. The [official release](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.8) confirms that engine; [GitHub release metadata](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.8) gives publication **2026-10-01T15:00:33Z**. These are publisher-controlled sources, not independent corroboration. Installed code independently checks implementation consistency, not performance or billing accuracy. Findings below describe this installed version, not the introduction date of each feature.

For exact local anchors, `R` means `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d`; IssueOps-relative paths resolve under `/Users/habin/workspace/issueops`.

## Findings

### 1. Usage survives compaction, but totals are not an invoice

`R/dist/core/agent-session.js:8593-8654` aggregates all session entries, including assistant messages, explicit usage entries, tool-reported usage, compaction, branch summaries, and cache prewarming. It returns input/output/cache-read/cache-write totals, cost, current context usage, and failure statistics. `R/dist/core/usage-totals.js:41-83` groups assistant usage by provider and `responseModel ?? model`; tools, summaries, and prewarming share a separate bucket.

**Certainty:** high for code behavior. **Applicability:** consume existing totals rather than estimate tokens from text; keep lifetime expenditure separate from current context occupancy. **Counterevidence:** grouping is not an agent hierarchy breakdown, and missing tool usage cannot be recovered here. Cost is summed from supplied usage, not reconciled against a provider invoice. RPC documentation says context values can be null after compaction (`R/docs/rpc.md:2055-2110`).

### 2. Structured streaming supports traces without cumulative snapshots

The [official JSON-mode documentation](https://raw.githubusercontent.com/code-yeongyu/senpi/main/packages/coding-agent/docs/json.md) defines session headers, turn/message events, tool-call IDs, retry/compaction events, and resolved tool names. Streaming updates omit cumulative message snapshots; their usage is cumulative and may remain zero until completion. `message_end` is authoritative. Installed documentation agrees (`R/docs/json.md:95-119`).

**Certainty:** high, official single-source contract corroborated by installed documentation, not a separate publisher. Public `main` is mutable and has no feature-release date in this document. **Applicability:** a future collector should correlate session and tool IDs, finalize usage on message completion, and avoid summing every streaming update. **Counterevidence:** this is an event stream, not proof of an OpenTelemetry exporter or a durable distributed trace store.

### 3. Diagnostic logs are bounded and deliberately lossy

`R/dist/core/session-log.js:5-75` writes JSON lines to `logs/session.log`, caps the active file at a default 5 MiB, rotates one `.1` backup, uses restrictive permissions, filters data keys, and redacts selected secret patterns. Strings are limited to 200 characters (`:99-102`). The fallback logger writes a separate `logs/fallback.log` with selector/model/reason fields (`R/dist/core/retry-fallback/log.js:3-47`).

**Certainty:** high for installed implementation. **Applicability:** use these logs for retry/compaction diagnostics; preserve their field-filtering approach in an IssueOps adapter. **Counterevidence:** retention is short, redaction is pattern-based, and there is no guaranteed shared correlation ID: fallback fields omit session ID, while session logging permits but does not require it. These files are not complete transcripts.

### 4. TPS means local assistant elapsed time, not gateway throughput

`R/dist/core/extensions/builtin/tps.js:8-65` measures assistant message intervals with a monotonic clock, divides output tokens by their summed duration, and reports cache-read share of prompt tokens. Notifications require a UI and positive output.

**Certainty:** high. **Applicability:** label this as local assistant throughput, separate from full workflow latency and gateway-reported rates. **Counterevidence:** [issue #7719](https://github.com/code-yeongyu/oh-my-openagent/issues/7719) requests gateway-reported cost/tokens/TPS for the OpenCode plugin. It is a feature request, not shipped documentation, independent validation, or proof that native Omo has gateway telemetry.

### 5. IssueOps currently needs normalization, not direct ingestion

`internal/adapter/trace/decode.go:40-82` extracts verification, guard, and document-upkeep fields; `internal/domain/trace/analysis.go:34-58` recognizes upkeep events and failed `step_end` events. This inspected path does not interpret Omo message usage or session diagnostics.

**Certainty:** high for this analyzer only. **Applicability:** normalize Omo evidence into an explicit host-neutral DTO before adding findings. **Counterevidence:** this does not establish absence of metrics elsewhere in IssueOps.

## EXPAND

- **EXPAND-P1 — integration:** design synthetic fixtures for cumulative usage, final-message authority, null context, and summarization costs; no private transcript collection needed.
- **EXPAND-P2 — attribution:** determine parent/child session lineage and stable correlation contracts before claiming whole-cycle cost.
- **EXPAND-P2 — measurement:** benchmark collector overhead and retention loss on synthetic streams; no performance percentage is established here.

## Verification and limits

Key public URLs were fetched and reopened (HTTP 200); cited implementation ranges were read. No inaccessible public source. One failed read: nonexistent `internal/domain/trace/analyze.go` (`ENOENT`); resolved to `analysis.go`. Broad release output and the metadata body were truncated; the specific release and visible metadata header establish the cited facts. `git diff --check -- .issueops/research/agent-updates-2026-10-02/omo-08.md` returned exit 0. The file is untracked, so that command does not inspect its contents; manual review and a separate whitespace check cover the new file. Tests, build, and language diagnostics are not applicable to this Markdown-only addition. No installs, external writes, agents, or production edits were performed.
