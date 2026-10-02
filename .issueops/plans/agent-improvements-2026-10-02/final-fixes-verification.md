# Independent verification of final-review fixes F1-F5

**Overall verdict: PASS.** All five defects reproduce as fixed on a fresh `/tmp` build with isolated HOME/state, every repository check is green (gofmt clean, vet 0, `go test ./...` 0 with 323 ok, `go test -race ./...` 0 with 323 ok and no DATA RACE, build 0, both golden suites 0), and the only shared artifact that needed regeneration (`ddd_responsibility_inventory.json`) was regenerated once with a delta consisting solely of symbols introduced by these fixes. No remaining defect was found.

Base HEAD `92eaa00143841f964c285dacdd41aedbeeecdf2e`, uncommitted working tree, Go 1.26.4 darwin/arm64. Verification date 2026-10-03 (KST). Verifier binary `/tmp/ffv/bin/issueops`; repository `bin/issueops` untouched. No commit, no push, real `$HOME` untouched, no launchd label touched, no edit to production code, to `.issueops/plans/wmux-windows-port.md`, `.issueops/research/2026-10-02-ghostty-cmux-windows.md`, or `openwiki/`.

Inputs read before any command: `final-review.md`, `contract.md` §2, `implementation-final-fixes-{guard,apidoc,evidence}.md`, `git status`/`git diff` of the tree, and the fix files named in each note (`internal/adapter/outbound/sqlstore/record_guard.go`, `sqlstore.go` diff, `cmd/issueops/issueopsapp/authority_wiring.go`, `cmd/issueops/mcpcli/mcp_request_scope.go`, `mcp_facade.go` diff, `internal/adapter/inspect/receipts.go`, `cmd/issueops/mcpcli/mcp_sdk_server.go` diff, and the new tests `mcp_http_lifecycle_parity_test.go`, `reconcile_guard_test.go`, `authority_wiring_test.go`).

## Per-defect verdicts

| Defect | Verdict | Evidence |
|---|---|---|
| F1 valid capability cannot resume/reseed over HTTP | **PASS** | Real HTTP: GRANT1 `resume` -> `execution resume requires an existing Orca binding`; `replace/reseed` on the active lease -> `reseed requires a released or claimable lease`; after `release` (lease `released`) `reseed` -> `stale replacement inventory fingerprint` (the exact decision the reviewer saw natively). Rotated GRANT1 -> `authority_invalid: authority credential was revoked or never issued` for both actions; GRANT2 reaches the domain decisions. `TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions` passes. |
| F2 API-doc HTTP reads outside workspace | **PASS** | Real HTTP `api_doc_review` with a repo-scoped grant: `prompt_file: ../outside-prompt.md`, absolute outside path, symlink `linked-prompt.md` -> outside, and `diff_file: ../outside-prompt.md` all return `isError:true`, `authority_invalid: <field> is outside the authorized workspace`, with `REVIEW_OUTSIDE_SCOPE_MARKER_7581` absent from the response. `prompt_file: inside-prompt.md` returns `verdict: pending`, `reason: host_agent_result_required`, inside marker present. Server cwd (`/private/tmp/ffv/real/server-cwd`) stayed empty. |
| F3 reconcile commit bypasses grant revalidation | **PASS** | `TestReconcileReceiptRechecksRequestGuardAfterGrantChange` (removed + rotated) PASS against real SQLite, asserting record and intent bytes unchanged; `TestBoundGrantRootWriteRechecksGrantAfterRotation` PASS (real `mcp authorize` rotation); 5 sqlstore `RecordGuard` tests PASS. Independent RED proof: a `-overlay` build that strips both `recheckRecordGuard` calls from `sqlstore.go` (tree untouched) makes the same test FAIL with the reviewer's symptom `err=<nil> next_stage=run_create`. |
| F4 failed host evidence promoted to verified | **PASS** | `inspect --json --host-receipts` with the reviewer's failed artifact (`{"type":"tool_result","name":"docs_index","isError":true,...,"protocolVersion":"2026-07-28"}`): all three hosts `discovered=unknown/receipt_artifact_missing_evidence`, `connected=unknown/receipt_artifact_missing_evidence`, `protocol=unknown/receipt_artifact_missing_revision`. Same result for the reviewer's own `$R/failed-host-receipt.json` against `$R/home` (claude row; codex/omo `no_receipt_for_host`). Real artifacts (`kinsp` and `receipt-art` sets): discovered/connected `verified` for all hosts; protocol `verified` only for Omo, whose artifact carries the negotiated revision in a response field. No receipts: `not_checked/host_receipt_required`. |
| F5 HTTP initialize describes a stdio process | **PASS** | Real HTTP `initialize` (200, `protocolVersion` 2025-11-25) returns `This MCP endpoint is the shared local service used by every host session, not a per-host process. Workspace tools need an authority_file issued by 'issueops mcp authorize'. ...`. Stdio `initialize` on the same binary still returns the in-process text (unchanged, as intended). `TestInitializeInstructionsFollowTransport` is in the green suite. |

## Commands and exit codes

`$X` = `/private/tmp/ffv/real` (HOME, ISSUEOPS_STATE_DIR, CODEX_HOME, XDG_* all below it; `ISSUEOPS_ROOT` unset). `$R` = `/tmp/issueops-final-review-01a0fd59` (reviewer's retained fixtures). Logs under `/tmp/ffv/logs/`.

| # | Command | Exit | Result |
|---|---|---|---|
| C1 | `gofmt -l $(git ls-files '*.go'; git ls-files --others --exclude-standard '*.go')` | 0 | no output |
| C2 | `go test ./internal/architecture -run 'TestDDDResponsibilityInventoryMatchesSource\|TestDDD' -count=1` (before regen) | 1 | `production file or symbol has no recorded responsibility` (expected: fix symbols not yet inventoried) |
| C3 | `go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -update-ddd-inventory` | 0 | inventory regenerated once; delta reviewed below |
| C4 | `go test ./internal/architecture -count=1` (after regen) | 0 | `ok` |
| C5 | `go build -o /tmp/ffv/bin/issueops ./cmd/issueops` | 0 | verifier binary |
| C6 | `go vet ./...` (`GOFLAGS=-p=4`, log `vet.log`) | 0 | no output |
| C7 | `go test ./... -count=1` (log `test.log`) | 0 | 323 `ok` packages, 0 `FAIL` |
| C8 | `go test -race ./... -count=1` (log `race.log`) | 0 | 323 `ok` packages; `grep -E 'DATA RACE\|^FAIL\|--- FAIL'` empty |
| C9 | `go build -overlay /tmp/ffv/overlay.json -o /tmp/ffv/bin/helper ./cmd/issueops` | 0 | seed/receipt helper (replaces `cmd/issueops/main.go` only in the overlay; no repo file added) |
| C10 | `go test ./internal/adapter/outbound/issueopslease -run '^TestReconcileReceiptRechecksRequestGuardAfterGrantChange$' -count=1 -v` | 0 | `removed` PASS, `rotated` PASS |
| C11 | `go test ./cmd/issueops/issueopsapp -run '^(TestBoundGrantRootWriteRechecksGrantAfterRotation\|TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions\|TestMCPAuthorizeIssuesPreLeaseCapabilityAndGuardRechecksEachSpan)$' -count=1 -v` | 0 | all three PASS |
| C12 | `go test ./internal/adapter/outbound/sqlstore -run RecordGuard -count=1 -v` | 0 | 5 PASS (`BindsAfterLockAndFeedsCallback`, `FailureStopsCallbackAndReleasesLock`, `IgnoresNilArguments`, `PassesOtherRootsThrough`, `RechecksGuardedWritesBeforeCommit`) |
| C13 | `go test -overlay /tmp/ffv/norecheck-overlay.json ./internal/adapter/outbound/issueopslease -run '^TestReconcileReceiptRechecksRequestGuardAfterGrantChange$' -count=1 -v` | 1 | deterministic RED without the recheck: both subtests `receipt after grant change: err=<nil> next_stage=run_create, want the guard error` |
| C14 | `issueops inspect --json --host-receipts /tmp/f45/work/receipts-failed.json` (HOME=/tmp/f45/home, stubs on PATH) | 0 | all hosts unknown (F4 table) |
| C15 | same with `receipts-real.json` | 0 | discovered/connected verified; protocol verified for omo only |
| C16 | same with `receipts-real2.json` | 0 | same as C15 |
| C17 | `issueops inspect --json` (no receipts) | 0 | `not_checked/host_receipt_required` |
| C18 | `issueops inspect --json --host-receipts $R/failed-host-receipt.json` (HOME=$R/home) | 0 | claude: unknown/missing_evidence x2, missing_revision |
| C19 | stdio: `printf '<initialize>' \| issueops mcp` | 0 | in-process instructions (stdio unchanged) |
| C20 | `go test ./cmd/issueops/contractgolden -run Golden -count=1` | 0 | `ok` |
| C21 | `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` | 0 | `ok` (no golden regenerated) |
| C22 | `kill -TERM <server pid 20769>`; session `quit` | 0 / 0 | both processes exited; `server-cwd` empty |

### Real-surface matrix (one fresh server, real JSON-RPC POSTs via Bun `fetch`, bearer from `$X/state/mcp-http/bearer`)

Session: `bash /tmp/ffv/session.sh` (PID 20706, live for the whole run) ran `issueops mcp authorize --workspace-root $X/repo --host codex --session-id ffv-a --session-pid $$ --session-started-at <observed> --session-executable <observed> --json` -> GRANT1, then seeded `io-3db668df2a92` (generation 1, direct mode, active, holder `ffv-a`/20706) through the production `WriteIssueOps` via the overlay helper. Server: `issueops mcp --http --addr 127.0.0.1:0` from the empty `$X/server-cwd`, ready at `http://127.0.0.1:55439/mcp` pid 20769. Raw results: `/tmp/ffv/logs/http-repro.json`, `http-repro-extra.json`.

| Call | Grant | HTTP | Result |
|---|---|---|---|
| `initialize` | - | 200 | shared-service instructions (F5) |
| `status` | GRANT1 | 200 | ok; lease gen 1 active, holder ffv-a/20706 |
| `resume` (gen 1, worktree cwd, confirm) | GRANT1 | 200 | isError, `execution resume requires an existing Orca binding` |
| `replace`+`reseed` | GRANT1 | 200 | isError, `reseed requires a released or claimable lease` |
| rotation: session re-ran authorize -> GRANT2 (GRANT1 file still on disk, secret revoked) | | | |
| `resume` / `reseed` | GRANT1 | 200 | `error_code: authority_invalid`, `revoked or never issued` |
| `resume` / `reseed` | GRANT2 | 200 | Orca binding / released-or-claimable decisions |
| `status` | GRANT2 | 200 | gen 1 active, same holder |
| `release` (generation 1) | GRANT2 | 200 | ok, lease `released` |
| `replace`+`reseed` | GRANT2 | 200 | isError, `stale replacement inventory fingerprint` |
| `replace`+`reseed` | GRANT1 (revoked) | 200 | `authority_invalid` |
| `api_doc_review` prompt `../outside-prompt.md` | GRANT2 | 200 | `authority_invalid: prompt_file is outside the authorized workspace`, no marker |
| `api_doc_review` prompt absolute outside | GRANT2 | 200 | same |
| `api_doc_review` prompt `linked-prompt.md` (symlink out) | GRANT2 | 200 | same |
| `api_doc_review` diff `../outside-prompt.md` | GRANT2 | 200 | `authority_invalid: diff_file is outside the authorized workspace`, no marker |
| `api_doc_review` prompt `inside-prompt.md` | GRANT2 | 200 | `verdict: pending`, inside marker present |

Incidental observation, kept because it is evidence: in a first attempt the eval kernel killed the session process at cell end, and every GRANT1 call was then rejected with `authority_invalid: authorized session process: native process identity is not live: pid=9297 status=dead`. That is the live-session check doing its job; the run was redone with a persistent session.

## Race

`go test -race ./... -count=1` (`GOFLAGS=-p=4`, monitor `mon_JS3TP9Q2E68YG74X`, log `/tmp/ffv/logs/race.log`, 398 lines): exit 0, 323 `ok` packages, `grep -E 'DATA RACE|^FAIL|--- FAIL'` returned nothing. `internal/domain/artifactreadability` (including the wall-clock `TestCheckLargeBodyIsFast`) passed inside the full parallel run, so no isolated rerun was needed. Note: another agent's postfix battery (`/tmp/issueops-ten-improvements-01a0fa31`) was running its own `go test -race ./...` on the same tree at the same time (observed via `pgrep`); that adds load but no dependency, and its outcome is not part of this report.

## Shared artifacts

- `internal/architecture/testdata/ddd_responsibility_inventory.json`: regenerated once (C3). Full delta against the pre-regen copy (`/tmp/ffv-inventory-before.json`), every line a symbol introduced by these fixes:
  - `cmd/issueops/mcpcli/mcp_request_scope.go` (F2): `confineFile`, `confineRequestFiles`, `requestFileValues`, `resolveExistingAncestor`, `type confinedFiles`
  - `internal/adapter/inspect/receipts.go` (F4): `artifactScan.consume`, `consumeClaudeStream`, `consumeCodexStep`, `consumeCodexToolResult`, `consumeCodexWire`, `consumeJSONRPCResponse`, `consumeOmoTransport`, `isDocsIndexTool`, `listsDocsIndex`, `readArtifactFacts`, `type artifactFacts`, `type artifactScan`
  - `internal/adapter/outbound/sqlstore/record_guard.go` (F1/F3): `DB.recheckRecordGuard`, `DB.recordGuard`, `txRecordReader.Get`, `type recordGuard`, `type txRecordReader`
- `ddd_contract_function_roles.json`: not touched (no new contract function; architecture package passes).
- `cmd/issueops/testdata/*.golden.*`: not regenerated; both golden suites pass as-is (C20, C21).

## Review of the fixes themselves

- F1/F3 mechanism (`record_guard.go`, `sqlstore.go`): `WithRecordGuard(ctx, root, bind)` binds only when `d.dir == root` (grant root), is not consumed, and `Apply`/`CompareAndApplyFunc` call `recheckRecordGuard` with a `txRecordReader` on the open `_txlock=immediate` transaction before `commitData`. Claim token, holder, generation, cwd/confirm, and raw lifecycle/intent CAS remain in the existing handlers; the reseed fence DB and loop/worker stores pass the guard through. Server process ancestry is not used anywhere as HTTP caller identity (`authority_wiring.go` binds via `service.Bind` + `service.BindSpan` only).
- One existing assertion changed (`authority_wiring_test.go:118-140`): the foreign-root span was previously required to be rejected; it is now required to run (guard passed through) while `Verify` inside it still rejects the revoked capability with `authority_invalid` and accepts the fresh one. The rejection of a foreign root as an authority source was the F1 mechanism itself, so this is a necessary contract change that keeps the security property, not a weakening. No test was deleted.
- F2 (`mcp_request_scope.go`): `confineRequestFiles` runs inside `NewRequestScope` before credential read, uses `os.OpenRoot(scope.WorkspaceRoot)` + `resolveExistingAncestor` + `filepath.IsLocal` + `root.Stat`, opt-in for `api_doc_review` (`diff_file`, `prompt_file` cwd-relative; `files`, `result_file` root-relative) and `api_doc_static_check` (`files`). Native CLI and capability-less stdio are untouched.
- F4 (`receipts.go`): substring search replaced by per-line JSONL parsing of four recognized shapes; any failed docs_index result demotes Connected to `receipt_artifact_tool_failed`; a revision counts only from a response field; unrecognized formats stay unknown.
- F5 (`mcp_sdk_server.go:33-41`): `httpServerInstructions` set only when `transport == transportHTTP`.

## Residual notes (not defects against the required corrections)

- F2 leaves a same-user TOCTOU between `root.Stat` and the later `os.ReadFile` in the API-doc adapter, as the implementer recorded. Closing it means moving the reads onto `os.Root`, which also changes native CLI symlink behavior; out of this scope.
- F2's confinement table is tool-enumerated; a future workspace tool that reads files itself must be added to `confinedFileArgs`.
- F4 artifact parsing is host-agnostic (a receipt may cite another host's artifact format); config path/sha, host version, and transport still bind the receipt to the host. Evidence quality, not an adversarial boundary.
- F1/F3 add one live process inspection per capability-bound write on the grant root while the write lock is held; this is what serializes rotation with the write.
- A live Orca reconcile end to end was not run here either; F3 is covered by the real-SQLite repository test, the real-authority rotation test, the sqlstore tests, and the overlay RED proof.
