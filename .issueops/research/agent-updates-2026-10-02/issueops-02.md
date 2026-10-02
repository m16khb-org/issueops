# IssueOps SQLite: serialization, query paths, and visibility

Retrieval date: **2026-10-02**. Scope: repository source, neighboring tests, and public official documentation; no tests or benchmarks executed. Certainty labels distinguish observed source facts from unmeasured optimization hypotheses.

## Version and source provenance

`go.mod:14` pins **modernc.org/sqlite v1.53.0**. The Go module proxy records its tag timestamp as **2026-06-21T10:19:42Z**: https://proxy.golang.org/modernc.org/sqlite/@v/v1.53.0.info. Version-specific package documentation lists **SQLite 3.53.2**, including darwin/arm64: https://pkg.go.dev/modernc.org/sqlite@v1.53.0. SQLite's release record dates 3.53.2 to **2026-06-03**: https://sqlite.org/releaselog/3_53_2.html. These establish source dependency versions, not the identity of an installed binary.

SQLite documentation below is **official single-source evidence**; multiple SQLite pages are not independent corroboration. Go-hosted module metadata and modernc-authored documentation corroborate dependency identity, not SQLite performance. Repository tests provide local executable specifications, not independently reproduced results.

## Findings

### 1. Root-wide span exclusion is separate from data atomicity

**High certainty; directly applicable.** `internal/adapter/outbound/sqlstore/sqlstore.go:203-241` opens a WAL data database and a separate lock database with immediate transactions. `sqlstore.go:340-411` holds a per-handle token gate and lock-database transaction throughout the callback, then rolls back that lock transaction. Consequently, different record IDs in the same root cannot run these spans concurrently. `internal/adapter/issueops/issueops_lock.go:13-43` confirms the cycle lock delegates to this root-wide mechanism.

The lock rollback does not undo data writes: `sqlstore.go:674-685` performs an autocommit upsert, while `sqlstore.go:699-780` implements explicit atomic mutation batches and raw-byte CAS in the data database. Official transaction semantics allow only one simultaneous writer and make BEGIN IMMEDIATE acquire write intent immediately: https://sqlite.org/lang_transaction.html.

**Counterevidence:** root-wide exclusion protects multi-step invariants; replacing it with per-record locks cannot remove SQLite's single-writer limit. `internal/adapter/outbound/sqlstore/sqlstore_test.go:241-292` specifies atomic CAS and mismatch rollback. These tests were read, not run.

### 2. Cancellation and contention bounds differ across paths

**High certainty for code; optimization impact unknown.** `sqlstore.go:34-47,423-476` uses typed BUSY/LOCKED retry with exponential gaps from 1 ms to 10 ms and a 60-second SQLite acquisition timer. Local gate waiting separately selects on context cancellation; its duration is not covered by that timer. `internal/adapter/outbound/sqlstore/span_context_test.go:52-120` specifies local and SQLite waiter cancellation.

Read-only existing-store queries instead open connections with a 2-second busy timeout (`sqlstore.go:655-671`). `GetExisting` uses context-free QueryRow (`sqlstore.go:496-511`), whereas streaming uses QueryContext. `internal/adapter/outbound/issueopsrecord/store.go:42-74` checks context before entering the context-free read/list functions.

**Counterevidence:** this is not proof of observed hook latency. Neighboring contention tests deliberately use DELETE journaling and exclusive locks, not normal WAL traffic (`sqlstore_test.go:81-190`); one releases after a fixed sleep. They do not measure production WAL checkpoint behavior.

### 3. Query shape already exploits ordered keys and streaming

**High certainty for implementation; performance hypotheses only.** The records table is `WITHOUT ROWID` with `(bucket,id)` primary key (`sqlstore.go:207-212`). Point reads constrain both keys (`sqlstore.go:479-511`); `WalkExisting` performs one ordered bucket query, visits rows inline, and closes rows and connection on return (`sqlstore.go:553-579`). `issueopsrecord/store.go:93-123` decodes through that stream, avoiding list-then-per-ID reads.

Official WITHOUT ROWID documentation dates support to **3.8.2, 2013-12-06**, and describes clustered primary-key storage: https://sqlite.org/withoutrowid.html.

**Counterevidence:** that same document warns large BLOB rows can favor rowid tables. No row-size distribution or query plan was measured. Inline visitors also keep the read cursor active; long WAL readers can impede checkpoint completion: https://sqlite.org/wal.html, sections 2.2 and 6. `reader_test.go:15-81` checks cleanup after completion, cancellation, and visitor error, not during a slow visitor.

### 4. Visibility exists but cannot separate all waiting causes

**High certainty; directly applicable.** `issueopsrecord/observer.go:16-26,44-93` exposes operation, outcome, contention, wait, and hold milliseconds. It suppresses fast successful uncontended events; the default and composition-root threshold is 100 ms (`cmd/issueops/issueopsapp/issueops_record_store_wiring.go:10-24`). `observer_test.go:16-51` checks emission and omission of sensitive identifiers.

**Counterevidence:** aggregate span wait combines local gate and SQLite acquisition; it excludes preceding Open work and does not time standalone queries. Maintenance already returns checkpoint success and WAL bytes before/after (`internal/adapter/outbound/sqlstore/maintain.go:16-34`). Existing instrumentation should be assessed before adding another telemetry layer.

## EXPAND

- **EXPAND-MEASURE:** Separate local-gate, SQLite-acquisition, Open, query, and visitor durations in an authorized measurement lane before choosing lock granularity or connection reuse.
- **EXPAND-QUERY:** Obtain representative JSON sizes and EXPLAIN QUERY PLAN output on disposable fixtures before changing schema/indexes.
- **EXPAND-CHECKPOINT:** Correlate active-reader duration with existing checkpoint/WAL results; preserve cancellation, cursor cleanup, and CAS guarantees.

No speedup percentages are asserted. Inaccessible sources: none. Failed commands: none. Verification: cited sources reopened/read; `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-02.md` exited 0. Because the report is untracked, its whitespace and word count were also checked directly. Tests, builds, diagnostics, and benchmarks are intentionally outside this evidence-only lane.
