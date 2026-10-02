# Omo monitor subscriptions: deduplication and restart persistence

Retrieval date: **2026-10-02**. Scope: public official material and installed implementation; no operational state or transcripts inspected.

## Provenance and evidence boundaries

Canonical Omo project: [code-yeongyu/oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent), established by `/Users/habin/node_modules/omo-ai/package.json:2-26`: installed `omo-ai` **5.1.8** depends on `@code-yeongyu/senpi` **2026.10.1-2**. Its [official release notes](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.8) identify that engine; [release metadata](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.8) dates publication to **2026-10-01T15:00:33Z**.

Below, `R` means `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d`; `T` means `R/dist/core/extensions/builtin/terminal`. `R/package.json:122-126` identifies the engine's canonical repository as **code-yeongyu/senpi**, directory `packages/coding-agent`. These are local implementation observations, not assertions about every Omo edition. Official docs, changelog, and shipped code share maintainer provenance: their agreement is implementation corroboration, **not independent-source corroboration**. No independent corroboration or performance benchmark was established.

## Findings

### 1. Deduplication is batch-local bookkeeping, not durable exactly-once delivery

**High certainty, installed implementation.** `T/monitor-notify.js:60-69,160-169,220-234` keeps the last injected batch in an in-memory map keyed by runtime monitor ID. Fingerprints concatenate sanitized line bodies; only line-only batches without overflow qualify. Equal consecutive fingerprints suppress injection. Summaries clear previous fingerprints (`:73-76`); rearm clears them too (`:97-103`). This is neither individual-event deduplication nor a persisted event cursor.

**Applicability:** issueops watches should print stable, state-changing sentinels and avoid assuming restart deduplication. **Counterevidence/limit:** the stable `mon_` identity survives restart, but the command gets a new runtime handle and the notifier map is not part of the manifest. Repeated identical text grouped differently can evade suppression; legitimate repeated identical batches can be suppressed.

### 2. Delivery has separate bounded batching and noise budgets

**High certainty.** Defaults are 2,000 ms coalescing, 5,000 ms per-monitor rate limiting, 50 lines, 4,096 characters, and a wake budget of five (`T/monitor-notify.js:19-26`). Overflow is counted rather than retained as unlimited lines (`:79-89,117-127`). Only contributing monitors are paused when the notification budget is reached (`:180-211`); completion summaries have separate handling. Persistent command watches additionally consume a 200-matching-line/24-hour budget **before notifier deduplication** (`T/monitor-registry.js:598-630`; `T/shared.js:54-68`).

**Applicability:** distinguish “watch alive,” “delivery muted,” and “completion” in visibility work; source-side filtering matters even when duplicate notifications disappear. **Counterevidence/limit:** these bounds do not establish any percentage improvement. Rearm deliberately resets delivery bookkeeping; restart restoration adopts the saved fire window (`T/restore-session.js:67-73`).

### 3. Command persistence means restart execution, not output replay

**High certainty.** `T/durable-command.js:75-141` respawns the recorded command in its absolute cwd with stable ID, restored flag, downtime upper bound, and persistent state-directory environment. A two-second grace classifies an immediate zero exit as completed and a nonzero exit as lost; previous PTY output is not replayed. The [official changelog](https://raw.githubusercontent.com/code-yeongyu/senpi/main/packages/coding-agent/CHANGELOG.md), also installed at `R/CHANGELOG.md:651-677`, dates baseline-directory and restore-reliability changes to **2026.9.24-3, 2026-09-24**.

**Applicability:** restart-safe issueops watches need saved baselines and read-only/idempotent command behavior. **Counterevidence/limit:** a tail-only watcher cannot reconstruct downtime events automatically; the environment enables reconciliation, not replay.

### 4. File restoration compares checkpoints; “native” does not mean polling-free

**High certainty.** `T/durable-file.js:17-27,88-108` compares presence, device/inode, size, mtime, and digest and emits at most one detached-change line. It reports a previously present but now missing file as lost. However, `T/monitor-file-watch.js:1-33` also runs a **250 ms interval** because filesystem notifications can miss rename-heavy changes. Pausing clears that timer; resume immediately checks.

**Applicability:** prefer the file branch for one-file subscriptions, but account for stat/digest work when many watches are active. **Counterevidence/limit:** this is state comparison, not a journal: intermediate changes returning to the saved state may be invisible. No CPU measurement was performed.

### 5. Official documentation lags installed limits and ephemeral restoration

**High certainty.** [Official terminal documentation](https://raw.githubusercontent.com/code-yeongyu/senpi/main/packages/coding-agent/docs/terminal-tools.md) says five standing watches and no ephemeral restoration. Installed admission defaults to unlimited unless configured (`T/tools/monitor-manifest-binding.js:58-76`); `T/restore.js:78-85` restores ephemeral watches with remaining deadlines. The official changelog dates cap removal to **2026.10.1, 2026-10-01**. Seven-day absolute expiry remains (`T/shared.js:45-49`).

**Applicability:** issueops guidance should use installed contracts and expose configured limits. **Counterevidence/limit:** unlimited admission is not unlimited system capacity, and missing/expired ephemeral deadlines are rejected.

## EXPAND

- **EXPAND / validation:** run an isolated future restart test for repeated identical batches, surviving fire budgets, muted file watches, and downtime baseline reconciliation; this lane only inspected implementation.
- **EXPAND / visibility:** assess exposing stable IDs, mute reasons, overflow counts, and restart outcomes without duplicating Omo's scheduler.
- **EXPAND / documentation:** reconcile official five-watch/no-ephemeral statements with shipped behavior.

Inaccessible sources: none. One failed inspection used the wrong path `T/monitor-manifest-binding.js` (ENOENT); corrected to `T/tools/monitor-manifest-binding.js`. Oversized release-list/features responses were not treated as complete evidence; targeted sources supplied the cited claims. Tests/build/diagnostics were not run: this evidence-only report has no executable change.
