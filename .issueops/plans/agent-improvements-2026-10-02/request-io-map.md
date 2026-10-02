# I6 — request-scoped Git observation plan

## Decision

Reuse the existing operation-local readiness observer and add only the missing
request-local Git/history observations. Do not add a package-global or
cross-request cache. A cached Git fact is observational data, never proof of
actor, lease holder, generation, workspace authorization, or permission: keep
those checks on their existing live paths.

First target the measured-by-count, not measured-by-time, duplicates:

- Preflight (`internal/adapter/preflight/preflight.go:17-57`) performs separate
  log reads for last commit, 5 recent commits, 10 style commits, and 10 bodies.
  Replace those overlapping reads with one bounded `log -10` result and derive
  all four projections while preserving existing order, formatting, limits,
  body parsing, and empty/error behavior. Keep distinct Git facts (root,
  branch, HEAD, upstream, status, remotes, ahead/behind) distinct. `helpers.go`
  owns parsing/projection; `git.go` remains the command boundary.
- Readiness already memoizes exact branch/status calls by literal root and
  argv in `internal/adapter/issueops/readiness_git.go:16-53`. Preserve its
  tuple semantics, exact keys, per-operation lifetime, and fetch invalidation.
  Extend this exact-root/exact-argv pattern only for a demonstrated duplicate;
  do not broadly memoize HEAD, refs, counts, or authorization-sensitive checks.
- Next has a separate, request-local reuse for local readiness/review paths
  (`cmd/issueops/issueopsapp/issueops_next_wiring.go:35-118`). The concrete
  duplicate is `CurrentBranch(cwd)` versus `WorktreeState(root)` when cwd and
  execution root identify the same worktree: `WorktreeState` reads root,
  branch, and HEAD (`:147-153`). Introduce a small request-scoped observer at
  this wiring seam so matching exact-root+argv observations are shared within
  one `Next` call only. Do not merge observations when root strings differ. Preserve
  service phase gating and warnings (`internal/application/issueopsnext/service.go:140-185`).

The preflight history consolidation is independent of next/readiness wiring.
The shared abstraction should be a tiny scoped memo/observer, not a universal
Git snapshot DTO unless implementation proves multiple callers need identical
facts and parsing. Avoid caching whole readiness results: filesystem facts and
change fingerprints have separate freshness and verification contracts.

## Exact test seams and cases

1. **Preflight seam:** inject a `GitCmd`-compatible runner into an unexported
   observer constructor (default production runner remains `GitCmd`), then use
   a recording fake returning deterministic history output.
   Assert one history invocation for a successful preflight and unchanged
   last/recent/style/body projections, including fewer than 10 commits, empty
   bodies, and malformed/unexpected records. Retain
   `internal/adapter/preflight/preflight_test.go:9-65` as real-Git behavior
   coverage. Do not add timing assertions.
2. **Readiness seam:** extend
   `internal/adapter/issueops/readiness_git_observation_test.go:12-121`.
   Assert same root+argv is one run; `/repo` vs `/repo/.`, changed ref argv,
   and unsupported argv stay separate/fresh; returned code/stdout/stderr,
   including failures, remain identical. Fetch between observations must make
   the next branch/status read fresh. Existing cleanup parity and
   `readiness_scope_test.go:70-88` ensure a later operation sees changed HEAD.
3. **Next seam:** inject a counting Git runner into the wiring observer and
   exercise current-branch matching plus `WorktreeState`. Same exact
   worktree/root should execute an overlapping branch observation once;
   different root strings must each execute it. A second `Next` invocation after an
   edit must observe changed branch/HEAD/status, not reuse the first call.
   Preserve `issueops_next_observation_test.go:11-94` same-record,
   defensive-copy, unverified-path, and fallback cases. No authorization
   decision may be sourced from the memo.
4. **Concurrent edit boundary:** deterministic fake-runner tests should change
   returned Git state between request scopes and after an explicit invalidation
   (including fetch), then assert the next observation gets the new value and
   exact call counts. No sleeps. Real concurrent filesystem/Git mutation can
   occur between separate subprocesses; these commands do not provide an
   atomic repository snapshot. Do not claim otherwise. Decide whether a
   stronger in-operation consistency contract is required before adding locks,
   retries, or a single-command protocol.

## Ownership and dependency order

- Preflight history: `internal/adapter/preflight/{preflight.go,helpers.go}` and
  `preflight_test.go`; independent.
- Readiness cache: `internal/adapter/issueops/readiness_git.go` and
  `readiness_git_observation_test.go`; preserve current public API where possible.
- Next request observer: `cmd/issueops/issueopsapp/issueops_next_wiring.go`
  plus a focused wiring test (prefer extending
  `issueops_next_observation_test.go`); depends only on its own seam, not
  preflight history consolidation.
- Parent owns cross-scope integration. Do not edit unrelated files or run broad
  verification as part of this I6 implementation track.

## Executable focused validation

```sh
go test ./internal/adapter/preflight -run 'TestGitPreflight' -count=1
go test ./internal/adapter/issueops -run 'TestGitObservation' -count=1
go test ./cmd/issueops/issueopsapp -run 'TestNextLocalReadinessObservation|TestNextGitObservation' -count=1
```

The first two commands are existing package tests; the final test name is the
proposed new next wiring regression test. Do not report performance improvement
without a separate controlled benchmark.

## Unresolved decisions

- The preflight history result formats differ (`%h %s`, tab-delimited subject,
  and full body). Choose a collision-safe delimiter/record parser, or retain
  separate reads if one command cannot preserve exact output semantics.
- “Same root” must use the repository’s established root identity rules.
  Existing readiness cache deliberately keys literal root strings; changing
  that equivalence is not implied by this plan.
- External edits cannot invalidate an in-memory request snapshot automatically.
  Define any required concurrent-edit guarantee before implementation; current
  evidence supports request/operation boundaries and explicit fetch reset only.

Research basis: `.issueops/research/agent-updates-2026-10-02/issueops-06.md`
and `issueops-07.md`. Their call counts are static source-derived evidence;
performance gains remain hypotheses.
