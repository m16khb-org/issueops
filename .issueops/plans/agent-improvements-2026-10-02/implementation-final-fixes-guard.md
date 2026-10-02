# Final-review fixes F1 and F3: root-scoped record guard with a write-time recheck

Scope: `final-review.md` F1 (valid capabilities cannot resume or reseed over HTTP) and F3 (reconcile commits bypass grant revalidation). Both are fixed by one sqlstore guard change, built to the required design (a)+(b)+(c). No other mechanism was substituted. F2, F4, and F5 are not addressed here.

## Root cause (both defects)

`sqlstore.WithRecordGuard` armed the request's authority binder for "the next span on any root". It consumed the binder there, and nothing ran it again after that.

- **F1:** resume and reseed open the reseed-fence DB (`<state>/issueops-reseed-fence/<sha256(id)>`) first. That span consumed the guard and checked the grant against the fence DB's reader. The fence DB holds no grant, so every valid capability got `authority_invalid: ... revoked or never issued`.
- **F3:** reconcile's later receipt and clear writes are raw `CompareAndApply` calls outside any span. Data writes never consulted the guard, and grant rotation or removal changes neither CAS input.

## Design implemented

(a) **Root-scoped guard.** `WithRecordGuard(ctx, root, bind)` now records the canonical root it protects (`filepath.Abs`, which matches `sqlstore.Open`'s `d.dir` derivation). `bindRecordGuard` binds only when `d.dir == root`. Spans on other roots (reseed fence, loop store, worker store) pass the guard through untouched. The guard is no longer consumed, because a nested span on the same root is already rejected by the span chain (`NestedSpanError`) before the guard runs. `bindIssueOpsAuthority` passes `issueOpsStateRoot()`, which is the grant database root.

(b) **Write-time recheck.** `Apply` (and therefore `Mutate`) and `CompareAndApplyFunc` (and therefore `CompareAndApply`) call `d.recheckRecordGuard(ctx, tx)` after `applyMutationsTx` and immediately before `commitData`. On the guarded root, this re-runs the same binder (`authorityapp.Service.BindSpan`, i.e. the existing `check`: record lookup, token digest, scope, expiry, and live session). It runs against a `txRecordReader` backed by the same data transaction (`_txlock=immediate`), so the write already holds the SQLite write lock. Grant rotation or removal also needs that lock, so the two serialize. On failure the transaction rolls back and the authority error is returned unchanged. Contexts without a guard (native stdio, CLI, `Issue`) and writes on other roots are unaffected. No grant validation rule is duplicated.

(c) **Loop/worker fences.** `grantFencedLoopStore` and `grantFencedWorkerStore` open a grant-root span (`grantRoot: issueOpsStateRoot()`), which equals the guard root, so they still bind there. Their existing tests pass (see GREEN below).

Claim token, holder, generation, and raw lifecycle/intent CAS checks are unchanged. Server process ancestry is not used as HTTP caller identity.

## Files

Production:
- `internal/adapter/outbound/sqlstore/record_guard.go`: `recordGuard{root, bind}`, root-scoped `WithRecordGuard`, `bindRecordGuard` without consumption, plus `recheckRecordGuard` and `txRecordReader`.
- `internal/adapter/outbound/sqlstore/sqlstore.go`: recheck before `commitData` in `Apply` and `CompareAndApplyFunc`.
- `cmd/issueops/issueopsapp/authority_wiring.go`: `WithRecordGuard(bound, issueOpsStateRoot(), service.BindSpan)`.

Tests:
- New: `cmd/issueops/issueopsapp/mcp_http_lifecycle_parity_test.go` (`TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions`, F1).
- New: `internal/adapter/outbound/issueopslease/reconcile_guard_test.go` (`TestReconcileReceiptRechecksRequestGuardAfterGrantChange`, removed and rotated subtests, F3; the reviewer's probe committed as a repo test, now also asserting that the record and intent bytes are unchanged).
- New test in `cmd/issueops/issueopsapp/request_store_fence_wiring_test.go`: `TestBoundGrantRootWriteRechecksGrantAfterRotation` (F3 with the real authority service and a real `mcp authorize` rotation; `Apply` and `CompareAndApply` are rejected and persist nothing; an unbound native write still succeeds).
- New tests in `internal/adapter/outbound/sqlstore/record_guard_test.go`: `TestWithRecordGuardPassesOtherRootsThrough` and `TestWithRecordGuardRechecksGuardedWritesBeforeCommit`. The latter covers `Apply`, `CompareAndApply`, `CompareAndApplyFunc`, and an in-span write. It also checks that a write revoking its own grant is rejected (proving the recheck sees the applied mutations) and rolled back, and that unguarded writes succeed.
- Signature updates only: `record_guard_test.go` (existing tests pass `database.dir`; the bind test now separately counts span binds = 1 and write rechecks = 1 instead of total binder calls = 1), `cmd/issueops/issueopsapp/mcp_http_command_test.go:329`, and `internal/adapter/issueops/execution_publication_store_guard_test.go` (the guard root is the store's state root).
- `internal/adapter/outbound/issueopslease/resume_repository_test.go`: `seededResumeIntent` now delegates to a new `seededResumeIntentAt`, which also returns the state root. Existing callers are unchanged.
- `cmd/issueops/issueopsapp/authority_wiring_test.go`: see the contract change below.

**Contract change in one existing assertion:** `TestMCPAuthorizeIssuesPreLeaseCapabilityAndGuardRechecksEachSpan` previously required a span on a foreign root to be *rejected* ("foreign-root span was authorized"). That is exactly the F1 mechanism (consulting a non-grant root for the grant), and design (a) reverses it. The assertion now checks the property that should still hold: a foreign root is neither bound nor an authority source. Its span runs, but inside it `Verify` still rejects the revoked capability (`authority_invalid`) against the grant root and accepts the fresh one. The rest of that test is unchanged and passes, including grant-root rejection after rotation with zero callback entries.

Not touched: `internal/architecture/testdata/ddd_responsibility_inventory.json` (its ` M` status predates this work), the Windows docs, `openwiki/`, the real `$HOME`, launchd, and repo `bin/issueops`. No commit or push.

## RED (before the production change)

```sh
go test ./cmd/issueops/issueopsapp -run '^TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions$' -count=1
```
Exit 1:
```text
mcp_http_lifecycle_parity_test.go:62: HTTP resume payload=map[error:authority_invalid: authority credential was revoked or never issued ok:false] isError=true, want native decision "execution resume requires an existing Orca binding"
mcp_http_lifecycle_parity_test.go:62: HTTP reseed payload=map[error:authority_invalid: authority credential was revoked or never issued ok:false] isError=true, want native decision "reseed requires a released or claimable lease"
```
The native decisions come from the same handlers, called in the same test with an unbound context and the native holder actor.

```sh
go test ./cmd/issueops/issueopsapp -run '^(TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions|TestBoundGrantRootWriteRechecksGrantAfterRotation)$' -count=1
```
Exit 1. This run also includes `request_store_fence_wiring_test.go:130: write after rotation err = <nil>, want revoked grant`.

```sh
go test ./internal/adapter/outbound/issueopslease -run '^TestReconcileReceiptRechecksRequestGuardAfterGrantChange$' -count=1
```
Exit 1:
```text
--- FAIL: .../removed  reconcile_guard_test.go:56: receipt after grant change: err=<nil> next_stage=run_create, want the guard error
--- FAIL: .../rotated  reconcile_guard_test.go:56: receipt after grant change: err=<nil> next_stage=run_create, want the guard error
```
The RED version used the old two-argument `WithRecordGuard(ctx, bind)`. After the signature change it passes the seeded state root; the assertions are unchanged.

## GREEN (after the change)

| Command | Exit |
|---|---|
| `go build ./... && go vet ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopslease ./cmd/issueops/issueopsapp ./internal/adapter/issueops` | 0 |
| `go test ./cmd/issueops/issueopsapp -run '^(TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions\|TestBoundGrantRootWriteRechecksGrantAfterRotation\|TestScoped.*\|TestMCPHTTP.*)$' -count=1` (includes the existing loop/worker grant-fence tests) | 0 |
| `go test ./internal/adapter/outbound/issueopslease -run '^TestReconcileReceiptRechecksRequestGuardAfterGrantChange$' -count=1 -v` (removed and rotated subtests PASS) | 0 |
| `go test ./internal/adapter/outbound/sqlstore -run 'RecordGuard' -count=1 -v` (5 tests PASS) | 0 |
| `go test ./internal/adapter/issueops -run 'TestRemotePublicationTransactionKeepsRequestGuard' -count=1` | 0 |
| `gofmt -l $(git ls-files '*.go'; git ls-files --others --exclude-standard '*.go')` | 0, no output |
| `go vet ./...` | 0 |
| `go test -race ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopslease ./internal/application/issueopslease ./cmd/issueops/issueopsapp ./cmd/issueops/mcpcli ./internal/adapter/outbound/authority ./internal/application/authority -count=1` | 0, all 7 packages ok, no DATA RACE (`/tmp/ff-race2.log`) |

The first race run failed only on the `authority_wiring_test.go` foreign-root assertion described above. It is listed as the contract change, not hidden.

## Real-binary HTTP check

Binary: `go build -o /tmp/ff-bin/issueops ./cmd/issueops` (exit 0).

The seed/receipt helper was `/tmp/ff-bin/helper.go`, built with `-overlay` (same approach as the reviewer's helper; no repository file was added). It writes a generation-1, direct-mode, active execution with the production `WriteIssueOps`.

Environment was isolated under `/private/tmp/ff-realcheck`: `HOME`, `ISSUEOPS_STATE_DIR`, `CODEX_HOME`, `XDG_*`, `ISSUEOPS_ROOT` unset. The server cwd was the empty `server-cwd`.

1. Session process `/tmp/ff-bin/session.sh` (bash PID 90124) ran `issueops mcp authorize --workspace-root <repo> --host codex --session-id ff-a --session-pid $$ ...` and printed GRANT1 (`ok:true`). It then seeded `io-50c5766f6c04`.
2. `issueops mcp --http --addr 127.0.0.1:0` reported ready at `http://127.0.0.1:52346/mcp` (pid 90720). Real JSON-RPC POSTs (Bun `fetch`) were sent with the bearer, `initialize` returned 200, and `issueops_execution` was called:

| Call | Grant | Result |
|---|---|---|
| `status` | GRANT1 | OK; lease generation 1, active, holder `ff-a`/90124 |
| `resume` (gen 1, worktree cwd, confirm) | GRANT1 | `isError`, `execution resume requires an existing Orca binding` (domain) |
| `replace` + `replace_action: reseed` | GRANT1 | `isError`, `reseed requires a released or claimable lease` (domain) |
| `release` (gen 1) | GRANT1 | `ok:true`, lease `released` |
| `replace`/`reseed` | GRANT1 | `isError`, `stale replacement inventory fingerprint` (domain; the same decision the reviewer saw natively) |
| `resume` | GRANT1 | `isError`, `execution resume requires an existing Orca binding` |
| — | rotation: the session re-authorized and printed GRANT2 | |
| `resume` / `reseed` | GRANT1 (rotated) | `error_code: authority_invalid`, `authority credential was revoked or never issued` |
| — | second live session (bash PID 91764) authorized and printed GRANT3 | |
| `resume` | GRANT3 | `execution resume requires an existing Orca binding` |
| `replace`/`reseed` | GRANT3 | `stale replacement inventory fingerprint` |
| `resume` / `reseed` | GRANT2 (rotated) | `error_code: authority_invalid`, `... revoked or never issued` |
| `status` | GRANT3 | lease generation 1, `released`, no holder |

Two caveats about this run:
- **Second session:** the first session consumed both rotation reads, so it exited after GRANT2. That is why GRANT3 came from a second live session. GRANT1's rejection is the token-mismatch path, which is checked before liveness.
- **Shutdown:** SIGTERM made the server exit 0, and `server-cwd` stayed empty.

## Residual notes

- The write-time recheck runs the existing `check`, including the live session-process observation, inside the data transaction. Every capability-bound write on the grant root therefore pays one extra process inspection while it holds the write lock. This is deliberate, because it is what serializes grant rotation with the write.
- A same-root span still binds once at span start, and every write inside it rechecks again. `Verify` inside the span continues to use the span's locked reader.
- A test that exercises a live Orca reconcile end to end was not run. F3 is covered by the real-SQLite reconcile repository test, the real-authority rotation test, and the sqlstore tests.
