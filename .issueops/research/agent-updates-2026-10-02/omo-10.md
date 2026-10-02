# Omo task cancellation, orphan cleanup, and recovery

Retrieval date: **2026-10-02**. Scope: official public evidence, installed implementation, and read-only IssueOps applicability. No execution-based process-cleanup experiment was performed.

## Identity and release boundary

Installed `/Users/habin/node_modules/omo-ai/package.json:2-25` identifies **omo-ai 5.1.8**, its canonical repository as **code-yeongyu/oh-my-openagent**, and engine dependency **@code-yeongyu/senpi 2026.10.1-2**. These are not interchangeable with the separate OpenCode edition. The version-pinned [official changelog](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/CHANGELOG.md) dates 5.1.8 to **2026-10-01**, 5.1.1 to **2026-09-29**, and 5.1.0 to **2026-09-28**. These are changelog dates, not independently verified publication timestamps. Findings below describe shipped 5.1.8 code; their original introduction dates are not inferred. All selected releases fall within 90 days.

## Findings

### 1. Cancellation commits terminal state before teardown

The [5.1.8 steering controls](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/steering/controls.ts), lines 35-98, transition pending/running records to cancelled before aborting. Pending cancellation dequeues work and clears persisted steering. Running cancellation catches abort rejection and still invokes the lifecycle destruction port. Internal DAG cancellation can defer in-process disposal until the child's outcome settles. The [destruction implementation](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/lifecycle/destroy.ts), lines 41-91, always attempts disposal after best-effort abort/termination and forgets registered handles in `finally`.

**Certainty:** high, static implementation evidence. Installed `plugin/extensions/omo-task.js:2` contains the matching `steering cancel abort rejected` branch and deferred-destruction logic. This corroborates packaging, not independent operational correctness. **Applicability:** cancellation telemetry should distinguish durable terminal status from completed resource disposal. **Counterevidence:** cancellation of an already non-running task is a no-op; status alone cannot establish that every process has exited. Disposal errors can propagate.

### 2. Orphan termination is mode-specific, not a universal tree-kill guarantee

`destroy.ts:98-143` closes host-backed child sessions through their session identity rather than signalling their shared host. Standalone orphan PIDs receive SIGTERM, then SIGKILL if still alive after the configured grace. Events record each signal. Consumed PIDs are cleared except on lost records, whose PID remains a retention breadcrumb.

**Certainty:** high for the inspected branches; descendant coverage remains unproven. Installed `/Users/habin/node_modules/omo-ai/bin/lib/child-process.js:15-114` provides an important limitation: the launcher forwards TERM/HUP, waits up to a default ten seconds, then re-signals itself; that timer does not explicitly SIGKILL its child. Windows skips this POSIX forwarding. **Applicability:** IssueOps already exposes HUP/TERM/KILL in `internal/adapter/issueops/cleanup_process_control.go:19-23`, and group killing in `internal/adapter/hostprobe/process_group_unix.go:14-27`. Preserve these ownership distinctions rather than treating an Omo host PID as a task PID. **Counterevidence:** signalling a PID is not evidence of killing its descendants, and this lane did not audit every runner/signaller backend.

### 3. TTL cleanup preserves recoverability when close is uncertain

The [TTL implementation](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/lifecycle/ttl.ts), lines 39-157 and 195-232, refreshes one host-session snapshot per sweep, atomically tombstones eligible records, and ends children before deleting artifacts. Unconfirmed close restores the record; an in-flight close retains its tombstone until settlement. Sweep ownership prevents concurrent attempts from taking over a live deletion. Live handles/owners, nonterminal work, undelivered completion notifications, and lost processes without dead-PID proof are retained.

**Certainty:** high, official source and matching installed `omo-task.js:2` late-close recovery branch; no independent corroboration. **Applicability:** expose retained-versus-deleted counts and pending-close reasons before attempting more aggressive garbage collection. **Counterevidence:** retention can intentionally delay reclamation; an unreadable tombstone cannot identify a child, so artifact completion still proceeds.

### 4. Recovery and visibility improved, with an explicit isolation cost

The official changelog's 5.1.0 notes fix cancellation/cleanup/reload races during fallback. Its 5.1.1 notes introduce per-parent task hosts, host PID/session/memory/crash visibility, and one crash warning followed by recovery completion including cancelled children. The [5.1.8 lifecycle contract](https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/AGENTS.md), lines 94-106, distinguishes resumable shutdown from deliberate cancellation, documents owner-fenced recovery, and classifies retryable respawn failures as deferred versus unrecoverable nonterminal failures as lost.

**Certainty:** medium-high: official single-project documentation, not independent corroboration or measured recovery proof. **Applicability:** separate host crash, task cancellation, deferred recovery, and lost state in IssueOps reporting. **Counterevidence:** the release notes explicitly say per-parent isolation costs memory; it is not a demonstrated performance optimization.

## EXPAND

- **EXPAND: implementation verification.** Audit RPC/in-process descendant termination and PID identity checks, then design disposable cancellation/fallback/shutdown fixtures. Current evidence establishes ordering, not survivor-free process trees.
- **EXPAND: visibility.** Evaluate mapping Omo host/task identities and retained-cleanup reasons into existing IssueOps diagnostics without adding host-specific core behavior.
- **EXPAND: measurement.** Compare recovery latency and retained-host memory under equal workloads; no performance percentage is established here.

Access failures: GitHub release API `https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.6` returned a rate-limit response; guessed `https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/runners/rpc/kill.ts` returned 404. Official release-list output was truncated; version-pinned sources replaced it. No shell command failed during investigation.
