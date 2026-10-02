# Claude Code hooks: lifecycle, async execution, and observability

**Retrieved:** 2026-10-02
**Question:** What current Claude Code hook behavior is relevant to IssueOps performance, optimization, and visibility?

## Findings

1. **Lifecycle frequency varies by event.** Anthropic defines `SessionStart` / `SessionEnd` as per-session, `UserPromptSubmit` / `Stop` / `StopFailure` as per-turn, and `PreToolUse` / `PostToolUse` as per-tool-call (except `EndConversation`). Other standalone events include `ConfigChange`, `CwdChanged`, `FileChanged`, `InstructionsLoaded`, and compaction hooks. This favors low-frequency instrumentation over agentic-loop hot-path work. **Certainty: high; official single-source. Applicability: high.** Limit: `SessionStart` repeats on resume, clear, and compact; it is not once per process lifetime. Source: [Hooks reference, lifecycle and events](https://code.claude.com/docs/en/hooks#hook-lifecycle) (retrieved 2026-10-02).

2. **Parallel dispatch does not make synchronous hooks non-blocking.** Matching handlers run in parallel, and results combine after completion. Matchers and per-handler `if` conditions can avoid irrelevant command launches. Command hooks support `async: true` to run in the background without blocking; Claude Code does not enforce the configured timeout for that async command. `asyncRewake: true` wakes Claude on exit code 2 and surfaces stderr (or stdout if stderr is empty) as a system reminder. **Certainty: high; official single-source. Applicability: future opt-in diagnostics, not current IssueOps policy.** Limit: background dispatch is not a completion or durability guarantee, and failure wake-ups can create visible work. Source: [Hooks reference, handler fields and async hooks](https://code.claude.com/docs/en/hooks#command-hook-fields) (retrieved 2026-10-02).

3. **The latest release fixes async wake-up and count visibility defects.** Anthropic release `v2.1.287` (published 2026-10-01) says a missing `asyncRewake` script previously caused repeated “found issues” notifications; it is now reported once. The release also fixes transcript and debug counts that included internal callbacks as configured hooks. **Certainty: high; official single-source. Applicability: medium-high when reading hook UI/debug output.** Limit: the notes quantify no latency effect and do not rule out other async failures. Sources: [v2.1.287 release notes](https://github.com/anthropics/claude-code/releases/tag/v2.1.287); [release API date](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287) (retrieved 2026-10-02).

4. **Hook timing telemetry exists behind a beta gate.** Monitoring documents a `claude_code.hook` span carrying event/name, matching count, aggregate `duration_ms`, and success, blocking, non-blocking-error, and cancellation counts. It requires detailed beta tracing plus `BETA_TRACING_ENDPOINT`; interactive CLI use also requires organization allowlisting. Hook definitions require `OTEL_LOG_TOOL_DETAILS`. **Certainty: high; official single-source. Applicability: high for controlled measurement, low for default instrumentation.** Limit: traces are off by default, and duration alone does not establish user-perceived impact. Source: [Monitoring, detailed tracing and `claude_code.hook`](https://code.claude.com/docs/en/monitoring-usage#traces-beta) (retrieved 2026-10-02).

5. **IssueOps deliberately keeps its documented Claude hook narrow.** The repository registers `SessionStart` only, re-runs it after compaction, and injects static project context. Hooks must not read IssueOps state, write telemetry, perform maintenance, or block tool events; CLI/MCP own mutation authority. Keep the default hook cheap and static; assess richer async work only as an opt-in experiment. **Certainty: high for repository policy. Applicability: high.** Limit: policy is not a measurement of current runtime cost. Sources: [`.issueops/operations/hosts.md`, Claude Code](../../operations/hosts.md#claude-code); [`.issueops/OPERATIONS.md`, invariants](../../OPERATIONS.md#invariants) (read 2026-10-02).

## Source assessment

All external claims rely on official Anthropic / `anthropics/claude-code` sources. The docs, release notes, and GitHub API are primary-source views from one publisher, not independent corroboration. `v2.1.287` was published 2026-10-01; `v2.1.283` was published 2026-09-25. The latter adds MCP, WebFetch, and WebSearch output to `tool.output` spans with content logging enabled, not evidence of generally available hook spans. Sources: [v2.1.283 release](https://github.com/anthropics/claude-code/releases/tag/v2.1.283); [release API date](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.283) (retrieved 2026-10-02).

## EXPAND

- **Benchmark latency:** Compare baseline and one matched hook with detailed tracing; record event, count, duration, outcome, and any measurable `SessionStart` delay. Docs provide no improvement percentage.
- **Test async visibility:** On a recorded version, test success, failure, cancellation, and missing-script cases; inspect `/hooks`, transcript, and debug output.
- **Pin minimum versions:** Find dated official release history for async hooks and detailed hook spans before making either an installer requirement; fetched notes did not identify introduction versions.

## Access and failed probes

No cited public source was inaccessible. The WebFetch tool rejected two release-API attempts with `format: "json"` (supported formats are markdown, text, and HTML); retrying as `text` succeeded. Two repository reads failed: `configs/claude/settings.json` returned `ENOENT` (path absent), and reading `internal/adapter/claude` as a file returned `EISDIR` (directory). No shell command failed; `git diff --check -- .issueops/research/agent-updates-2026-10-02/claude-06.md` exited 0. Markdown diagnostics were unavailable because no `.md` LSP server is configured. No tests or build were applicable to this research-only report.
