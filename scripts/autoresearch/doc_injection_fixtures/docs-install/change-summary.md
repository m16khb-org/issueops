# Implementation summary for the docs stage

## git diff --stat (base..HEAD)

```text
 cmd/issueops/hookcli/hook.go                   | 12 +++++++++++
 cmd/issueops/hookcli/hook_pretooluse.go        | 58 ++++++++++++++++++++++++++++++++++++++++++
 cmd/issueops/installcli/install.go             | 18 ++++++++++++++----
 cmd/issueops/installcli/install_hooks.go       | 27 ++++++++++++++++++++++-----
 cmd/issueops/installcli/install_hooks_test.go  | 63 +++++++++++++++++++++++++++++++++++++++++++
 configs/claude/settings.hooks.json             |  9 +++++++++
 configs/codex/hooks.json                       |  9 +++++++++
```

## What changed

- New flag `issueops install --skip-hooks`: installs skills and MCP registration but leaves every host's hook
  settings untouched. Default behavior is unchanged.
- The default install now also registers a `PreToolUse` hook for Codex and Claude:
  `issueops hook pre-tool-use --host <host>`. It reads the active IssueOps lease from the state store and blocks
  shell commands that write outside the canonical worktree while a lease is active. Users asked for this after
  an agent edited the source checkout during stage 4.
- Pitfall found while implementing: `issueops install --dry-run` still executed `claude mcp list` to observe the
  Claude registration, which spawned the Claude CLI (and its hooks) during a dry run. Fixed by reading
  `~/.claude.json` directly in dry-run mode.

## Verification

```text
$ go test ./cmd/issueops/installcli/ ./cmd/issueops/hookcli/ -count=1
ok  	issueops/cmd/issueops/installcli	1.84s
ok  	issueops/cmd/issueops/hookcli	0.92s
```
