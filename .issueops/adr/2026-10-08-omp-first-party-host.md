# omp (oh-my-pi) as the fourth first-party host

> Family index: [`../../ADR.md`](../ADR.md)

- Date: 2026-10-08
- Status: accepted

## Context

omp (oh-my-pi, `omp/18.8.4`) is a pi-family coding agent that the user runs
next to Codex, Claude Code, and Omo native. Without an adapter it reached only
whatever leaked in through foreign discovery roots (`~/.agents/skills`), had no
issueops MCP entry, no project-doc lifecycle context, and no native actor
identity, so IssueOps lease mutations were impossible from an omp session.

Two omp facts shape the adapter:

- omp exports no session id to its shells. Its bundle sets `PI_SESSION_FILE`
  only for eval kernels; bash children see `OMPCODE=1` and nothing that names
  the session. omp does give extensions `ctx.sessionManager.getSessionId()`
  and `ctx.agent.kind`, and bash children inherit `process.env` at spawn
  (verified live: the exported value equals the session header id of the run).
- omp ships as a `#!/usr/bin/env bun` script, so `ps -o comm=` reports the
  session process as `bun`.

Orca 1.4.222 knows omp as an agent (`detectCmd: omp`, `titleIdentityGroup:
pi-compatible`) and recognizes omp terminals for injected dispatch, unlike Omo,
whose delivery needs a terminal-send prompt receipt.

## Decision

omp is the fourth first-party host, ordered after Omo everywhere
(`codex, claude, omo, omp`; installers end with `agy`).

- Install writes user scope only: skill symlinks in `~/.omp/agent/skills`,
  the `issueops` entry merged into `~/.omp/agent/mcp.json` (HTTP with bearer
  and catalog digest header, or stdio with the catalog digest env, because omp
  caches MCP tool lists by config hash like Omo), and
  `~/.omp/agent/extensions/issueops.js`. Explicit `--project-local` writes
  `.omp/mcp.json` only. Activation commits after MCP and extension readback.
- The lifecycle extension maps `session_start` and `session_switch` to
  `hook session-start` and `session_compact` to `hook post-compact`. None is
  accepted-only: omp emits `session_compact` only after a compaction and its
  payload has no `accepted` field.
- On `session_start` and `session_switch` the extension sets
  `process.env.ISSUEOPS_OMP_SESSION_ID` from the main session
  (`ctx.agent.kind === "main"`). Subagents share the omp process and never
  write it, so subagent shells carry the parent session id, matching Claude
  Code subagents.
- `execution whoami` and actor resolution read `ISSUEOPS_OMP_SESSION_ID` as the
  omp session identity. The reusable process receipt is the nearest ancestor
  whose executable is `omp` or `bun`, accepted only for `host=omp`. A session
  id alone never authorizes a lease; Omo and omp ids together are ambiguous and
  fail closed.
- Role models use built-in defaults only, like Omo (`anthropic/claude-opus-5-5`
  high for implement and reviews, Sonnet 5.5 for research, Haiku 5.5 for
  reader-check), with the effort ladder `off..max`. omp gets a print argv
  (`omp -p --model M --thinking E`) and no role-agent injection.
- Launch profiles: interactive and cmux `omp --model M --thinking E
  --auto-approve -- PROMPT`; Orca owner `omp --model 'M' --thinking 'E'
  --auto-approve`, probed through `omp --help` for `--model`, `--thinking`, and
  `--auto-approve`; worktree relaunch `cd <worktree> && omp`.
- Orca delivery for omp is the injected dispatch path of Codex and Claude; a
  terminal-send prompt receipt is rejected.
- Trace accepts `--input-format omp-json` (omp emits the pi `message_end`
  usage shape that `omo-json` already reads).
- The host-probe conformance runner gains an omp runner. Each episode runs
  with a private `HOME` and `PI_CODING_AGENT_DIR`, `OMP_MCP_REQUIRE_READY=1`,
  `--no-extensions --no-skills --no-rules --no-tools`, and only the canonical
  lifecycle and context-guard extensions. Credentials come from a 0600 snapshot
  of the `auth_credentials` rows of `agent.db`, read through a read-only SQLite
  attach; the real `~/.omp` is never written. Live defaults stay Codex and
  Claude; an omp episode needs `--hosts omp` and an explicit `--model omp=…`.

## Consequences

Install, update, dry-run, native-integration verification, inspect, MCP
`host`/`owner_host` enums, CLI usage, and the response-contract goldens cover
four hosts. The MCP catalog digest changes, so the tracked Omo, omp, and agy
MCP templates are regenerated. Default installs still write nothing into
target repositories.

The `bun` receipt rule trusts the nearest `bun` ancestor. A `bun` process
between omp and issueops (for example `bun run` wrapping issueops) would become
the receipt; its exit then reads as a dead holder, which fails safe. cmux's
receiver check compares the observed executable with the launcher path, and a
bun-script omp is observed as `bun`, so cmux handoff to omp fails closed at
that check.
