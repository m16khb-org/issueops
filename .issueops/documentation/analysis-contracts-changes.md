# Analysis skill contract preservation

Initial worker snapshot. Subsequent contract-check corrections and final measurements
are recorded in `quality-audit-2026-10-03.md` and `quality-after-2026-10-03.json`.

## Scope and decision
Compacted three independently activated actor contracts; frontmatter name/description unchanged.
Read `owner-design-review.md` and `owner-preservation-review.md` first.
Keep execution rules in each SKILL.md; move optional examples/catalogs, not required outputs.
Rejected a mandatory shared runtime/evaluator dependency and a hard 250-line cutoff.
| Skill | SKILL.md before → after | Reference changes |
|---|---:|---|
| algorithm-optimization | 569 → 158 | New `references/algorithm-catalog.md`: 111 lines |
| code-quality-metrics | 483 → 233 | New `references/measurement-recipes.md`: 67 lines |
| implementation-planning | 495 → 302 | Existing `references/clearance-checklist.md`: 46 → 49 lines |
Entrypoints total: 1,547 → 693 lines. Planning intentionally exceeds 250: its full task/template
schema, standalone confirmation boundary, and stage-owned cycle sequence must work body-only.
Optional references resolve relative to the real skill file; none requires a sibling installation.

## Moved, consolidated, and clarified

- Algorithm: profiler/classification/data-structure/transformation/growth/space tables and
  documentation/background examples moved into the catalog. Repeated essays, rule lists,
  numeric benchmark examples, and proof illustrations became one actor method.
  Catalog separates LCS time/space and labels workload-dependent costs instead of false ceilings.
  Identical benchmark commands now illustrate comparable runs; open-handle checks are explicitly
  supplementary, not race detectors. Five clean race runs remain required for concurrency.
- Metrics: language-specific function extraction, overhead, and pre-PR recipes moved to a
  reference. All formulas, counting procedures, thresholds, snapshot/card/gate fields remain local.
  Removed fabricated-zero history fallback; preserved the supported state-read command.
  Tracked-only SNR is named as such and untracked counts must be combined before full-scope scoring.
- Planning: consolidated interview/termination/clearance repetition and compacted the complete
  template; no required template field moved. Existing checklist now explicitly elaborates
  auto-transition rather than contradicting the explicit-generation exception.
  Replaced early direct linkage with stage-owned staging/materialization/linkage ordering,
  grounded in `skills/issueops-plan/SKILL.md` and `skills/issueops-implement/SKILL.md`.

## Preserved critical actor constraints

### Algorithm optimization

- Specialist activation; not style cleanup. Profile first, proven CPU hot path, top-20 check,
  bounded-N/startup/I/O no-change refusal with input/frequency threshold; missing evidence → measure.
- Exact labels: Hot path; Complexity; Scaling evidence; Correctness invariant; Before/after measurement.
  Speculative/refused optimization omits the block; a label proxy is not semantic proof.
- Saved baseline, same environment/generator/flags, warmup/setup exclusion, confidence/noise,
  five-run ±5% stability, default <5% inconclusive, CPU/memory/allocation and user-facing p95/p99.
- Problem classification, simple structures, time/space and update/preprocessing trade-offs,
  worst/average/amortized analysis; scaled inputs and code analysis, not keyword claims.
- Proportional proof: concise invariant + regression for simple substitutions; full pre/post,
  loop initialization/preservation/exit and termination for novel graph/DP/concurrency.
- Visible structured control flow, bit-identical behavior/edge tests, allocation/GC audit,
  shared-state/synchronization/short-lock/lock-order rules, five clean race runs.
- Document algorithm/alternatives/invariants/results; stage-authenticated feedback and issue/PR
  description duties. Stop on success, optimal algorithm, no bottleneck, noise, changed behavior
  (revert own optimization), or three unsuccessful attempts; no commit/publication authority implied.

### Code quality metrics

- Measurement-only activation and exact labels: Diff inventory; SNR before/after; Secondary metric;
  Heuristic caveat; No-input guard. Staged/unstaged/untracked inventory; no git-diff-only scoring.
- Zero-input/empty-file guards, docs/config N/A with line deltas, approximation disclosure,
  command/error versus no-match handling, alias bypass, explicit approval for global installs.
- Four formulas/procedures; SNR bands and baseline ≥0.60; entropy >6/>12; >80% similarity ratio,
  baseline same-file >6-line duplicates; >50% overhead; all four gate targets and original card rows.
- Snapshot fields: measured_at, scope, git_status_short, changed_files, snr counts/value/passed,
  entropy exceedances/total/passed, redundancy duplicate_blocks/passed, overhead file counts/passed.
- Baseline → regression → target → executor cleanup → identical-command gate → feedback.
  Regression blockers: SNR drop >0.10, more >12 functions, new duplicates, overhead rise >10%.
- Ignored/state/temp snapshots distinct from committed cycle reports; target priority order;
  shannonAudit before/after/target/passed and overall fields; execution quality-gate recording.
- Pre-PR >200-line full SNR, >250 LOC, >4 nesting, <60% coverage, unused-code checks.
  Stop on recorded pass, justified SNR ceiling waiver, documented entropy ceiling, measurement
  failure diagnosis, no-input/N/A, or baseline-only handoff without fabricated after results.

### Implementation planning

- Explicit/complexity-based activation; direct-execution three-line routing for unwarranted plans;
  explicit planner remains non-implementing. Five artifact labels remain exact.
- Read-only discovery before questions, facts versus preferences/defaults, no duplicate delegated
  exploration; bounded interview/research output, actionable turn endings, draft updates.
- Six clearance criteria/recheck, explicit trigger exception, gap analysis/classification,
  visible unresolved decisions; one plan, incremental write/readback, complete task/template schema.
- Per-task categories/ownership/dependencies/references/acceptance/happy+failure QA/commit fields;
  exact data/assertions/evidence, implementation+tests together, four final checks after implementation.
- Standalone execution choice and explicit final user "okay"; cycle adds neither gate.
  Draft deletion only after completion; two unproductive research waves/two failed-section stops.
- Exact lifecycle/native actor/generation/lease/canonical-worktree fences; no invented write flags.
  Four machine-checked cycle sections, G1..Gn CHECK/EXPECT, stage plan before reviews/preparation,
  digest-bound review/re-review/regression and contract-change feedback/issue-sync obligations.
- Prepare/materialize → authorized handoff → active-owner linkage → compatibility → ledger →
  implementation; RED/GREEN, cleanup recheck, verification evidence validity preserved.
  Orca/Herdr/current-session routing, old-holder release/new ownership, no double writer.
- Planning-only stops before preparation/lease/session/implementation; narrower holds/endpoints win;
  stage status is not approval; merge/deploy/destructive cleanup require separate authority.

## Verification and limits

- `python3 scripts/validate-skill.py skills/algorithm-optimization skills/code-quality-metrics skills/implementation-planning`
  → exit 0, all three `ok: Skill is valid!` (final run after entrypoint/checklist edits).
- `git diff --check` → exit 0, no output.
- `wc -l .issueops/documentation/analysis-contracts-changes.md` → 118; below the 120-line limit.
- `./bin/issueops feedback add --help` and `./bin/issueops status --help` → exit 0;
  feedback exposes native actor fields. Recording routes through the owner, not invented flags.
- `./bin/issueops quality --help` → exit 0, `issueops quality inspect [--repo PATH] [--json]`.
  It is not a semantic skill runner. Unsupported `issueops skill-bench` is not advertised.
- Eval JavaScript checks: `await read(file)`; extract local links with
  `/\[[^\]]*\]\(([^)]+)\)/g`; resolve with `path.resolve(path.dirname(await fs.realpath(file)), target)`;
  `await fs.access(resolved)` and require same-skill containment → 6/6 links pass.
  Nine installed roots under `~/.codex/skills`, `~/.claude/skills`, `~/.omo/agent/skills`
  resolve to this source and their references pass. Initial `~/.agents/skills` probe was absent;
  corrected to actual Codex location without installation.
- Eval compares `content.split('---')[1]` to pre-edit `originals[name].split('---')[1]` → 3/3 unchanged.
  Counts use `text.trimEnd().split('\n').length`; same-skill closure supports copied-directory use.
- Body-only desk review checked refusal/hot-path/noise/concurrency; zero/docs/untracked/error/baseline;
  trivial routing/explicit planner/defaults/standalone approval/cycle endpoint cases against each root.
  No fresh actor benchmark or semantic score is claimed; copied-directory closure is not a copy install.
- No prose-pinning tests or full self-verify run. Existing evaluator contradictions/signature-refusal
  mismatch remain outside scope; no historical results changed.
- All seven files authored here are scoped Markdown (three roots, three references, this report).
  `git status --short` also shows concurrent edits outside scope; none was edited or reverted here.
