# State/sqlstore locking and cancellation

Date: 2026-10-03. Source revision: `3e03b21bcf80b29047f62237d82ce8a4779d0efa`.
Scope: persistence adapters and their immediate application/composition callers.
Research only; no implementation, service, configuration, or commit changes.

## Decision

Keep root-wide serialization. A key-sized Go lock would not make the existing
SQLite lock database support concurrent writers, and removing that database
lock would discard cross-process exclusion. Moving the complete callback into
one data transaction would change the existing visibility and partial-commit
contract. Neither is a bounded optimization.

Prioritize three smaller tasks:

1. Keep an acquired span's cross-process lock alive until its callback exits,
   including after request cancellation.
2. Serialize State retention selection and deletion with State writers.
3. Reject already-canceled State operations before opening or scanning storage.

The first two address source-derived correctness gaps; the third closes a
demonstrable cancellation-entry gap without changing storage ports. None needs
new infrastructure, a schema change, or host-specific behavior. Implementation
must first add the focused regressions described below.

## Method and evidence limits

Read `AGENTS.md`, `.issueops/CONSTITUTION.md`, `ARCHITECTURE.md`,
`CONVENTIONS.md`, `TESTING.md`, `CAUTIONS.md`, `AGENT_WORKFLOW.md`, and their
relevant runtime/concurrency modules. Used CodeGraph before code searches;
its broad results located callers but did not reliably select the requested
sqlstore file. Read the current files directly afterward. Two LSP reference
requests timed out; narrow source searches and caller reads supplied the
remaining references.

Three evidence categories are kept separate:

- **Executed:** existing package tests and shipped microbenchmarks below.
- **Source-derived:** specific permitted interleavings, supported by current
  repository code plus the installed Go/driver implementation. These are not
  newly executed failing regression tests or reports of production incidents.
- **Hypothesized:** production contention frequency, latency percentiles,
  workload impact, and improvement magnitude. No production profile or live
  user-state workload was collected.

The initial `git status --short` was empty. Actual toolchain:
`go version go1.27.1 darwin/arm64`, Apple M4. The module declares Go 1.26.3
and `modernc.org/sqlite v1.53.0` (`go.mod:3`, `go.mod:14`).

## Current mechanism and invariants

All paths below are relative to the repository root.

| Boundary | Current implementation | Constraint on changes |
|---|---|---|
| Handle cache | `internal/adapter/outbound/sqlstore/sqlstore.go:136` `Open` holds global `handlesMu` while pruning every cached root, repairing permissions, and initializing an uncached root. `CloseRoot` starts at line 175. | Preserve one published handle/local gate per absolute root, removed-root eviction, private-file checks, and explicit close semantics. |
| Local span exclusion | `sqlstore.go:407` `runSpan` rejects active-root reentry, then acquires `spanGate` with a cancellation-aware select. | `WithKeyLock` validates a key but delegates to a root span, not a key lock (`internal/application/state/service.go:224`). |
| Cross-process span exclusion | `sqlstore.go:510` `beginSpanTxAfterContention` uses `d.span.BeginTx(ctx, nil)` on `issueops.lock.db`; its connection uses `_txlock=immediate`, `busy_timeout(0)` at line 244. | Keep the lock separate from the data transaction; SQLite serializes writers per database, not per row [S1, S2]. |
| Data mutation | `sqlstore.go:790` `Apply` and line 830 `CompareAndApplyFunc` acquire the record-write lease and use a separate transaction on `issueops.db`. | They intentionally do **not** acquire `spanGate`. Adding the gate here would deadlock existing callers already inside a span. |
| Atomicity and visibility | `sqlstore.go:465` `commitData`; `span_coverage_test.go:186`, `:208`, `:234`. | Cancellation checked before commit prevents that commit; cancellation after commit does not undo data. Committed data is visible to another handle before the outer span exits. A span is not a rollback transaction. |
| Conditional writes | `sqlstore.go:830` compares expected raw rows inside the data transaction; `sqlstore_atomic_test.go:41` checks concurrent require-absent writers. | A narrower lock must still protect read/decision/write and cross-row ownership, not merely individual SQL statements. |
| Authority | `record_guard.go:50` binds after acquiring the root span; `:67` rechecks against the mutation's data transaction. `cmd/issueops/issueopsapp/request_store_fence_wiring.go:20` and `:39` nest grant-root before loop/worker root. | Preserve context propagation, grant rechecks, and acquisition order. Multiple roots are not one atomic transaction. |
| Maintenance exclusion | `write_guard_unix.go:11` takes a shared process lease; `write_exclusion.go:13` takes the exclusive counterpart. `internal/adapter/outbound/processlease/acquire_unix.go:28` uses nonblocking flock. | This rejects mutations during destructive exclusion; it does not serialize ordinary writers with one another or replace the span lock. |
| State application | `internal/application/state/service.go:38`, `:142`, `:190` wrap Write/WriteRecord/Update in spans. `:178` Delete directly calls `Mutate`. `prune.go:23` lists, selects, then deletes separately. | State deletion currently lacks the ordering its writers use. Retention also has a selection-to-delete gap. |

The IssueOps record adapter already demonstrates the required ordering:
`internal/adapter/outbound/issueopsrecord/store.go:162` `UpdateRelated`,
`:220` `Delete`, and `:268` `DeleteIfUnchanged` all enter the root span.
The 2026-08-27 deletion incident documents why CAS alone did not prevent
related-state resurrection:
`.issueops/cautions/lessons/2026-08-27-record-delete-bypassed-the-span-gate.md`.

### Cancellation is not one timeout

- Local gate waiting observes the caller context, but a background context
  imposes no local-wait deadline (`sqlstore.go:407`).
- The 60-second retry timer starts inside `beginSpanTxAfterContention`, after
  local gate acquisition. Typed `SQLITE_BUSY`/`SQLITE_LOCKED` retries use
  1, 2, 4, 8, then 10 ms gaps (`sqlstore.go:510`, `:553`, `:557`).
- `Open` has no context. Its 10-second initialization retry timer is checked
  between attempts, while data connections have a 10,000 ms busy handler
  (`sqlstore.go:192`, `:223`). Neither timer bounds time waiting for
  `handlesMu`, nor makes the whole request a strict ten-second operation.
- `Get`/`GetExisting`/`List` use contextless SQL methods
  (`sqlstore.go:567`, `:583`, `:931`). Existing reads configure a 2,000 ms busy
  handler, not a request deadline. `WalkExisting` accepts a context but first
  opens through the contextless `openExistingData` (`:640`, `:746`).
- `Apply` passes the request context through BeginTx, statements, guard
  recheck, and pre-commit checking. That does not establish a measured upper
  bound for cancellation while the driver is executing its busy handler.
  SQLite describes accumulated busy-handler sleeps, not an end-to-end SLA
  [S3]; Go explicitly makes context support driver-dependent [S4].

## Executed baseline

Run from the checked-out revision, with dependencies already available:

```sh
cd /Users/m16khb/Workspace/issueops
git rev-parse HEAD
git status --short
go version
go test ./internal/adapter/outbound/sqlstore \
  ./internal/adapter/outbound/state \
  ./internal/application/state \
  ./internal/adapter/outbound/issueopsrecord \
  -count=1 -timeout=120s
go test ./internal/adapter/outbound/sqlstore -run '^$' \
  -bench '^(BenchmarkOpenCachedRoots|BenchmarkSpanLockShortContentionHandoff|BenchmarkSpanObserverOverhead)$' \
  -benchmem -benchtime=100ms -count=1 -timeout=120s
```

Both Go commands exited 0. These tests/benchmarks use their own temporary
stores; no source fixture or standalone benchmark file was added. Go may
populate its normal build cache. Package results:

```text
ok  issueops/internal/adapter/outbound/sqlstore       5.925s
ok  issueops/internal/adapter/outbound/state          1.913s
ok  issueops/internal/application/state              1.985s
ok  issueops/internal/adapter/outbound/issueopsrecord  1.567s
```

Shipped benchmarks, one 100 ms run, default CPU suffix `-10`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| OpenCachedRoots/roots=1 | 101072 | 4609 | 38 |
| OpenCachedRoots/roots=10 | 250866 | 7488 | 56 |
| OpenCachedRoots/roots=100 | 2223757 | 36321 | 236 |
| SpanLockShortContentionHandoff/adaptive | 2264816 | 3313 | 46 |
| SpanLockShortContentionHandoff/fixed_10ms_baseline | 12310569 | 3290 | 43 |
| SpanObserverOverhead/empty/nil | 19617 | 1678 | 21 |
| SpanObserverOverhead/empty/noop | 49329 | 1648 | 23 |
| SpanObserverOverhead/one_commit/nil | 321482 | 3210 | 53 |
| SpanObserverOverhead/one_commit/noop | 419175 | 3239 | 55 |

The handoff-specific metric was `2120083 handoff_ns/op` for adaptive and
`12241380 handoff_ns/op` for fixed 10 ms. Benchmark package elapsed: 5.069s.

Interpretation:

- **Observed:** cached Open cost grows with retained root count. This agrees
  with the full-cache `os.Stat` scan in `pruneRemovedHandlesLocked`
  (`sqlstore.go:161`; benchmark `resource_test.go:12`). It is not a measurement
  of contention on the root's span gate.
- **Observed:** the adaptive backoff already beats the fixed-gap comparison in
  this forced short-handoff workload (`span_backoff_test.go:29`). Recommending
  that optimization again would duplicate shipped work.
- **Not established:** production root counts, p95/p99, mutex wait attribution,
  or stable observer-overhead percentages. Tests and benchmarks were launched
  concurrently, so their host load was not isolated. Do not turn these single
  samples into a claimed production speedup.
- Existing tests cover canceled waiters, commit cancellation, visibility,
  process death, permissions, and record deletion ordering. Passing them does
  not prove the new interleavings below. Some old process/State lock tests use
  sleeps or negative time windows (`process_mutex_test.go:38`, `:110`,
  `internal/adapter/outbound/state/state_test.go:217`); they are baseline
  evidence, not templates for new deterministic tests.

No full self-verify, race battery, installed-service QA, or mutation against
user state was run: this deliverable is a bounded report, not verified code
delivery. Future implementation still owes the repository's Go verification
gates in `.issueops/TESTING.md`.

## Prioritized, file-bounded tasks

### 1. Preserve acquired span ownership through callback exit

**Priority: high. Status: source-derived correctness defect, not a measured
production incident.**

`runSpan` invokes arbitrary callback work before its deferred rollback
(`sqlstore.go:444-459`), but `beginSpanTxAfterContention` gives the lock
transaction the request context (`:525`). Go automatically rolls back a
transaction when that context is canceled [S4]. The installed Go source
confirms `beginDC` starts `tx.awaitDone`; it does not wait for the application
callback. The installed driver implements Rollback using
`context.Background()` and SQL `rollback`.

Therefore the permitted order is: A acquires lock, A enters callback, A's
request is canceled, Go rolls back A's lock transaction, B on another handle
or process acquires the root, A's callback has not yet returned. The local gate
still protects callers using A's cached handle, which can hide the discrepancy
in ordinary same-handle tests.

This matters beyond canceled data writes, which usually fail through
`Apply`: `internal/adapter/audit/handoff_delivery.go:189` ignores the callback
context while writing, syncing, and reading back an audit file. Its lock is
wired through State at
`cmd/issueops/issueopsapp/handoff_delivery_wiring.go:22`. Cancellation does not
forcibly stop those filesystem operations. Likewise State's transform has no
context parameter (`internal/application/state/service.go:190`).

**Proposed change boundary:** `internal/adapter/outbound/sqlstore/sqlstore.go`,
`span_context_test.go`, `process_crash_test.go`, and, only if observation
assertions need coverage, `span_observer_test.go`. Keep the root lock and data
transactions separate. Give acquired lock ownership a callback-scoped
lifetime independent of request cancellation; retain request cancellation
for acquisition/retries, callback work, and data transactions. Recheck
cancellation around successful acquisition. Do not detach the callback and
return while it still runs.

**Acceptance:** first reproduce actual contender entry while a canceled
holder callback remains blocked on an explicit channel. Observe the
rollback/contender event rather than assuming cancellation means rollback has
finished. Use two uncached handles, then the existing process-helper protocol.
After the fix, prove a real typed busy result while the callback owns the lock
and successful acquisition after explicit callback release; make the
lock-lifetime cancellation boundary directly testable if necessary. Keep
canceled local/SQLite waiters, pre-commit rollback, post-commit persistence,
panic release, and process-kill recovery passing. Channels/driver events are
ordering evidence; bounded deadlines only fail stuck tests, not establish
ordering.

**Risk/trade-off:** a callback that ignores cancellation retains the lock until
it returns or the process dies. That is the price of the advertised
serialization contract; releasing a lock cannot safely cancel its owner's
side effects. Do not apply a detached context to data writes. Existing
`SpanObservation.Hold` records deferred rollback time (`span_observer.go:118`);
today that may overstate actual SQLite ownership after cancellation.

**Constraint fit:** repairs the existing root-wide invariant from
`.issueops/architecture/runtime.md` without changing ports, authority schema,
or CLI/MCP contracts. Supported by the repository call chain plus official Go
transaction lifetime and SQLite lock semantics [S1, S4].

### 2. Put confirmed State retention under the writer's root span

**Priority: high. Status: source-derived stale-selection and ordering gaps.**

`prune.go:33-43` lists and selects before calling unguarded `Delete`.
Permitted schedule: retention selects old key K; a normal `Write` commits a
fresh K; retention deletes K using only its ID. The new checkpoint is removed
using the old timestamp. Independently, `Delete` can finish between Update's
read and commit because it never acquires the span (`service.go:178-221`).
The sequential tests do not exercise either schedule.

**Proposed change boundary:** `internal/application/state/service.go`,
`prune.go`, `service_test.go`, and
`internal/adapter/outbound/state/state_request_context_test.go`.
Make standalone Delete acquire the root span. For confirmed prune, acquire
that same root span **before** listing/selecting and keep it through deletion.
Use the already-open store's mutation operation inside that span rather than
calling the newly locking Delete recursively. Preserve prefix, age, retained
count, dry-run results, and schema rejection. No new CAS port is needed for
this bounded choice. Do not move locking into sqlstore Apply.

The production entry points are
`cmd/issueops/issueopsapp/state_wiring.go:18` (public State dependencies) and
`self_workflow_planning_wiring.go:38` (lesson retention). The selection policy
is `internal/domain/state/prune.go:13`; retained-count selection spans multiple
keys, so a key-only lock is insufficient.

**Acceptance:** a barrier-controlled writer that refreshes K before confirmed
prune acquires its span must leave K kept. A writer attempting to enter after
prune begins selection must enter only after prune finishes; it can then
recreate K normally. Standalone Delete must use the same exclusion as Update.
Use injected storage wrappers/channel barriers to force these orders, plus a
real sqlstore adapter integration test. Preserve the existing age/count/prefix
tests and dry-run no-deletion assertions. An application-only fake whose
`WithSpan` simply calls the callback does not prove mutual exclusion.

**Risk/trade-off:** root hold time now includes listing/decoding and the
selected deletes. Keep current per-delete transaction semantics rather than
silently promising batch rollback. If later measurements show long retention
holds, snapshot/CAS revalidation would require a separate contract decision.
For now, narrower locking would preserve the stale-selection bug.

**Constraint fit:** follows the established IssueOps record-delete fix and
the canonical prohibition on bypassing related writer spans
(`.issueops/cautions/runtime.md`). SQLite's per-statement atomicity alone does
not cover an earlier application selection [S1].

### 3. Fail fast before storage work on already-canceled State requests

**Priority: medium. Status: source-confirmed entry-path gap.**

State's context-taking methods call `OpenStore` before checking cancellation
(`service.go:38`, `:142`, `:178`, `:190`, `:224`). Prune lists before its first
context-aware mutation (`prune.go:23`). `Open` may create the root and both
databases, repair permissions, scan cached roots, or wait under the global
mutex. Thus an already-canceled valid write can perform initialization before
WithSpan finally rejects it. Current cancellation coverage cancels **inside**
Update's transform, after opening (`state_request_context_test.go:25`).

**Proposed change boundary:** `internal/application/state/service.go`,
`prune.go`, `service_test.go`, and
`internal/adapter/outbound/state/state_request_context_test.go`.
After the existing pure input validation and before the first storage call,
return `ctx.Err()` for canceled requests. Cover Write, WriteRecord, Delete,
Update, WithKeyLock, and the common prune path. Keep existing result shapes.
This is deliberately not a context retrofit of every storage interface.

**Acceptance:** cancel before entry; inject an OpenStore counter and assert
zero calls, zero transform/callback calls, and `errors.Is(err,
context.Canceled)`. At the adapter boundary use an absent child of
`t.TempDir()` and prove it remains absent. Include canceled dry-run prune,
which currently has no mutation at which cancellation must be noticed.
Retain existing input-validation precedence and normal operation tests.

**Risk/limit:** closes only pre-canceled entry. Cancellation arriving during
Open, contextless reads, or a busy driver call remains a separate limitation.
Do not claim an end-to-end cancellation latency SLA or a throughput win.

**Constraint fit:** a small common-application fix, consistent with Go's
context propagation guidance [S5], keeps all hosts aligned and avoids a
cross-package OpenContext redesign.

## Lock-granularity alternatives: defer, do not implement

| Alternative | Can it preserve the invariants? | Decision |
|---|---|---|
| Per-key local gates, unchanged `issueops.lock.db` | The SQLite write transaction still serializes all keys [S1]. State retained-count pruning and grant-root fencing still need wider coordination. | No demonstrated concurrency gain; do not change. |
| Per-key lock databases or removal of the SQLite span | Only after defining cross-key/bucket ownership, deletion ordering, cross-process protocol, acquisition order, and old/new participant compatibility. | Redesign; rejected as this report's task scope. The historical per-entity-lock rejection is recorded in `.issueops/cautions/lessons/2026-07-07-sqlite-sqlstore-span-discipline.md`. |
| One data transaction for the whole callback | Would change visibility before callback exit, preserve/rollback behavior after errors, and external-effect duration under a data write lock. | Breaks current tests/contracts; not an optimization. |
| Narrow global `handlesMu`, preserve per-root span | Possible in principle, since cache ownership and record serialization are different concerns. Requires safe publication, same-root initialization coordination, eviction/CloseRoot ordering, and unchanged permission checks. | The growing Open benchmark makes this a future measurement candidate, not an approved fourth task. `resource_test.go:40` explicitly requires pruning other removed roots even on a live cache hit. Simply skipping the scan changes behavior. |
| Increase data-pool connections or rely on WAL | WAL permits reader/writer concurrency, not simultaneous database writers [S2]. | No evidence this fixes the measured cache cost or callback cancellation gap. |

Existing `WithSpanObserver` already separates wait/hold/callback/commit and
marks unattributed writes unknown (`span_observer.go:25`, `:50`, `:126`;
`sqlstore.go:484`). Use that existing evidence surface before proposing any
future granularity change. Its phases overlap and must not be summed; its
commit coverage is handle-local, not proof that every process's writes were
observed.

## Official source index and cross-check

Retrieved on 2026-10-03; all URLs below were fetched successfully.

| ID | Official source | Assertion checked |
|---|---|---|
| S1 | https://www.sqlite.org/lang_transaction.html | One simultaneous write transaction; BEGIN IMMEDIATE can return SQLITE_BUSY; transactions persist to commit/rollback; implicit SQL transactions do not enclose application-side selection. |
| S2 | https://www.sqlite.org/wal.html | WAL supports concurrent readers and a writer, but still only one writer at a time. |
| S3 | https://www.sqlite.org/c3ref/busy_timeout.html | Busy timeout installs repeated sleeps; nonpositive values disable the handler. |
| S4 | https://pkg.go.dev/database/sql#DB.BeginTx | BeginTx context lasts through transaction end; cancellation triggers rollback. Package documentation also states unsupported driver cancellation waits for query completion. |
| S5 | https://go.dev/doc/database/cancel-operations | Propagate caller contexts to context-aware database APIs; cancellation is cooperative through those APIs. |

Source fan-out: official SQLite transaction/WAL/busy-handler documentation,
official Go SQL/cancellation documentation, installed Go implementation, and
the exact module-cache driver. Context7 `/golang/go/go1.27.1` independently
located the `beginDC`/`awaitDone` implementation; local source confirmed it.

Local upstream implementation checks, reproducible on this toolchain:
`$(go env GOROOT)/src/database/sql/sql.go:1864`, `:1904`, `:2214`;
`$(go env GOMODCACHE)/modernc.org/sqlite@v1.53.0/tx.go:19`, `:57`;
the driver's `conn.go:1294` forwards BeginTx into that transaction constructor.
These are local installed-source paths, not repository files.

Claim verification: official API guarantees cross-checked against current
implementations where they drive a proposal; repository call paths read
directly. Performance claims limited to the printed samples. New failure
schedules remain explicitly source-derived pending their acceptance tests.
Access boundary: no login, paywall, external secret, or live-service access
was needed. The only authored artifact is this report.

## Independent recheck: 2026-10-03

This section records a second investigation at the same revision. The report
above already existed when this investigator inspected the destination, so it
has been preserved rather than overwritten. Its earlier command results are
not attributed to this pass. The independently executed evidence below supports
the same three proposals; it does not add a fourth task.

**Workspace provenance.** This pass initially observed untracked
`.issueops/drafts/2026-10-03-harness-improvement.md` and the research directory,
not an empty status. Other sessions subsequently changed projectdoc files and
created a plan. This pass made no such changes. Its only edit is this appended
report section, through `apply_patch`. Source revision remained
`3e03b21bcf80b29047f62237d82ce8a4779d0efa`; Go was
`go1.27.1 darwin/arm64` on Apple M4.

### Independently executed baseline

Exact commands, from `/Users/m16khb/Workspace/issueops`:

```sh
go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/state ./internal/application/state ./internal/adapter/outbound/issueopsrecord -count=1 -timeout=90s
go test ./internal/adapter/outbound/sqlstore -run '^$' -bench '^(BenchmarkOpenCachedRoots|BenchmarkSpanObserverOverhead|BenchmarkSpanLockShortContentionHandoff)$' -benchtime=100x -benchmem -count=1 -timeout=90s
```

Both exited 0. These are existing tests using temporary stores, not new files
or operations against installed service/user state. Package output:

```text
ok  issueops/internal/adapter/outbound/sqlstore       5.814s
ok  issueops/internal/adapter/outbound/state          0.832s
ok  issueops/internal/application/state              0.203s
ok  issueops/internal/adapter/outbound/issueopsrecord  1.548s
```

All benchmark rows used 100 iterations, CPU suffix `-10`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| OpenCachedRoots/roots=1 | 21655 | 4528 | 38 |
| OpenCachedRoots/roots=10 | 36978 | 7520 | 56 |
| OpenCachedRoots/roots=100 | 246244 | 36320 | 236 |
| SpanLockShortContentionHandoff/adaptive | 1345125 | 3371 | 46 |
| SpanLockShortContentionHandoff/fixed_10ms_baseline | 11445498 | 3297 | 43 |
| SpanObserverOverhead/empty/nil | 18536 | 1632 | 21 |
| SpanObserverOverhead/empty/noop | 19132 | 1648 | 23 |
| SpanObserverOverhead/one_commit/nil | 122102 | 3277 | 53 |
| SpanObserverOverhead/one_commit/noop | 73853 | 3332 | 56 |

Handoff metrics were `1316956 handoff_ns/op` and
`11360426 handoff_ns/op`, respectively. Benchmark package elapsed: 2.570s.
The test and benchmark commands overlapped. In particular, the faster
`one_commit/noop` sample is noise-compatible, not evidence that observers
accelerate writes. No latency percentile, production contention rate, or
before/after improvement is established by either pass.

### Cross-check and implementation qualifications

- **Proposal 1 remains high priority.** Re-read
  `sqlstore.go:407-459`, `:510-550`, and the installed Go
  `database/sql/sql.go:1904-1932`, `:2212-2227`. The latter launches
  `tx.awaitDone` and rolls back on cancellation independently of the callback.
  Re-read modernc v1.53.0 `tx.go:19-32`, `:56-79` and
  `conn.go:1293-1301`: request context reaches BEGIN, while rollback executes
  with a background context. The callback-outliving-lock schedule is
  source-derived; this pass did not execute a new failing regression.
  Before changing lock lifetime, explicitly prove cancellation racing with a
  successful acquisition releases an unused transaction and never invokes the
  callback. Acquisition must remain cancelable; only acquired lock ownership
  should survive cancellation until callback exit.
- **Proposal 2 needs both parts.** Re-read State `service.go:178-232`,
  `prune.go:23-50`, and `internal/domain/state/prune.go:13-62`.
  A Delete completing during an overlapping Update is not, by itself, proof
  of a non-linearizable abstract map: overlapping operations can legally
  linearize Delete first. The stronger concern is the repository's explicit
  span contract and retention using an old selection to delete a newly
  refreshed record. Merely wrapping Delete in a span does not close that
  selection-to-delete gap. Confirmed pruning must serialize selection and
  deletion, as proposed. Its guarantee is relative to participating State
  writers, not arbitrary raw `DB.Put` callers that bypass spans.
- **Proposal 3 remains deliberately narrow.** Re-read the context-taking
  State entry points and `state_request_context_test.go:25`. The existing
  test cancels inside the transform, not before OpenStore. The proposed
  pre-entry tests must prove zero storage calls and an absent root remaining
  absent. Passing them would not establish cancellation responsiveness after
  initialization begins.

Reference precision: `WithSpanObserver` starts at
`internal/adapter/outbound/sqlstore/span_observer.go:49`; line 50 is its first
guard. The remaining cited local symbol anchors were checked against current
source, including authority bind/recheck, record deletion, State composition,
retention policy, and the existing benchmark/test functions.

**External verification.** This pass retrieved the official pages
https://sqlite.org/lang_transaction.html,
https://sqlite.org/wal.html,
https://sqlite.org/c3ref/busy_timeout.html,
https://pkg.go.dev/database/sql#DB.BeginTx, and
https://go.dev/doc/database/cancel-operations on 2026-10-03. Context7 used
`/websites/sqlite_docs`; the earlier Go Context7 lookup above belongs to the
pre-existing report, not this pass. SQLite's one-writer rule and Go's
automatic rollback contract were directly confirmed. SQLite pages share one
publisher, so they are not independent corroborating sources; the relevant
cross-check is against Go/driver implementation and repository call paths.

**Completion boundary.** Retain root-wide record serialization. Defer
per-key locking and handle-cache redesign despite the measured cached-open
scaling. No production/test implementation, commit, full self-verify, race
battery, service operation, or user configuration modification was performed
by this pass. The requested deliverable is the report and its bounded
acceptance criteria, not a claim that the proposed fixes already work.
