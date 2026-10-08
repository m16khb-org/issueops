---
name: OPERATIONS.md
description: Install, runtime, and operating procedures; read before running, deploying, or troubleshooting.
---

# Operations Map

`issueops` gives Codex, Claude Code, Omo native, and omp the same Go binary, MCP schema,
command policy, state store, shared skills, lifecycle hooks, and project-doc
workflow.

This file is the canonical index for the operations family. Read the focused
guide that matches the task.

## Guides in this family

| Task | Read |
|------|------|
| Install, bootstrap, refresh, daily commands, release smoke | [install-and-update.md](operations/guides/install-and-update.md) |
| CLI discovery, command policy, guard, state maintenance, contract conformance, quick smoke | [cli-and-state.md](operations/guides/cli-and-state.md) |
| Native skills, host hook rules, hook kill-switch | [skills-and-hosts.md](operations/guides/skills-and-hosts.md) |
| Health gate, diagnosis, one-time reconciliation | [troubleshooting.md](operations/guides/troubleshooting.md) |
| IssueOps provider publication, branch linkage, issue snapshots | [issueops-providers.md](operations/guides/issueops-providers.md) |
| IssueOps execution lifecycle, recovery, sync-base, owner sequence, ten-stage operation | [issueops-execution.md](operations/guides/issueops-execution.md) |
| Direct CLI, policy, guard, state, loop, MCP cleanup, worker, audit | [cli-and-mcp.md](operations/guides/cli-and-mcp.md) |
| Codex/Claude/Omo/omp native skills, MCP registration, lifecycle hooks | [hosts.md](operations/guides/hosts.md) |
| Project bootstrap, project-doc routing, MCP document updates | [project-docs.md](operations/guides/project-docs.md) |
| Web-fetch deterministic benchmark and opt-in live parity | [web-fetch-live-parity.md](operations/guides/web-fetch-live-parity.md) |
| Stability audit baseline and child-host smoke | [stability-baseline.md](operations/guides/stability-baseline.md), [child-host-smoke.md](operations/guides/child-host-smoke.md) |

## References read by code at fixed paths

These references stay directly under `operations/` because code or contract
tests read their exact paths. Do not move or rename them.

| Topic | Owner |
|-------|-------|
| First-run install, `io update`, command shims, MCP refresh | [operations/install.md](operations/install.md) |
| self-verify, self-augment, api-doc gate, general smoke | [operations/verification.md](operations/verification.md) |
| Release checklist, clean-machine smoke, build matrix, rollback | [operations/release-reproducibility.md](operations/release-reproducibility.md) |
| Release dogfood transcripts | [operations/release-dogfood-notes.md](operations/release-dogfood-notes.md) |
| Quality dashboard and score inputs | [operations/quality-dashboard.md](operations/quality-dashboard.md) |

Skill-quality scorecards and dogfood evaluations are research records under
[research/skill-quality/](research/skill-quality/).

## Core Surfaces

1. Native skills: `io-update`, `io-model`, `atomic-commit-push`, `issueops`, `self-augment`,
   `project-bootstrap`, `self-verify`, `stability-audit`, plus the named
   specialist skills in `skills/`. The IssueOps stage skills are
   `issueops-create-issue`, `issueops-prepare`, `issueops-plan`,
   `issueops-implement`, `issueops-slop-clean`, `issueops-docs`, `issueops-verify`,
   `issueops-create-pr`, `issueops-complete`, `issueops-cleanup`, and
   `issueops-abandon`; the shared ones are `issueops-review`, `gates-ledger`,
   and `issueops-remote-write`; `issueops-sync-issue` and `issueops-sync-pr`
   refresh an already published issue or PR/MR body. `issueops next` decides which stage
   a cycle is in and which command advances it.
2. MCP: on darwin/linux, `install`/`update`/`bootstrap` default to
   `--mcp-transport=http`. Codex, Claude Code, Omo, and omp then connect directly to
   one shared Streamable HTTP service at `http://127.0.0.1:47831/mcp`, sending the
   bearer from `<state>/mcp-http/bearer` (0600). Each host config holds only the
   issueops entry (`url` + `Authorization` header), written 0600. `issueops mcp
   --http` runs the service in the foreground, and `issueops mcp service
   start|stop|status --json` controls the LaunchAgent `io.issueops.mcp` or the
   systemd user unit `issueops-mcp.service`. The status DTO is
   `{ok,status,pid,build_id,url,error_code}`. `status` is
   running/stopped/stale/conflict. It is empty (with `ok:false`) when the
   supervisor, the service state, or the service binary could not be observed:
   `error_code` is then `supervisor_unsupported`, `supervisor_unavailable`,
   `state_unreadable`, or `build_mismatch` from an unreadable binary on start. Workspace
   tools over HTTP need a capability: the native session runs `issueops mcp
   authorize --workspace-root PATH ...` once, and the tool call passes the printed
   path as `authority_file` plus `workspace_root`/`cwd`. The token itself is
   never printed. `--mcp-transport=stdio` keeps the previous `issueops mcp`
   stdio entry, which serves inside the host session's own process. agy always
   uses stdio. Service limits: the unit passes only `ISSUEOPS_ROOT` and
   `ISSUEOPS_STATE_DIR`. HTTP `harness_inspect` therefore sees the supervisor's
   default `HOME`/`PATH` and no `CODEX_HOME`, and may report
   `host_version_unobservable`. The SDK also advertises `idempotentHint:false`
   on the two read-only tools (`harness_inspect`, `docs_index`), next to the
   catalog's `readOnlyHint:true`/`openWorldHint:false`.
3. CLI: 26 harness top-level commands (`install/update/bootstrap`,
   `inspect/preflight/system-status/doctor/docs`,
   `policy/guard/quality/verify-work/trace/contract/api-doc`, `project/hook`,
   `state/mcp/worker`, `loop/gates/channel`,
   `self-verify/self-augment/web-fetch`), plus 36 IssueOps lifecycle commands
   registered flat at the top level (`start`, `status`, `next`, `execution`,
   `remote`, `cleanup`, ...; `internal/contract/cli.LifecycleCommands`) and the
   built-in `help`/`version`. `issueops --help` is the canonical list.
4. Loop contracts: `issueops loop start/record-attempt/status/stop` records
   verify-until-done state and strict readiness gates without executing
   verification commands.
5. Task gate ledgers: `issueops gates init/check/status/report/abandon`
   discovers per-issue `.issueops/issues/<n>/gates.md` first, then generic
   `.issueops/gates/*.md` ledgers.
   Generic `gates init` still defaults to `.issueops/gates/<scope-slug>.md`;
   IssueOps owns `.issueops/issues/<provider-issue-number>/gates.md` and
   judges only its own or anonymous ledgers for strict PR readiness.
   CHECK commands run through the command policy engine (never a raw shell),
   and unmet gates add `gates_incomplete:<file>`. See
   [operations/guides/cli-and-mcp.md](operations/guides/cli-and-mcp.md).
6. Cross-session channels: `issueops channel send/recv` gives Codex,
   Claude Code, Omo, and omp sessions a durable shared mailbox over issueops state —
   the transport for front/server-style multi-session coordination.
   `recv --wait --since <id>` is the blocking consumer; MCP exposes
   `channel_send`/`channel_recv` with the same contract.

## Invariants

- Default install writes only user-level host configuration. Target repos get
  files only through explicit project bootstrap or project-local opt-in.
- Host adapters are thin wrappers around the same CLI/core behavior. They must
  not duplicate policy, schema, or state semantics.
- Default hooks provide only static project-doc context. They must not create
  issues/PRs, run tests, edit shared docs, read IssueOps state, perform
  maintenance, or perform long network/file reads.
- IssueOps implementation must pass durable design, compatibility,
  devil's-advocate, and execution v1 gates. Hooks do not decide compatibility,
  side effects, sub-agent usage, or lease ownership.
  `issueops execution prepare/status/claim/release/replace/reconcile/complete`
  and MCP `issueops_execution` are the single execution contract.
- Native install/update, readiness, and self-verification remain standalone.
  After native activation, the declarative `configs/upstream.json` catalog may
  provision missing Claude plugins and Git skills; it is dry-run visible,
  Claude-scoped, and non-fatal. Other companion tools keep their own setup paths.
- Worker functionality remains policy-gated and state-first until
  write/network/background execution has explicit audit, timeout, cancellation,
  and redaction coverage.

## Updating this family

1. Add focused detail to the guide that owns the responsibility.
2. Keep this index to navigation, the universal summary, and invariants only.
3. Every guide links back to `OPERATIONS.md`; cross-family rules link to their
   canonical owner, not a duplicate summary.
4. Run the documentation check before committing:

```bash
uv run --directory skills/project-docs-optimize python -m scripts.check \
  --root "$PWD" --mode check --json
```
