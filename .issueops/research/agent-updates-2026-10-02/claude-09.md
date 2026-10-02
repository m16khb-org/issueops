# Claude CLI headless streaming and session resume

Retrieved: 2026-10-02
Scope: One bounded evidence lane for IssueOps performance, optimization, and visibility.
Word count: approximately 820.

## Findings

### 1. Headless mode exposes real-time structured output, but streaming must be requested

**Fact (high certainty; official, single-source):** Claude Code's current headless guide documents `claude -p --output-format stream-json --verbose --include-partial-messages` for incremental output. Each output line is a JSON event; the final `result` event contains final response text, cost, and session metadata. Text can be surfaced from `stream_event` / `content_block_delta` / `text_delta`; tool activity also has structured events. The guide says a slow consumer applies backpressure: Claude waits for queued output to drain, up to 30 seconds. This is a concrete visibility mechanism, not a measured speedup.

**Version/date:** Current guide retrieved 2026-10-02; it explicitly records an exit-drain fix in v2.1.214 (release date not established in the accessed primary release evidence). The separate streaming SDK page corroborates the event shape, but is Anthropic first-party documentation, not independent corroboration.

**IssueOps applicability:** `internal/adapter/hostprobe/claude.go:102-107` runs Claude with a process runner, then parses `output.Stdout` at `claude.go:118`; `runner.go:258-264` decodes the collected byte slice. The hostprobe gets structured evidence after process completion, not live progress. A visibility improvement could stream bounded events while retaining final validation and output limits. **Counterevidence/constraint:** this is a bounded conformance probe, not a live agent console; callbacks change its runner contract and may not be justified absent measured need.

### 2. Recent releases document real stream and resume correctness fixes

**Fact (high certainty; official publisher source):** Anthropic's GitHub release notes for v2.1.287, published 2026-10-01, fix `--include-partial-messages` occasionally emitting a delayed or missing `message_stop` after a cut-short reply, and fix `stream-json` / SDK output omitting turns of a forked skill. v2.1.286, published 2026-09-30, fixes `--resume` and `--continue` losing turns after parallel tool calls when the prior session crashed or was killed. These are observed product fixes, not forecast features. Release metadata and notes are from the same publisher, so they are not independent corroboration.

**IssueOps applicability:** Any future parser should account for these fixes and test terminal events and resumed history. **Counterevidence/constraint:** these notes do not show that IssueOps currently loses events or turns. No performance percentage is claimed.

### 3. Resume is supported, but headless session discovery has important boundaries

**Fact (high certainty; official, single-source):** The sessions guide says local transcripts are saved continuously; `--resume <session-id>` can resume a `claude -p` session even though print/SDK sessions are excluded from the interactive picker and plain `claude --continue`. `claude -p --continue` includes print, SDK, and `/loop` sessions. The CLI reference also documents resuming by ID, name, or absolute transcript path. Release v2.1.285 (published 2026-09-29 in the official release stream) changed running background-session resume to open/attach rather than reject, with restrictions when output is redirected or flags such as `--output-format json` are used.

**IssueOps applicability:** The hostprobe passes `--no-session-persistence` (`claude.go:166-170`), consistent with an ephemeral probe. A future recoverable task should associate an explicit session ID with its durable task record rather than assume `--continue` finds a `-p` session. **Counterevidence/constraint:** no existing IssueOps Claude workflow was found that needs recovery; persistence adds state without demonstrated benefit.

### 4. `--bare` is a targeted startup optimization, not a drop-in for this probe

**Fact (high certainty; official, single-source):** Anthropic's headless guide says `--bare` reduces startup work by skipping discovery of hooks, skills, commands, subagents, plugins, MCP servers, auto memory, and `CLAUDE.md`; API-key authentication is required because bare mode does not use subscription credentials. Current IssueOps Claude hostprobe explicitly supplies a strict, temporary MCP config, suppresses ambient tools, and (outside hook-smoke mode) installs a temporary SessionStart hook (`claude.go:74-100,156-178`).

**Applicability:** benchmark `--bare` only for a repeatable workload that does not need those customizations, holding prompt, model, auth, and tool contract constant. **Counterevidence/constraint:** here it would remove the hook/MCP evidence under test. No local benchmark was run and no gain is asserted.

## EXPAND

- Measure Claude process startup, first structured event, probe completion, and total duration separately across repeated runs and supported CLI versions; preserve existing output bounds. This determines whether streaming or `--bare` addresses an actual IssueOps bottleneck.
- If recoverable long-running Claude work becomes a product requirement, prototype explicit session-ID persistence and test resume after normal exit, interruption, and parallel-tool activity; do not reuse ephemeral hostprobe sessions.
- Check release notes again before adopting streaming in production, especially terminal event guarantees and resume fixes.

Repository evidence shows no current resumable workflow or measured bottleneck, so these are investigation leads, not proposed changes.

## Sources and retrieval notes

- Official headless/CLI guide: https://code.claude.com/docs/en/headless (retrieved 2026-10-02).
- Official CLI reference: https://code.claude.com/docs/en/cli-reference (retrieved 2026-10-02).
- Official session guide: https://code.claude.com/docs/en/sessions (retrieved 2026-10-02).
- Official SDK streaming guide: https://code.claude.com/docs/en/agent-sdk/streaming-output (retrieved 2026-10-02).
- Official v2.1.287 notes: https://github.com/anthropics/claude-code/releases/tag/v2.1.287; official publication metadata: https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287.
- Official v2.1.286 notes: https://github.com/anthropics/claude-code/releases/tag/v2.1.286; official publication metadata: https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.286.
- Official v2.1.285 notes: https://github.com/anthropics/claude-code/releases/tag/v2.1.285.
- Repository anchors: `internal/adapter/hostprobe/claude.go:102-128,156-178`; `internal/adapter/hostprobe/runner.go:258-306`.
- Inaccessible source: legacy URL `https://docs.anthropic.com/en/release-notes/claude-code` returned a page-not-found response. The current official GitHub release pages and API metadata above were fetched instead.
- Failed command: none during investigation. Verification commands are recorded in the task completion response.
