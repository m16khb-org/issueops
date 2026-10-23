# Cycle Lane: Authority, Startup and Shell Safety

Moved from SKILL.md "Cycle Lane: Authority and Startup". Read when a cycle exists, before the owner starts, replaces a holder, runs verification commands, or publishes.

Before execution, verify that the plan states the current issue, exact lifecycle ID, branch, base SHA, canonical worktree, bounded scope, acceptance criteria, verification, completion, and cleanup boundary. Never link an unrelated plan as readiness evidence.

After the canonical worktree exists, the active generation holder owns plan and implementation edits there. The source checkout may observe the selected cycle but must not steer or mutate it.

Do not cite stale tools such as positional `state write <key> <content>` forms as executable commands. IssueOps v1 liveness is the persisted generation plus exact native process receipt; inspect it with `issueops execution status --id "$ISSUEOPS_ID" --json`.

For the selected execution, the source checkout is observation-only. From it, use only non-mutating reads such as `git status`, `git diff`, `git log`, `git show`, `git rev-parse`, `git ls-files`, and `rg`. Tests, builds, formatting, installation, generation, commits, and publication run only in the canonical worktree. CLI/MCP generation, actor, and cwd denials must never be bypassed. The owner uses the installed `issueops` command unless the plan shows `./bin/issueops` exists in the exact worktree.

For a report-only cycle, run only the verification commands declared in the plan. Do not invent API, provider-ref, or history probes; the bounded report is not authority to widen verification or inspect unrelated external state. If a declared command cannot run, record the exact failure instead of substituting a new probe.

The Verified Execution report path is a safe relative path from the canonical worktree. The report must exist inside that root as committed regular-file content; an absolute path, parent escape, or leaf symlink fails. The worktree must be clean before completion.

Execution shell red flags include the eval and source primitives, active command substitution, unquoted process substitution, zsh equals expansion (`=git` or `=(...)`), parameter/tilde expansion, and unquoted brace/glob pathname expansion. Use explicit canonical argv paths. Do not steer a launched native owner through raw terminal injection; lifecycle control uses the persisted execution status, replacement, reconciliation, release, and completion commands.

Before draft publication, query `git rev-parse HEAD` in the canonical worktree as a standalone observation. Use the exact active generation, explicit head/base branches, labels, assignee, native actor, and cwd for `issueops remote create-pr`. Do not use implicit branch defaults, force/delete push, merge, or close.

For supervised evidence, self-verify requires binary/source contract parity. If an evidence worker is intentionally on a base checkout while the installed binary is feature HEAD, record a response-contract mismatch as a version-skew observation, do not mutate the base, and leave the final self-verify score to the coordinator running matching feature HEAD. The opt-in LLM path currently renders a read-only prompt only. No Z.AI request is sent, so `gate` is expected to remain non-passing without an ingested verdict. When the coordinator environment intentionally exports `ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate`, use explicit `--llm-eval=false` for the required deterministic completion sequence, record the override, and restart from its first gate after an interrupted or prompt-only run.

Hooks are context-only: SessionStart emits the static project-doc catalog, and `post-compact` remains available for Omo and diagnostics. Hooks do not enforce tool mutations or own execution authority; CLI/MCP commands enforce their own generation, native actor, and canonical cwd fences. For focused hook coverage, use `./cmd/issueops/hookcli` and its retained `./cmd/issueops/hookcli/hookinput` tests; `./internal/core/hookinput` does not exist.

A targeted Go test is GREEN only when the intended test names actually ran. `[no tests to run]` is not GREEN; update stale regex names and rerun with `-v`, requiring the named `=== RUN` lines and PASS:

```bash
go test -v ./cmd/issueops/hookcli -run '^TestRetiredHookSubcommandsAreRejected$' -count=1
```

When a zsh verification wrapper captures an exit code, never assign to `status`: zsh reserves `status` as a read-only parameter. Use `rc` or `exit_code`, and report the test command verdict separately from wrapper bookkeeping errors.

Shell arguments containing Markdown backticks must be single-quoted or passed as direct argv; never place backticks inside a double-quoted shell command argument, where zsh executes command substitution.

Native owner startup must use the exact installed Codex or Claude command and the canonical worktree. A hook-trust, usage-limit, rate-limit, reset, or model-selection prompt is a user decision boundary; never automate it. Resume only after the native process receipt and generation claim are observable.

Before any replacement, inspect the exact native process, canonical worktree, branch, HEAD, and dirty paths. Any possible writer blocks another writer even when the diff appears stable. A stable diff is not lease evidence. Follow preview → revoke → finalize-preview → finalize with generation and inventory fingerprints; never adopt WIP while the old writer may still be active.

Startup evidence is not a convenience bundle: run cwd, git root, branch, HEAD, dirty paths, and source-checkout status as independent commands. A combined command loses which raw read or exit established authority and must be rerun before mutation. When a long suite fails, inspect and fix the first failing test/golden before another unchanged long rerun.

Preview `issueops execution prepare` first and review workspace root, branch, base SHA, and next command. Confirmation repeats the identical request with only `--confirm`. Any later ambiguity uses `issueops execution reconcile` and never another create attempt.

Installed-file readback does not prove an already running host process loaded the new code. MCP runs in-process; reconnect the host's MCP server to apply an updated binary. Inspect `issueops execution whoami --json` and `issueops execution status --id "$ISSUEOPS_ID" --json` for current native identity and lease authority rather than probing retired enforcement hooks.

When an owner is blocked, it remains mutation-free and returns the exact state error and rendered next command in the fixed report. It must not create a second decision system. The active holder publishes and verifies the draft PR/MR, then records `issueops execution complete` with the exact lifecycle ID, generation, actor, cwd, committed report, final HEAD, artifact URL, and verification results. CLI/MCP commands own this boundary; hooks only supply project-doc context.

A yielded execution cell is unfinished evidence. Poll that exact cell or its returned process session through a terminal exit and capture the final exit/output before counting, replacing, or proceeding past the gate; if an edit follows, restart the ordered verification gate from step 1. Never infer completion from partial package output or from starting a later command.

Never construct `gofmt -w` arguments with shell command substitution such as `$(git diff --name-only ...)`. Inspect and verify the changed Go paths, then invoke `gofmt` with the explicit direct argv list so whitespace, glob, and option-like filenames cannot change the formatting scope.

Before this fence, each worker commit must use a Conventional Commit subject and a literal `Lore:` block with `Intent`, `Why`, `Changes`, `Verify`, and `Risk` as required by `.issueops/COMMIT_POLICY.md`.

A completed execution is never a new mutation lease. Review feedback that requires edits starts a new bounded execution or an explicitly authorized continuation before completion.

Pending external intent survives interruption: reconcile ambiguous workspace/publication state, or replace a failed holder with exact generation and quiescence evidence. Record before/after process, worktree, branch, HEAD, and dirty-path observations. Cleanup remains a separate human-authorized operation after verified merge evidence.
