# IssueOps CLI startup and process execution evidence

Retrieval date: **2026-10-02**. Scope: current local implementation, neighboring tests, and official Go documentation. Model: `gpt-6.1-sol` (session `PI_MODEL`). No benchmarks, tests, installation, production changes, or external mutations were performed. All optimization benefits below are hypotheses, not measured improvements.

Version context: `go.mod:1-3` declares module `issueops` and Go **1.26.3**; this is a repository requirement, not proof of an installed compiler or an IssueOps release date. Relevant upstream APIs are already released: Go **1.20**, released **2023-02-01**, added `Cmd.Cancel` and `Cmd.WaitDelay` [official announcement](https://go.dev/blog/go1.20), [release notes, os/exec](https://go.dev/doc/go1.20#os/exec). These are same-publisher sources, not independent corroboration. No anticipated release is used.

## Findings

### 1. Root dispatch does not establish universal Git startup overhead

**High certainty; local source evidence.** `cmd/issueops/main.go:11-16` delegates to `RunRootCommand`; `cmd/issueops/issueopsapp/root_command_facade.go:18-31` wires dependencies once per process. Its actual `wireBasicCLIDeps` body is empty (`cli_facade.go:16-18`). Root help/version return before subcommand dispatch (`cmd/issueops/rootcmd/root_command.go:21-36`). The neighboring `root_command_test.go:11-73` checks usage/version and runner dispatch, but does not measure child-process counts.

**Applicability:** investigate selected command paths rather than describing every invocation as a repeated Git startup. Harness-root discovery itself uses environment, executable/symlink resolution, and ancestor filesystem checks (`cmd/issueops/pathutil/path_helpers.go:16-66`), not Git.

**Counterevidence/limit:** this establishes the inspected dispatch path, not the absence of all transitive package-initialization cost or subprocesses in every subcommand.

### 2. Git preflight has concrete sequential subprocess fan-out

**High certainty; static call-count derivation.** After successful root discovery, `internal/adapter/preflight/preflight.go:17-57` invokes ten Git commands without an upstream, eleven with one: root, branch, short HEAD, upstream, status, remote listing, four log queries, and conditional ahead/behind counts. Helpers confirm one command per remote/log request (`helpers.go:10-31`). Each reaches a synchronous `exec.Command("git", ...).Run()` (`git.go:9-38`).

**Applicability:** the four overlapping history reads are a bounded candidate for one history snapshot with derived projections. Preserve the five-entry recent list, ten-entry style sample, last-commit formatting, and body parsing.

**Counterevidence:** these requests serve different fields, not identical argv duplicates. `preflight_test.go:9-65` checks conventional/Lore hints, secret-like paths, and staged/unstaged behavior. It contains no latency or spawn-count assertion; fan-out alone proves no bottleneck.

### 3. Readiness already reuses selected observations correctly

**High certainty; implementation plus same-repository tests.** `internal/adapter/issueops/readiness_git.go:16-54` caches only exact two-argument branch/status queries, keyed by root and argv, retaining failures and stderr. Fetch clears the cache; other probes remain fresh. Production wiring creates the shared readiness/cleanup scope (`cmd/issueops/issueopsapp/cycle_readiness_wiring.go:23-36`).

**Applicability:** preserve this operation-local boundary when evaluating further deduplication. Tests cover tuple preservation, cleanup parity, distinct root strings, argument differences, and fetch/reset invalidation (`readiness_git_observation_test.go:12-121`).

**Counterevidence:** a claim that readiness has no cache is false. This narrow cache does not eliminate HEAD/count probes or span separate CLI processes. Repository tests support intended behavior, not independent performance corroboration.

### 4. Direct Git and policy execution have materially different bounds

**High certainty.** `internal/adapter/preflight/git.go:16-31` uses unbounded `bytes.Buffer` capture and sets no context deadline, process group, or explicit environment filter. In contrast, `internal/adapter/policy/policy_run.go:53-126` filters environment, drains both owned pipes, waits for process and stream completion, and closes readers after a 250 ms post-timeout grace. Capture retains at most 64 KiB per stream (`policy_run.go:130-160`). Unix termination targets the process group; other platforms kill only the direct child (`process_group_unix.go:12-20`, `process_group_other.go:7-9`).

**Applicability:** inventory direct Git callers before considering a bounded runner contract.

**Counterevidence:** policy execution already handles inherited pipes; replacing it with plain `CommandContext` could lose guarantees. Official [os/exec documentation](https://pkg.go.dev/os/exec#CommandContext) says default cancellation kills the process and leaves `WaitDelay` unset. Bounds tests exist (`policy_run_bounds_test.go:85-148`), but were read, not run.

### 5. Existing duration is not subprocess-only visibility

**High certainty.** `internal/application/policy/runner.go:62-86` starts timing before policy evaluation and finishes after executor completion; reported duration includes evaluation and pipe draining, while output redaction follows the finish timestamp. Direct `GitCmdRaw` returns only exit/stdout/stderr (`preflight/git.go:16-31`).

**Applicability:** distinguish evaluation, spawn/wait, drain, and total command duration before attributing cost.

**Counterevidence:** timing already exists; it simply cannot isolate these stages. Timeout/fake-run contracts are covered by `internal/application/policy/runner_test.go:23-50`.

## EXPAND

- **EXPAND-MEASURE:** authorize focused per-command spawn counts and timings before ranking history consolidation.
- **EXPAND-CONTRACT:** trace direct Git callers' raw-output and cancellation requirements before runner unification.
- **EXPAND-VISIBILITY:** design opt-in redacted stage timings without changing default CLI/MCP responses.

Verification: cited public URLs were reopened successfully; source and test anchors were read. The requested report-scoped `git diff --check` exited 0. The report is untracked, so that command alone does not validate its contents; manual review supplemented it.

Limits: two exploratory reads returned ENOENT: `cmd/issueops/issueopsapp/wire.go` and `cmd/issueops/rootcmd/root.go`; actual files were located. No public source was inaccessible and no validation command failed. Broad diagnostics/tests/builds and benchmarks were excluded by scope.
