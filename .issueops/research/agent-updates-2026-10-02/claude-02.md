# Claude Code context compaction and persistent memory

**Retrieved:** 2026-10-02
**Observed CLI:** 2.1.287
**Scope:** Official Claude Code documentation and releases, compared with the current IssueOps repository contract. No production, installation, configuration, user-state, or history changes were made.

## Findings

### 1. Auto memory is a bounded index, not durable project authority

**Fact and version:** Anthropic documents auto memory as enabled by default in local sessions, stored under `~/.claude/projects/<project>/memory/`, and shared by worktrees of the same Git repository on one machine. Only the first 200 lines or 25 KB of `MEMORY.md` (whichever limit is reached first) loads at startup; topic files are read on demand. The feature distinguishes user, feedback, project, and reference notes, and skips facts derivable from source or existing instructions. The `modified` frontmatter timestamp is documented as requiring Claude Code 2.1.214+. [Official memory documentation, retrieved 2026-10-02](https://code.claude.com/docs/en/memory#auto-memory).

**Certainty / applicability:** High for documented behavior; Anthropic’s docs are primary, with no independent corroboration. Local memory may help an IssueOps operator, but is not a team-shared or cross-machine record. Tracked project docs and durable IssueOps state remain portable sources of truth.

**Counterevidence:** Memory is heuristic (“Claude decides” what is useful), limited at startup, and local; repository-provided settings can disable it or change its directory. Subagents do not inherit the parent’s auto memory; a subagent may instead use its own configured memory. No claim that enabling it improves task success or latency is established here.

### 2. Compaction restores selected context, not the full interaction

**Fact and version:** Anthropic describes compaction as summarizing older conversation to make room. Root `CLAUDE.md` is reread after `/compact`; nested and path-scoped instructions return only when traversed/matched. The context-window guide says a structured summary replaces verbatim history; recent modified files and invoked skill bodies can be reintroduced, but full tool outputs/intermediate reasoning are not preserved. The cost guide cautions that compaction itself reads the conversation and consumes tokens; `/clear` is the zero-cost choice when continuity is unnecessary. Sources: [memory/compaction behavior](https://code.claude.com/docs/en/memory#troubleshoot-memory-issues), [context-window guide](https://code.claude.com/docs/en/context-window), [cost guidance](https://code.claude.com/docs/en/costs#why-usage-climbs-in-a-long-session). Retrieved 2026-10-02; docs state no publication date.

**Release evidence:** Official Claude Code release v2.1.287, published 2026-10-01, fixed duplicate attachment of a folder’s `CLAUDE.md` after resume/compaction and cloud sessions losing earlier conversation when restarted during compaction. v2.1.283 (2026-09-25) added `/doctor prompt-audit` and fixed `/context` omitting MCP server instructions. [Immutable v2.1.287 notes](https://github.com/anthropics/claude-code/releases/tag/v2.1.287); [immutable v2.1.283 notes](https://github.com/anthropics/claude-code/releases/tag/v2.1.283); publication timestamps from the [official GitHub Releases API](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287) and [v2.1.283](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.283).

**Certainty / applicability:** High for release-note contents and documented behavior; the docs and release notes are one publisher, not independent corroboration. IssueOps already re-injects its static project-doc catalog on Claude `SessionStart` with `source:"compact"` and keeps hooks context-only (`.issueops/AGENT_WORKFLOW.md:63-68`). Preserve explicit resume pointers and test post-compaction visibility; do not assume the transcript survives intact.

**Counterevidence:** The latest fixes show compaction/resume correctness has had regressions. The documentation is not a guarantee that every detail survives, and does not quantify quality or runtime improvement.

### 3. Native visibility can separate context pressure from wasted work

**Fact and versions:** Anthropic documents `/context` for context composition and `/usage` for session tokens, cache hit/miss and expected-rebuild information; the cache line requires v2.1.251, likely-miss-cause labeling v2.1.260. `/usage` can flag long-context/cache-miss behaviors and attribute recent usage to skills, subagents, plugins, and MCP servers; attribution is approximate, local, and excludes subagents from prompt-cache statistics. `/insights` analyzes up to 200 previously unanalyzed local sessions and produces friction suggestions, but consumes tokens. [Official cost/usage documentation](https://code.claude.com/docs/en/costs#track-your-costs), retrieved 2026-10-02.

**Certainty / applicability:** High as documented capability, without independent corroboration. Before telemetry or core changes, compare `/context`, cache misses, and `/usage` attribution across representative IssueOps sessions. Local docs explicitly exclude lifecycle state and telemetry (`.issueops/AGENT_WORKFLOW.md:63-68`).

**Counterevidence:** These product metrics are not IssueOps metrics, may vary by provider/account, and `/insights` is neither free nor cross-device. No performance percentages or IssueOps benefit were measured in this lane.

### 4. Reduce always-loaded material before optimizing compaction

**Fact:** Anthropic recommends keeping always-loaded `CLAUDE.md` concise (target under 200 lines), moving workflow-specific guidance to on-demand skills or path-scoped rules, and using deferred MCP tool schemas. Imports organize files but do not reduce context because imported content is also loaded. [Memory docs](https://code.claude.com/docs/en/memory#write-effective-instructions), [cost docs](https://code.claude.com/docs/en/costs#reduce-token-usage), retrieved 2026-10-02.

**Certainty / applicability:** High as vendor guidance, not an IssueOps measurement. IssueOps has host-neutral skills and context-only catalog injection. Test lazy-loaded guidance on repetitive Claude tasks, recording initial context and outcomes.

**Counterevidence:** The repo’s `AGENTS.md` and project docs also carry mandatory safety/architecture constraints; indiscriminate trimming or lazy-loading could hide required rules. Imports do not save tokens by themselves.

## EXPAND

- Measure `/context` and `/usage` across a small, representative set of IssueOps Claude sessions; compare compact vs fresh-session behavior and record provider/version. Actionable because official tooling exposes direct signals; not done here.
- Review v2.1.287’s `/doctor prompt-audit` against a disposable or explicitly selected instruction set before considering any production-document edits. Actionable for stale paths/conflicts; no edits or audit run in scope.
- Inspect documented `PreCompact`/`PostCompact` and `InstructionsLoaded` hook events for an opt-in diagnostic experiment, only if it can preserve the existing context-only/no-lifecycle-state-write invariant. Actionable but requires a separate design and measurement.

No independent third-party corroboration was used; Anthropic docs and anthropics/claude-code notes are same-publisher sources. Failed calls: `webfetch(url=https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.283, format="json")` and the same call for `v2.1.287`; unsupported format was rejected, and both succeeded as text. The requested `git diff --check -- .issueops/research/agent-updates-2026-10-02/claude-02.md` exited 0. Because the file was untracked, a no-index check also ran; its first pass found trailing spaces on lines 3-4, which were removed. Final no-index check was clean. No source remained inaccessible.
