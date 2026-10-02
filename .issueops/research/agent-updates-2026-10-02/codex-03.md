# Codex noninteractive execution and structured output

**Retrieved:** 2026-10-02
**Scope:** Official Codex CLI/TypeScript SDK evidence relevant to IssueOps performance, optimization, and execution visibility. No production code, configuration, installation, user state, or git history was changed.

## Findings

1. **`codex exec` is an established noninteractive automation surface, not itself a performance optimization.** The CLI source says omitting a subcommand starts a “new non-interactive session” and exposes `--json`, `--output-schema`, and `--output-last-message`. OpenAI’s evaluation guide (2026-01-22) presents `codex exec` for automation/CI and recommends least permissions. This supports measuring a bounded IssueOps repeatable-check experiment rather than assuming it is faster. **Certainty: high** for CLI behavior; **medium** for fit. **Counterevidence:** no IssueOps integration or latency gain is reported. Sources: [Codex CLI source, `Cli`](https://github.com/openai/codex/blob/main/codex-rs/exec/src/cli.rs#L14-L71); [OpenAI evaluation guide](https://developers.openai.com/blog/eval-skills).

2. **JSONL events and schema-constrained final output serve different visibility needs.** `--json` emits JSONL events; OpenAI’s guide uses ordered `item.started` / `item.completed` command events and `turn.completed` token usage for deterministic checks. `--output-schema` separately shapes the final response for a stable summary or rubric. IssueOps could retain the trace and request a compact finding, without treating the final object as a trace substitute. **Certainty: high** for interfaces; **medium** for this instrumentation. **Counterevidence:** the guide demonstrates skill evals, not production telemetry; schemas constrain shape, not truth. Sources: [Codex CLI source](https://github.com/openai/codex/blob/main/codex-rs/exec/src/cli.rs#L48-L71); [OpenAI evaluation guide](https://developers.openai.com/blog/eval-skills).

3. **The TypeScript SDK wraps the CLI process rather than exposing an independent execution engine.** `CodexExec.run` starts `exec --experimental-json`, optionally adds `--output-schema`, writes input to stdin, yields stdout lines, buffers stderr, and raises on nonzero exit. It enables orchestration and live event consumption but inherits CLI startup and stream behavior. **Certainty: high** for current source; **medium** for advantage over direct invocation. Official release metadata records CLI **0.160.0, published 2026-10-01**; this does not date the SDK implementation. **Counterevidence:** no comparative benchmark. Sources: [TypeScript SDK `exec.ts`](https://github.com/openai/codex/blob/main/sdk/typescript/src/exec.ts#L1-L225); [release metadata](https://api.github.com/repos/openai/codex/releases/401312540).

4. **The repository has a concrete reason to treat streamed output as a failure boundary.** IssueOps architecture describes Codex as a thin adapter that invokes the shared CLI/MCP surface, while its documented July 8 incident attributes invalid Codex hook JSON to a co-resident process truncating piped stdout: 19,489 bytes when redirected to a file versus 512 bytes through a pipe. That incident was a hook-output defect, not evidence that `codex exec --json` truncates; it makes pipe integrity, stderr separation, exit status, and complete-line parsing sensible checks for any new capture path. **Certainty: high** for the repository’s recorded incident; **medium** for applying its lesson to exec traces. **Counterevidence:** the incident does not test the TypeScript SDK or JSONL execution output. Sources: [`.issueops/architecture/host-integration.md:13`](../../architecture/host-integration.md#L13); [`.issueops/cautions/lessons/2026-07-08-codex-invalid-json-output-pipe-truncation.md:12-18`](../../cautions/lessons/2026-07-08-codex-invalid-json-output-pipe-truncation.md#L12-L18).

## Source classification and applicability

These are official OpenAI/Codex sources, not independent corroboration; no independent source was found. The 2026-01-22 guide is outside the preferred 90-day window, so the search widened to current CLI code and the 2026-10-01 release. [Release 0.160.0](https://github.com/openai/codex/releases/tag/rust-v0.160.0) mentions plugin-loading maintenance, not a measured `exec`, JSONL, or schema performance gain. Treat this as an instrumentation lead, not a speedup conclusion. IssueOps likewise calls for measured bottlenecks (`.issueops/CONSTITUTION.md:50`).

## EXPAND

- Compare direct CLI versus TypeScript SDK on one existing, repeatable, read-only IssueOps task; capture wall-clock duration, exit status, JSONL command count, and reported token usage across repeated runs. No baseline or benchmark was found in the inspected sources.
- Inspect whether the current IssueOps verification/evaluation surfaces can preserve event traces and a schema-validated summary without retaining secrets or introducing a mandatory Codex dependency. Existing architecture keeps Codex optional and core behavior host-neutral.
- **Not expanded:** no independent vendor benchmark or 2026-10-01 release-note claim supports a numeric speedup; further numerical claims would be invented.

## Access and command notes

No cited source was inaccessible. One GitHub API fetch attempt used unsupported `webfetch` format `json`; it was retried successfully with `text`. No shell command failed.
