---
name: PROJECT_AUDIT.md
description: Whole-project audit findings, triage status, and hardening follow-ups.
---
# Agent-Harness Whole-Project Audit

> Generated: 2026-06-14. **Last reconciled 2026-10-07 (#548)** against HEAD
> `93649a26` plus the #548 worktree diff. Every ID below was re-checked against
> the current tree with `git grep`/`ls`; evidence cites current `file:line` or
> the commit that changed the code.
> Detail sections (per-subsystem findings with incident-time `file:line`), the
> 2026-06-16 triage narrative, the historical resolution plan, and the
> lock-deletion incident moved to
> [archive/issueops-audit.md, Appendix C](archive/issueops-audit.md#appendix-c-whole-project-audit-detail-moved-from-project_auditmd).
> Scope: the 31 subsystems audited on 2026-06-14 (daemon, MCP proxy, worker,
> command policy, state, lifecycle hooks, project bootstrap/docs, self-verify,
> self-augment, CLI, install, and the rest listed in Appendix C.0).

## How `quality inspect` reads this file

`CollectAuditItems` (`internal/adapter/outbound/quality/source.go:68`) counts
only the rows of the table under the heading that starts with **Open**. The
table needs unique `ID`, `Area`, `Title`, `Priority`, `Size` columns and a
separator row; `P0`/`P1`/`P2` rows are counted, `P3` rows are skipped, any
other priority or a malformed row is a warning that blocks the quality gate
(#542). The history tables below omit the `Priority` column on purpose, and
`TestCollectAuditItemsCurrentProjectDocument`
(`internal/adapter/outbound/quality/audit_test.go:101`) pins this document at
0 items and 0 warnings. A new P0–P2 finding goes into the Open table and must
update that test in the same change.

## Removed subsystems

These subsystems no longer exist, so every finding against them is
**closed by removal** (the matrix below lists which IDs each removal closed).

| Subsystem | Former path | Removed by |
|-----------|-------------|------------|
| Legacy shared daemon (socket server, `daemonlock`) | `cmd/issueops/daemoncli`, `internal/*/daemon` | `33915907` (2026-10-06) |
| Legacy hook surface: PreToolUse/Stop hooks, hook-failure log, compact capsule, next-action relay, `HostHookOutput` | `cmd/harness/hookcli/hook_pre_tool_use.go`, `hookfailure`, `internal/adapter/lifecycle/compact`, `internal/adapter/lifecycle/nextactionrelay` | `2e810e13` (2026-08-27) |
| Draft wiki staging queue | `internal/core/draftwiki` (removed) | `11a9c142` (2026-08-08) |
| Z.AI external LLM wrapper | `internal/core/externalllm` (removed) | `2b429005` (2026-07-07) |
| `internal/core` tree (contents relocated, not removed) | `internal/core/*` | `ccee5d5f`, `cba3b023` (2026-08-08) |
| Exported `StateUpdate` (0 production callers) | `internal/core/state` (removed) | `57091827` (2026-10-06) |

Only two hooks remain (`cmd/issueops/hookcli/hook.go:23`, session-start and
post-compact); they render the static project-doc catalog and touch no durable
state.

## Summary Matrix

> Of the 2026-06-16 baseline (24 P1/P2 items plus the follow-up completeness
> and convergence findings), the 2026-10-07 reconciliation finds **0 open**:
> 10 resolved in live code, 17 closed by removal, 1 accepted, and 4
> out-of-scope or theoretical. Two P2 findings from the detail sections that
> never had a matrix row (C.2.3 worker retry, C.10.3 shebang) are now triaged
> into the out-of-scope table. The six #548 maintenance fixes are recorded
> separately; none of them reopened or matched an existing ID.
> `branch_candidate_functions` (`internal/contract/quality/types.go:24`) stays
> accepted structural noise: it sums raw branch nodes including every `case`
> arm, so it flags any dispatcher; it is informational, not a work item.

### Open — the counted `audit_p1_p2_items`

_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._

### Accepted / documented — excluded from the count (won't-fix with rationale)

| ID | Subsystem | Problem | Disposition |
|----|-----------|---------|-------------|
| S2 | State | No atomic multi-key transactions | accepted — no state caller writes 2+ keys atomically; each key write is one SQLite transaction (`internal/adapter/outbound/sqlstore/sqlstore.go:684` `Apply`, which already accepts a batch of mutations); add a multi-key state API only when a real caller needs it |

### Resolved — excluded from the count (verified in live code)

| ID | Subsystem | Problem | Resolved by (current evidence) |
|----|-----------|---------|--------------------------------|
| W1 | Worker | Stuck "running" jobs on crash | `b89d311`; now `internal/application/worker/service.go:121` `DetectStuck` (PID + `PIDAlive`, re-checked under the job lock) |
| W2 | Worker | No concurrent job guard | `b89d311`; now per-job `WithLock` spans in `internal/application/worker/service.go:47,56,84,105` |
| W3 | Worker | Job store write not atomic | 2026-06-16 completeness audit; now one sqlstore row upsert (`internal/adapter/worker/store.go:49`) under the caller's span lock |
| S1 | State | No write locking | `40013f8`; now SQLite span lock (`internal/application/state/service.go:200` `WithKeyLock`, `internal/adapter/outbound/state/state_lock.go:7`) |
| TC2 | State | `WriteStateRecord` accepts a record key diverging from the write key | 2026-06-16 convergence re-audit; `TestWriteStateRecordRejectsKeyMismatch` (`internal/adapter/outbound/state/state_test.go:59`) |
| SA1 | Self-augment state | Snapshot write bypasses lock + atomic write | 2026-06-16 hardening; `TestWriteSelfAugmentSnapshotRecordIsLockedAndAtomic` (`cmd/issueops/selfworkflow/stateio/self_verify_state_snapshot_test.go:16`) |
| P1 | Project state | Init race condition | `0800fca` + `a56d5e8`; now `internal/application/lifecycle/service.go:64` (exclusive create, then adopt the winner's profile on `fs.ErrExist`) and `createJSONAtomic` (`internal/adapter/lifecycle/lifecycle_project_state_store.go:101`) |
| CP1 | Command policy | Hardcoded catalog | resolved 2026-07-02; `.issueops/policy.json` loaded per root at `internal/adapter/policy/policy_catalog.go:26` |
| M1 | MCP proxy | Dual transport code paths | `034bda93`; one go-sdk path, `ServeMCPStreamContextWithDependencies` (`cmd/issueops/mcpcli/mcp_sdk_server.go:304`), serves every stream |
| M2 | MCP proxy | Tool catalog drift | `b8f2f11`; `DispatchMap` derived from the catalog (`internal/adapter/inbound/catalog/mcp/catalog.go:45`), pinned by `TestDispatchMapCoversAllCatalogTools` |

### Closed by removal — excluded from the count

| ID | Subsystem | Problem | Closed by |
|----|-----------|---------|-----------|
| D1 | Daemon | No connection limit | removal of the daemon, `33915907` (had been fixed by `5536cc8`) |
| D2 | Daemon | NFS lock safety (O_EXCL) | removal of the daemon and `daemonlock`, `33915907` (had been accepted) |
| D3 | Daemon | No graceful shutdown | removal of the daemon, `33915907` (had been fixed by `5536cc8`) |
| H1 | Hook failure | Unbounded log growth | removal of the hook-failure log, `2e810e13` |
| H2 | Hook failure | Concurrent append > PIPE_BUF | removal of the hook-failure log, `2e810e13` |
| W4 | Hook failure | Prune `Close()` error ignored before rename | removal of the hook-failure log, `2e810e13` |
| W5 | Hook failure | Prune unlocked while append locked | removal of the hook-failure log, `2e810e13` |
| C1 | Compact | Double PreCompact overwrite | removal of the compact capsule, `2e810e13` |
| C2 | Compact | Read-delete race | removal of the compact capsule, `2e810e13` |
| P2 | Project state | Compact-capsule read-modify-write lost update | removal of the compact capsule, `2e810e13`; the remaining profile write replaces the whole profile via temp+rename (`internal/application/lifecycle/service.go:82`) |
| Q1 | Draft wiki | No stale lock detection | removal of the draft wiki, `11a9c142` |
| Q2 | Draft wiki | Capsule overwrite | removal of the draft wiki, `11a9c142` |
| N1 | Next-action | Read-write race | removal of the next-action relay, `2e810e13` (had been theoretical) |
| L1 | External LLM | Single provider | removal of the external LLM wrapper, `2b429005` (had been accepted) |
| L2 | External LLM | No retry on malformed output | removal of the external LLM wrapper, `2b429005`; the surviving pure decoder (`internal/domain/judgement/structured.go:39`) has no invoke handle to retry |
| HK1 | Hooks | Host output format divergence | removal of the legacy hook surface and `HostHookOutput`, `2e810e13` (had been fixed by `ff0e668`); the two remaining hooks format through `internal/adapter/hostprotocol/hook.go:5` |
| TC1 | State | Exported `StateUpdate` untested, 0 callers | removal of `StateUpdate` as dead code, `57091827` |

### Closed in #548 — maintenance-sweep findings (excluded from the count)

| ID | Area | Problem | Closed by |
|----|------|---------|-----------|
| MS1 | Toolchain | Go 1.26.3 stdlib vulnerabilities | closed in #548 — `go.mod` `go 1.26.6`, `golang.org/x/sync` and `golang.org/x/term` bumped |
| MS2 | MCP tests | Concurrency test panicked instead of failing | closed in #548 — `cmd/issueops/issueopsapp/mcp_concurrency_test.go` |
| MS3 | Readability tests | Assertion depended on wall-clock timing | closed in #548 — `internal/domain/artifactreadability/readability_test.go` |
| MS4 | Quality inspect | Coverage collection failed because a coverage-built policy helper warned without `GOCOVERDIR`, and the failing package was not named | closed in #548 — `internal/adapter/policy/policy_run_bounds_test.go` sets `GOCOVERDIR`; `internal/application/quality/inspect.go` and `internal/domain/quality/policy.go` report the failing package |
| MS5 | CLI | Duplicated `printJSON` / `containsString` helpers | closed in #548 — one `printJSON` in `cmd/issueops/jsonout/jsonout.go` (e.g. `cmd/issueops/basiccli/json.go`), the production `containsString`/`contains` copies with the plain loop replaced by `slices.Contains` |
| MS6 | Tests | Tests leaked the developer's git config, state dir, and `HOME` | closed in #548 — `testsupport.IsolateGitConfig` (`internal/testsupport/git_config.go`) plus test helpers that set `HOME` and `ISSUEOPS_STATE_DIR` |

### Out-of-scope / theoretical — excluded from the count

| ID | Subsystem | Problem | Disposition |
|----|-----------|---------|-------------|
| V1 | Self-verify | Temp dir leak on kill | documentation-only; temp dirs are created by `cmd/issueops/issueopsapp/self_verify_facade.go:90` with the `issueops-self-verify-*` prefix and removed on normal exit by `internal/application/selfverify/loop.go`; SIGKILL leftovers need explicit hygiene |
| CP2 | Command policy | No chained command analysis | out of scope — the policy evaluates an argv vector, not a shell string (`internal/domain/policy/catalog.go:97`); shell interpreters are denied unless `shell_allowed` + `shell_reason` (`internal/domain/policy/command_decision.go:75`) |
| CP3 | Command policy | Shell detection reads argv[0] only (shebang scripts not inspected; detail C.10.3) | out of scope — without `write_allowed` an unlisted script is denied by the read-only allowlist (`internal/domain/policy/command_decision.go:91`); with `write_allowed`, `python`/`node`/`perl` already run (`internal/domain/policy/catalog.go:45`), so the shell check is not a sandbox boundary there |
| W6 | Worker | No retry or scheduling (detail C.2.3) | out of scope — feature gap, not a defect; the worker stays MVP and no caller needs retries (`internal/application/worker/service.go` has no retry path) |
