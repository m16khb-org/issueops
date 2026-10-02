# Independent final review: I1-I10

**Verdict: FAIL.** Six items pass, two are conditional, and two fail. The existing Go suite is green, but it misses three HTTP authority/workspace defects and an inspection false-positive. Server PID ancestry is **not** a requirement for HTTP caller identity; none of the findings asks to restore it.

Reviewed against `contract.md`, `brief.md`, and the approved ten-item objective. Base HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`. Review date: 2026-10-02 UTC. Source references below are repository-relative. `$R` below means `/tmp/issueops-final-review-01a0fd59`.

## Severity-ordered defects

### F1 — High: valid capabilities cannot execute resume or reseed

**Locations:** `cmd/issueops/issueopsapp/authority_wiring.go:35-41`; `internal/adapter/outbound/issueopslease/reseed_fence.go:31-37`; `cmd/issueops/issueopsapp/issueops_resume_wiring.go:41`; `cmd/issueops/issueopsapp/issueops_reseed_wiring.go:31`; `internal/application/authority/service.go:115-129,188-197`.

The request's record guard is installed for the authority database. Resume and reseed first open a different database under `issueops-reseed-fence/<id-hash>`. That database's `WithSpan` consumes the guard and passes its own reader to `BindSpan`. It contains no authority grant, so every otherwise valid capability is rejected as revoked before the lifecycle callback runs.

**Reproduction on the final `/tmp/review-bin`:** issue a grant with `mcp authorize`, seed an isolated generation-1 execution using the existing record writer, and call `issueops_execution`:

```json
{"action":"resume","id":"io-02b0bb3ebde6","cwd":"$R/worktree","expected_generation":1,"confirm":true,"authority_file":"<issued grant>"}
```

HTTP returns `isError:true`, with `authority_invalid: authority credential was revoked or never issued`. `action:"replace", replace_action:"reseed"` returns the same error. The same grant succeeds for status and preflight, including after a server restart.

Native stdio requests with the same caller and lifecycle inputs reach the actual domain decisions: resume reports `execution resume requires an existing Orca binding`, and reseed reports `stale replacement inventory fingerprint`. The fixture deliberately lacks those prerequisites; this comparison proves HTTP fails at a different, incorrect authority boundary. Source inspection shows the misplaced guard executes before those prerequisites for valid Orca fixtures too.

This violates the explicit HTTP lifecycle parity requirement in contract §2. Merely testing that the dispatcher exposes all action names does not prove parity.

**Required correction:** distinguish the lifecycle fence database from the grant database. Recheck authority with the grant-root reader at the actual mutation boundary while preserving the lifecycle fence and its generation/CAS checks. Do not remove the guard or fall back to server ancestry.

### F2 — High: API-doc HTTP calls read outside the authorized workspace

**Locations:** `cmd/issueops/mcpcli/mcp_request_scope.go:149-185`; `cmd/issueops/mcpcli/mcp_tool_project.go:76-86`; `internal/adapter/outbound/apidoc/reviewfiles/files.go:13-18,27-30`.

`requestRelativePath` makes a relative path absolute but does not enforce containment. The API-doc adapter then calls `os.ReadFile` on `prompt_file` and `diff_file` without a workspace policy check or a confined filesystem.

**Final-binary reproduction:** repo A has `api.diff`; its sibling `$R/outside-prompt.md` contains `REVIEW_OUTSIDE_SCOPE_MARKER_7581`. With a grant scoped to repo A, send:

```json
{"name":"api_doc_review","arguments":{"repo":"$R/repoA","authority_file":"<repo A grant>","diff_file":"api.diff","prompt_file":"../outside-prompt.md"}}
```

Observed HTTP 200, `isError:false`, `verdict:"pending"`, `reason:"host_agent_result_required"`; the returned prompt contains the outside marker. The server runs from a third directory, `$R/server-cwd`, so this is not a cwd ambiguity.

This violates contract §2's workspace/file-access boundary. Same-user trust does not remove the explicitly promised repository scope. The unconfined readers predate HTTP, but exposing them behind a workspace capability without enforcing that scope is new.

**Required correction:** confine the HTTP API-doc file readers, including explicit files and symlink resolution, to the authorized scope or an explicit existing policy allowance. Keep CLI behavior separate where necessary.

### F3 — High: reconcile commits can bypass grant revalidation after canonicalization

**Locations:** `internal/application/issueopslease/reconcile.go:38-44,66,80`; `internal/adapter/outbound/issueopslease/reconcile_repository.go:51,155-156,216-219,301-307`; `internal/adapter/outbound/issueopslease/resume_repository.go:257-260`; `internal/adapter/outbound/sqlstore/sqlstore.go:824-873`.

Canonicalization opens a guarded span, then returns. External inventory work follows outside that span. The later receipt/clear paths use raw `CompareAndApply` without another guarded span. Their expected records include the lifecycle and intent, not the grant. Rotating or removing a grant does not change either CAS input. `CompareAndApply` preserves lifecycle atomicity, but it does not invoke `WithRecordGuard`.

**Deterministic reproduction:** `$R/reconcile_guard_test.go`, injected only through `$R/reconcile-overlay.json`, uses the existing `seededResumeIntent` fixture and real SQLite. Its request guard reads an active test grant row. Canonicalization checks it once; a separate span removes that row; `ApplyReceipt` then succeeds and advances the stored intent without checking the guard again:

```text
after revocation: checks=1 err=<nil> next_stage=run_create
reconcile persisted after revocation without running its request guard
```

Command:

```sh
go test -overlay "$R/reconcile-overlay.json" \
  ./internal/adapter/outbound/issueopslease \
  -run '^TestReviewReconcileWriteRechecksRevokedGuard$' -count=1 -v
```

Exit 1 is the expected defect reproduction, not a failure in the unmodified test suite. Full output: `$R/reconcile-probe.log`. The probe uses the production guard seam and real persistence; it is not a claim of a completed live Orca operation.

**Required correction:** recheck the bound grant under the authority-root lock for each reconcile mutation, retaining raw lifecycle/intent CAS. Test a genuine capability rotation during inventory as an end-to-end regression.

### F4 — Medium: failed host evidence is promoted to verified connectivity

**Location:** `internal/adapter/inspect/receipts.go:184-205`.

The strengthened artifact check still only searches for the substring `docs_index`, or a revision string. It does not distinguish a successful response from a request, failure, or prose mentioning the tool.

**Final-binary reproduction:** a matching Claude receipt points to this artifact:

```json
{"type":"tool_result","name":"docs_index","isError":true,"error":"review fixture: connection failed","protocolVersion":"2026-07-28"}
```

The receipt has the currently observed config path/hash and actual Claude version `2.1.287`. Running:

```sh
/tmp/review-bin inspect --json --host-receipts "$R/failed-host-receipt.json"
```

returns `discovered=verified`, `connected=verified`, and `protocol=verified`. No successful catalog, successful tool response, or negotiated revision exists in that artifact. Fixture and receipt are retained under `$R`.

This violates the original I3 requirement that actual host catalog and successful `docs_index` responses establish discovery/connection. The later contract amendment requiring textual presence is weaker than that objective and does not establish it.

**Required correction:** validate the relevant artifact event/result semantics, including failure status and negotiated rather than merely mentioned revision. Unrecognized artifact formats must remain unknown. This is an evidence-quality defect, not a claim that receipts provide an adversarial security boundary.

### F5 — Low: HTTP initialize still describes a host-owned stdio process

**Location:** `cmd/issueops/mcpcli/mcp_sdk_server.go:57`.

The live HTTP initialize response says:

```text
This MCP endpoint runs the issueops harness in-process for the calling host session.
```

That is false for the shared service and omits its capability requirement. The operating documents describe the new topology, but the instructions delivered directly to every MCP client retain the obsolete one.

**Required correction:** make the instructions transport-aware or transport-neutral and describe native grant issuance for HTTP workspace calls.

## Per-item verdicts

Command references V1-V8 and live checks Q1-Q7 are defined below.

| Item | Verdict | Evidence and qualification | Commands/checks |
|---|---|---|---|
| **1. JSONL completeness** | **PASS** | `internal/adapter/trace/decode.go:38-61` records malformed lines and Scanner errors; `internal/domain/trace/analysis.go:9-29` preserves warnings; `internal/application/trace/service.go:79-83` publishes completeness separately from execution success. Real CLI: normal sentinels produce two findings/complete=true; malformed middle preserves two findings/complete=false; a 70,000-byte middle line preserves one finding/complete=false with `jsonl_scan_error`. Private malformed text is absent from output. | V1-V2, Q5 |
| **2. Probe durations and reused statistics** | **conditional** | Command durations are preserved at `internal/adapter/verification/probe/contractauditworker/validation_contract_check.go:47` and `validation_worker_lifecycle.go:46-47`. Reuse is marked at `internal/application/selfverify/steps.go:84,147`, excluded from timing samples at `internal/domain/selfaugment/summary.go:47-60`, and counted separately. `history_compare.go:50-59` rejects cross-contract duration comparisons. Contract v7 is at `internal/domain/selfverify/contract.go:13`; envelope v1 remains at `internal/domain/selfaugment/summary_snapshot.go:13`. Relevant fixture/regression tests pass. **Condition:** the requested successful real self-verify summary is not established: at the review cutoff, `implementation-z-integration.md:104-110` contains failed/interrupted runs and `SV3_RESULT`/`SV3_DETAIL`, not a completed third-run result. Do not substitute the injected-probe fixture for that operational acceptance. | V1-V2; `TestSuccessfulProbesPreserveCommandDurations` and `TestSelfVerifyReuseFixtureEntryPoint` ran within V2 |
| **3. Host diagnostic evidence levels** | **FAIL** | The six observations, no-receipt `not_checked`, config/version checks, and host-specific paths exist (`internal/adapter/inspect/hosts.go`, `receipts.go:35-72,208-231`). Real CLI confirms the no-receipt states. F4 nevertheless turns explicitly failed evidence into verified discovery, connection, and protocol. | V1-V2, Q4 |
| **4. Structured MCP results** | **PASS** | `internal/contract/mcp/output_schemas.go:8-30,90` supplies schemas/annotations; `cmd/issueops/mcpcli/mcp_sdk_server.go:118-129,136-174` validates output and returns payload-free `-32603` on invalid output. Both real transports list the same 51 tools. Only `harness_inspect` and `docs_index` advertise output schemas/read-only annotations; their parsed text equals `structuredContent`. SDK-added `idempotentHint:false` is observed and documented, not silently ignored. Null, direct Markdown, and invalid-output regressions pass in mcpcli. | V1-V3, Q1-Q2 |
| **5. SDK and protocol compatibility** | **conditional** | `go.mod:34` pins SDK v1.8.0. `cmd/issueops/mcpcli/mcp_revision_compatibility_test.go:95-264` and HTTP tests cover revision negotiation, metadata, cancellation, and subscriptions. Real 2025-11-25 and sessionless 2026-07-28 requests work over stdio and HTTP. **Condition:** this is wire/SDK evidence, not independent final-build execution through all three native hosts. Prior host reports record discovery/docs calls, while `implementation-z-integration.md:117` explicitly leaves each host's native-authorize plus lifecycle scenario unexecuted. F1 also disproves full lifecycle parity regardless of client host. | V1-V3, Q1-Q2; read all host reports |
| **6. Request-local Git observation reuse** | **PASS** | `internal/adapter/preflight/preflight.go:55` obtains all four history projections from one log call; `helpers.go:22-65` parses NUL triples. `cmd/issueops/issueopsapp/issueops_next_wiring.go:39,142-183` creates the branch memo per Next call and leaves HEAD/toplevel reads fresh. Recording-runner and real-Git tests cover counts, separate roots/refs, subsequent requests, and observation boundaries. No atomic Git snapshot is claimed. | V1-V3; relevant preflight/readiness/Next tests within V2-V3 |
| **7. Bounded metadata discovery** | **PASS** | `internal/adapter/projectdoc/catalog.go:15-23,114-132,138-182,196-226` scans batches to EOF, retains 64 priority-selected names, and reads at most 8 KiB per selected file. Root/symlink, oversize, truncation, order, 200/1200-file selection, and omission regressions pass. Real hook output injects 13 project docs. The implementation uses a bounded sorted slice rather than the specified heap; with fixed cap 64 it retains the required bounds and deterministic selection. | V1-V2, Q6 |
| **8. SQL phase latency** | **PASS** | `internal/adapter/outbound/sqlstore/sqlstore.go:360-478` and `span_observer.go:25-42,108-137` distinguish wait, hold, callback, data commits, total, and unknown coverage; commit cancellation is checked before `Commit`. Tests cover contention/cancellation, panic, reentry, CAS, attribution, and visibility. Live error spans expose the phase fields; independent benchmarks measured nil/no-op/JSON observer costs. Coverage remains same-handle/process observation, not a cross-process accounting guarantee. | V1-V3, V7, Q7 |
| **9. Read-only usage ingestion and trace correlation** | **PASS** | `internal/adapter/trace/input.go:35-67`, `host_usage.go:58-95,233-361`, and `internal/domain/trace/host_usage.go:40-147` implement input limits, host authorities, cumulative resets, unknown/zero, nullable metrics, digest IDs, and deduplication without adding totals. `internal/domain/trace/trace_context.go:9-27` validates v00; HTTP binds request headers at `cmd/issueops/mcpcli/mcp_http.go:136-153`. Real CLI fixtures match expected samples; two concurrent live requests emit distinct matching trace IDs. Actual Codex exec export remains a documented residual gap, not a fabricated host measurement. | V1-V3, Q5, Q7 |
| **10. Shared local HTTP MCP** | **FAIL** | Direct stateless HTTP, bearer/Host/Origin controls, native issuance, request-local scope, nil capability ancestry, and ordinary holder/generation rejection are implemented (`mcp_http.go:52-102`; `mcp_request_dispatch.go:54-88`; `application/authority/service.go:53-158`). Two clients, grant rotation, release readback, and server restart succeed. However F1 breaks required lifecycle parity, F2 breaks workspace containment, and F3 misses revocation serialization. The green dispatcher tests do not cover these real compositions. | V1-V4, V6, V8, Q1-Q4, Q7 |

## Commands and directly observed results

All experiments used isolated HOME, CODEX_HOME, state, and XDG directories below `$R`. Module/build caches were copied into `$R/go` before use; no command was allowed to write the real HOME. No supervisor was started or stopped by this review. Foreground servers used an ephemeral loopback port, then reused that port for the restart. Temporary Git repositories/worktrees and overlay files were also below `$R`; no review branch or commit was created.

| Ref | Actual command | Result |
|---|---|---|
| V1 | `go vet ./...` | Exit 0, including a final rerun after concurrent integration changes. |
| V2 | `go test ./... -count=1` | Final run exit 0: **323 passing packages**, no FAIL. `GOFLAGS=-p=2`, `GOMAXPROCS=4`; `ISSUEOPS_ROOT` unset. Complete output: `$R/full-test.log`; monitor `mon_0CEAMX6M798BE349`. |
| V3 | `go test -race ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp ./internal/adapter/outbound/sqlstore ./internal/application/authority ./internal/adapter/mcpservice -count=1` | Exit 0, all five packages pass; no race report. Monitor `mon_NCN0G3F0PNDCAHSD`. |
| V4 | `go build -o /tmp/review-bin ./cmd/issueops` | Exit 0; repeated after the concurrent integration delta. Final binary was used to reconfirm F1, F2, F4 and persisted grant/lease state. Repository `bin/issueops` was not replaced. |
| V5 | `go build -overlay "$R/overlay.json" -o "$R/helper" ./cmd/issueops` | Exit 0. The temporary helper observes the review process receipt and seeds an isolated execution with the production record writer. `$R/helper.go` is retained; no source file was added to the repository. |
| V6 | `go test -overlay "$R/reconcile-overlay.json" ./internal/adapter/outbound/issueopslease -run '^TestReviewReconcileWriteRechecksRevokedGuard$' -count=1 -v` | Exit 1, deterministic F3 reproduction. `$R/reconcile-probe.log`. |
| V7 | `go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -run '^$' -bench 'Benchmark(SpanObserverOverhead\|StoreSpanObserverOverhead)$' -benchmem -benchtime=100x -count=1` | Exit 0. `$R/observer-benchmark.log`; measurements below. |
| V8 | `go test -race ./internal/adapter/omo ./cmd/issueops/issueopsapp -run 'TestOmoHTTP\|TestSelfWorkflowHistory' -count=1` | Exit 0 after reading the concurrently changed Omo cache-header code and history test helper. The final-delta chain ran V1, then V8, then V4; all exited 0 (`mon_GSDJNHYJ6KGGZDS2`). |

Earlier runs are not hidden: the first full test run inherited this review's `ISSUEOPS_ROOT`, causing `TestBinaryDriftUsesRealDoctorObservation/stale` to inspect the wrong fixture root (`Healthy:true`). Removing that override fixed the observation. A subsequent run reported an issueopsapp build failure while the shared tree was being edited; its exact compiler diagnostic was not retained by the filtered output, so no root cause is claimed. A direct compile check passed, and the final complete V2 run passed. No test was edited by this review.

### Real-surface matrix

HTTP requests were actual JSON-RPC POSTs using Bun `fetch`, not mocked dispatch calls. Stdio checks spawned `/tmp/review-bin mcp` and parsed its newline-delimited responses. Tool comparisons used deep object equality, not JSON key order.

| Ref | Commands/actions | Observation |
|---|---|---|
| Q1 | `/tmp/review-bin mcp --http --addr 127.0.0.1:0`; HTTP initialize/tools/list/tools/call | Server PID 14853, URL `http://127.0.0.1:51831/mcp`. Two concurrent clients return their own repo roots. 51 tools; structured/text parity for both representative tools. 2026 requests return `resultType:"complete"`. |
| Q2 | `/tmp/review-bin mcp`, legacy initialize/initialized/list/calls; separate 2026 metadata list/call without initialize | Exit 0. Same catalog and annotations as HTTP; structured/text deep equality. Native preflight and capability status both work. Native versus capability resume/reseed comparison gives F1. |
| Q3 | `/tmp/review-bin mcp authorize --workspace-root "$R/repoA" --host codex --session-id review-a --session-pid <observed PID> --session-started-at <observed start> --session-executable <observed executable> --json`; corresponding B grants; reissue A | Actual CLI issuance from an observed ancestor, not direct grant-row fabrication. Separate repo B and same-repo caller B grants were created. Missing capability, foreign repo, and mixed actor inputs are rejected. Old A fails after rotation; new A succeeds. |
| Q3 | `issueops_execution` release/status against `io-02b0bb3ebde6` | Other holder rejected; generation 2 rejected; source cwd rejected; generation 1/current holder/canonical worktree releases successfully. HTTP status reads back `released`, generation 1, no holder. A still-valid grant does not become lease ownership after release. |
| Q3 | SIGTERM; rebuild; `/tmp/review-bin mcp --http --addr 127.0.0.1:51831` | First server exits 0; new server PID 45696 accepts the previously issued grant and reads the same released lease. F1 and F2 reproduce on this final binary. Final server also exits 0 after SIGTERM; neither test PID remains. |
| Q4 | `/tmp/review-bin inspect --json`, with and without `--host-receipts "$R/failed-host-receipt.json"` | Without receipts, all discovered/connected/protocol observations are `not_checked`. With matching but explicitly failed artifact evidence, all three Claude observations incorrectly become `verified` (F4). Final-binary repeat gives the same result. |
| Q5 | `/tmp/review-bin trace analyze --input - --json` with normal/malformed/oversize sentinel JSONL | Exit 0; findings/complete: `2/true`, `2/false`, `1/false`. Warning codes and no raw malformed-marker leakage confirmed. |
| Q5 | `/tmp/review-bin trace analyze --input internal/adapter/trace/testdata/host-usage/<fixture> --input-format <format> --json` | Claude cumulative-reset fixture: two epochs, latest 250 then 20 input tokens, partial coverage/reset warning. Codex duplicate-number fixture without stable turn IDs: two samples, unknown coverage. Omo final-authority fixture: one sample, 120/30 tokens, cache 0/7, cost 0.3, complete coverage. All exit 0. |
| Q6 | `/tmp/review-bin hook session-start --repo "$REPO_ROOT" --json` with stdin EOF | Exit 0; `should_inject:true`, 13 project docs. Bounded selection/byte counts are proved by the production tests in V2, not inferred from this small catalog. |
| Q7 | Two concurrent 2026 execution calls with distinct valid `traceparent` headers | Error spans contain only their matching `aaaaaaaa...` or `bbbbbbbb...` trace/parent IDs and expose acquired/callback/commit/count/coverage fields. Server cwd remained empty. |

The caller labeled `codex` in Q3 is the review process's verified native receipt. This is not represented as a real Codex model session or as the missing three-host lifecycle acceptance.

### I8 independent benchmark sample

Apple M1 Pro, darwin/arm64, 100 iterations per case. These are measurements, not a performance-improvement claim or a stable latency budget.

| Case | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Empty span, nil observer | 15,544 | 2,098 | 22 |
| Empty span, no-op observer | 14,810 | 1,770 | 23 |
| One commit, nil observer | 280,635 | 3,291 | 54 |
| One commit, no-op observer | 265,406 | 3,301 | 56 |
| Store update, nil observer | 256,002 | 12,644 | 129 |
| Store update, no-op observer | 247,007 | 12,092 | 135 |
| Store update, actionable JSON observer | 259,159 | 12,856 | 139 |

## Scope and ownership audit

I read every `implementation-*.md` and `*-verification.md` present under this plan directory, plus `parent-verification.md`, `host-qa-preflight.md`, `z-docs-scope.md`, `brief.md`, and the full contract. This includes the later-arriving `implementation-z-integration.md`; its changing final-battery section was reread. I inspected whole-worktree status/diff and included untracked implementation files rather than treating `git diff` alone as the change set.

The two explicitly excluded Windows documents were ignored. Generated OpenWiki pages were not changed. No new database technology, scheduler, per-session HTTP proxy, or host-specific core was found.

The reports disclose several deviations from contract §5's narrow ownership table:

- G modified foundation authority DTO/port files assigned to Z; its report describes them as handed over.
- H's corrective context propagation reached G/E-owned persistence, loop/worker/state ports, application callers, self-workflow composition, and many fixtures. These changes serve I10 rather than an unrelated feature, but exceed H's original transport/dispatch ownership row. Parent verification records the corrective assignment.
- J edited H's `mcp_http_command.go`, service command/tests, and composition files to connect instance ownership and supervisors.
- K edited MCP dependencies/handlers and extra composition/test callers beyond its inspect/basiccli/app.go row.
- L edited F's `go.mod`/`go.sum`, regenerated Z's MCP golden, and removed the old H authority-schema helper. The resulting dependency/catalog changes are required by I4, but ownership transfers are report claims rather than information derivable from the final diff.
- Research documents under `.issueops/research/agent-updates-2026-10-02/` were also edited to anonymize local paths, outside `z-docs-scope.md`'s operating-document list. The diff is ancillary evidence hygiene, consistent with contract §6's portable-path requirement, not a new product feature. The new QA caution also records editing-tool workflow advice rather than one of the ten runtime improvements.
- I7 uses a fixed-size sorted slice instead of the contract's heap. It still bounds memory/body I/O and selection deterministically; this is a disclosed implementation deviation, not a blocking defect.

During this review another writer changed Omo's HTTP entry to include `X-Issueops-Mcp-Catalog-Sha256`, its tests/docs/inventory, and the history test helper. I read that delta and ran V8/V1/V4. The earlier report's Omo cache-token defect is **not open in the final tree**: `internal/adapter/omo/mcp.go:53-66` now includes the catalog digest, matching the installed Omo config-hash cache behavior. None of those edits was made by this reviewer.

## Documentation consistency and residual risks

- The updated architecture/operations documents correctly describe shared HTTP, native issuance, same-OS-user trust, stdio compatibility, supervisor limitations, and the SDK's extra annotation. Their broad lifecycle/workspace guarantees are not fully delivered because of F1-F3. The client-facing initialize instructions are independently stale (F5).
- `implementation-z-integration.md:121` still describes the Omo HTTP cache omission even though the final code fixes it. Earlier implementation/verification reports also retain historical failures later fixed; they are not current verdicts.
- The no-server-cwd statement is broader than the literal code: `cmd/issueops/issueopsapp/mcp_facade.go:139` constructs `newLoopService` for record lookup, whose identity construction calls `os.Getwd` in `loop_wiring.go:21`. That value is unused by the status read. I found no resultant server-cwd file write in this path, but it does not literally satisfy contract §2's ban on that fallback.
- Unit environments intentionally carry only `ISSUEOPS_ROOT` and `ISSUEOPS_STATE_DIR`. Supervisor PATH/HOME and absent CODEX_HOME can change command availability and inspection results. This is documented and follows the chosen contract; it is not treated as proof of three-host operational parity.
- Request cancellation reaches SDK handlers and the covered state transactions. `policy.Run` still derives its command timeout from `context.Background` (`internal/adapter/policy/policy_run.go:54`); cancellation of a 2026 HTTP request does not prove immediate cancellation of every worker/gate command. The H report explicitly identifies this boundary.
- Linux supervisor behavior was not executed on this macOS machine. This review exercised real foreground HTTP, not fresh launchd installation/update. Existing supervisor tests passed; historical live supervisor reports were read, not silently promoted to new live evidence.
- Actual per-host authorize/prepare/claim/mutation acceptance and a completed successful self-verify summary remain unproved in the supplied final reports. The current failures are not fixed by adding those reports alone.
- I9's Codex exec path is independently fixture-tested, not a newly acquired native export. The supplied host QA records a `service_tier` parsing blocker. No real credentials or host settings were copied or altered to bypass it.
- The project-docs API review gate was reported as `no_api_doc_candidate_files` on the unstaged tree. That skip is not evidence that an agent reviewed the new Go DTO/schema contracts. I reviewed those contracts here, but did not fabricate a recorded `api_doc_review` result or mutate gate state.

**Disposition:** do not approve the complete ten-improvement delivery until F1-F4 are corrected and verified. F5 is a small documentation correction. Keep the working caller-capability design, claim token checks, holder/generation fencing, and stdio support; the defects are in integration boundaries, not a reason to reintroduce server ancestry.
