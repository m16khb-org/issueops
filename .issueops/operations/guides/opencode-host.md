---
name: opencode-host.md
description: opencode v2 skills, MCP registration, context plugin, and standalone lease operations.
---

# opencode Host Operations

opencode v2 keeps user configuration under `$XDG_CONFIG_HOME/opencode`, or
`~/.config/opencode` when `XDG_CONFIG_HOME` is unset or relative:

- User skills: `~/.config/opencode/skills/<skill>/SKILL.md` (symlinks). A skill
  that also exists in `~/.claude/skills` is listed once; the config copy wins.
- User MCP: `~/.config/opencode/opencode.json`, entry `mcp.servers.issueops`
  (mode `0600`; HTTP `type: remote` with bearer and
  `X-Issueops-Mcp-Catalog-Sha256` headers and `oauth: false`, or stdio
  `type: local` with `ISSUEOPS_MCP_CATALOG_SHA256` in `environment`;
  `codemode: false` in both, so the tools appear as native `issueops_*`
  tools). Other keys and other servers stay as they were.
- User lifecycle plugin: `~/.config/opencode/plugins/issueops.js`. Other
  plugin files are not touched.
- Project MCP: `.opencode/opencode.json` (stdio `issueops_project`, removed
  under HTTP transport) only with explicit `--project-local`; no project skill
  links.

Tracked templates are `configs/opencode/opencode.json` and
`configs/opencode/issueops.js`. issueops never writes `opencode.jsonc`.
opencode loads it after `opencode.json` and lets it replace a same-named
server whole, so an `mcp.servers.issueops` entry there fails the install with
`opencode.jsonc defines mcp.servers.issueops and would shadow the managed
entry`; delete that entry from `opencode.jsonc` and run the install again. An
unreadable `opencode.jsonc` only adds a warning. An `opencode.json` with
comments or trailing commas, which opencode accepts, fails the install with
`<path>: opencode.json has comments or trailing commas`, because the managed
merge writes plain JSON back and would drop them; move the commented settings
into `opencode.jsonc` or delete the comments, then run the install again.

## Context plugin

The plugin registers one `context` session hook. For each session it runs
`issueops hook session-start --json` once (a subagent session, found through
`ctx.session.get` and its `parentID`, runs `hook subagent-start --json` with
its agent type on stdin), caches the catalog for up to 512 sessions, and
appends it to the system prompt of every model call, which also covers the
first call after a compaction. A failed or timed-out hook (10 seconds) is
cached as no catalog and logged once. The plugin never writes `process.env`;
opencode exports `OPENCODE_SESSION_ID` to each session's shell tools itself.

## Lease ownership

Only a `--standalone` session can own an IssueOps lease. `execution whoami`
picks the nearest `opencode` ancestor, and core verification (claim, prepare,
mutation authority, `mcp authorize`) accepts an opencode receipt only when the
live process arguments are `opencode serve --stdio`. A session of the shared
background service (`opencode serve --service`) is refused. Over HTTP its
workspace MCP tools then fail with `authority_required`, while server-scope
tools and argument-free `harness_inspect` still work; over stdio the MCP
process has no session id, so `issueops_execution` mutations fail. A subagent
shell carries its child session id, which never matches the holder.

Install/update strict readback seals the MCP entry and plugin bytes before
activation commits. Plugin updates reach an already running opencode only
after `opencode service restart` or a new `--standalone` session.

## Checks

```bash
test -f ~/.config/opencode/skills/atomic-commit-push/SKILL.md
test -f ~/.config/opencode/opencode.json
test -f ~/.config/opencode/plugins/issueops.js
```

Agent-harness does not install, upgrade, or restart opencode itself. To remove
the integration by hand, delete `plugins/issueops.js`, the
`mcp.servers.issueops` entry, and the issueops skill links under the opencode
config directory.
