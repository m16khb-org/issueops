# Pioneer skill instruction ownership review

## Decision

Use one canonical evaluator reference, with small self-contained runtime contracts in each skill.
Do not replace the entrypoint evidence blocks or invocation-specific integration rules with a mandatory shared-file read.
Centralize evaluation policy, not the instructions needed to perform an individual request.
This accepts a little repeated framing to preserve standalone use; copying the entire rubric would create competing owners.

## Current evidence and consumers

- `skills/algorithm-optimization/SKILL.md:20-33` requires five labeled clauses only for IssueOps artifacts or benchmark responses; speculative/refused optimization instead reports the no-change threshold.
- `skills/code-quality-metrics/SKILL.md:20-33` owns measurement-only output, diff inventory including untracked files, and the no-input guard.
- Other local output contracts are `skills/web-research/SKILL.md:22-34`, `skills/database-design/SKILL.md:19-30`, `skills/issueops-debugging/SKILL.md:22-36`, `skills/prompt-engineering/SKILL.md:20-32`, and `skills/git-operations/SKILL.md:22-34`.
- `skills/implementation-planning/SKILL.md:25-37` exempts unwarranted planning; `:435-452` distinguishes stage invocation from standalone use and forbids adding a new approval gate.
- `skills/verified-execution/SKILL.md:26-48` couples evidence to risk, execution ownership, canonical-worktree boundaries, and the narrower report-only mode. These are not generic benchmark boilerplate.
- `skills/code-quality-metrics/SKILL.md:365-374,473-484` owns baseline-before-cleanup and gate-after-cleanup timing. `skills/algorithm-optimization/SKILL.md:559-569` owns performance invocation and algorithm/invariant reporting.
- `.issueops/operations/pioneer-skill-quality-rubric.md:88-124` explicitly evaluates a fresh actor given only the target skill body and case request; the evaluator, not the actor, scores it.
- Current rubric consumers name it directly: `.issueops/operations/pioneer-skill-quality-cases.md:5`, `pioneer-skill-quality-scorecard.md:3`, `pioneer-skill-rerun-fixtures.md:5`, and `harness-skill-quality-scorecard.md:4` in that same directory.
- `.issueops/operations/pioneer-skill-quality-rubric.md:126-145` documents the labeled signature proxy; `internal/domain/issueopsbenchmark/dimensions.go:21` and `issueops_benchmark_score.go:154` in that directory identify the separate runtime dimension consumer.
- `skills/issueops/SKILL.md:8-10,21-27,58-74` already owns shared routing and authorization boundaries and identifies the stage consumers of pioneer skills.

## Bounded ownership proposal

| Owner | Owns | Does not own |
|---|---|---|
| `.issueops/operations/pioneer-skill-quality-rubric.md` | Evaluator protocol: cases, calibration, evidence grades, scoring version, holdouts, result records and quality gates. | Mandatory instructions for ordinary skill invocation; CLI availability. |
| Each existing pioneer `skills/<name>/SKILL.md` | Activation, local evidence labels, refusal/no-input exceptions, proportionality, domain-specific integration timing and artifacts. | A copied scoring rubric or generic evaluation ceremony. |
| `skills/issueops/SKILL.md` and its routed stages | Common cycle routing, authorization and stage-owned recording/publication workflow. | Replacing the domain skill's method or imposing an IssueOps cycle on standalone use. |
| Current Go dispatch/contracts | Executable commands, accepted flags and runtime schemas. | Treating a prose evaluation proposal as implemented behavior. |

Keep the existing local evidence blocks as the authoritative actor-facing requirements.
Make the rubric's per-skill label table an evaluator crosswalk, explicitly subordinate to those entrypoints, rather than a second independently maintained runtime contract.
Keep local integration summaries conditional on an actual cycle; retain each skill's specific timing, artifacts and exceptions.
Delegate common recording mechanics to the invoking IssueOps stage; do not reproduce unchecked feedback commands in every domain skill.
No new shared runtime file or installer change is needed for this ownership split.

## Installed paths and standalone correctness

- The invocation may supply only `SKILL.md` text: such a skill must still know its required outputs and stopping rules without filesystem access.
- For optional evaluator navigation, resolve a link from the real, symlink-resolved `SKILL.md` location, never from the target repository's working directory.
- From a source-backed `skills/<name>/SKILL.md`, `../../.issueops/operations/pioneer-skill-quality-rubric.md` reaches the rubric; resolving that path from a host's unresolved skill-link directory can reach the wrong tree.
- A separately copied skill may have no sibling skills or `.issueops` tree. Missing optional evaluation material must not block ordinary execution or justify an install.
- A full quality evaluation does require the evaluator's rubric. If it is absent, supply it to the evaluator or report evaluation incomplete; do not silently substitute the actor's summary.
- Preserve the current body-only evaluation protocol. Requiring injected reference bundles would be a separate protocol change, not a line-count cleanup.

## Runtime truth versus evaluation protocol

`cmd/issueops/issueopsapp/root_command_facade.go:33-67` registers `quality`; `cmd/issueops/issueopsapp/cli_facade.go:36-38` delegates to `qualitycli.Run`.
`cmd/issueops/qualitycli/quality_inspect.go:15-30` supports `inspect` and help only; `:33-48` parses `--repo`, `--json`, `--save-baseline`, and `--trend`.
Thus `issueops quality inspect --repo PATH --json` is supported syntax, not a skill-evaluation runner.
Reject `issueops skill-bench ...` and any invented `issueops quality benchmark ...` examples.
The independent review corrected the lead's incomplete command check: `feedback --help`, `feedback add --help`, and `status --help` all exit 0. Root dispatch also registers lifecycle commands at `cmd/issueops/issueopsapp/root_command_facade.go:70-73`; feedback is declared in `internal/contract/cli/issueops_catalog.go`.
Preserve feedback-recording duties. The examples in the named skills and at `skills/database-design/SKILL.md:76-83`, `skills/web-research/SKILL.md:359-368`, `skills/issueops-debugging/SKILL.md:368-378`, `skills/prompt-engineering/SKILL.md:455-465`, `skills/git-operations/SKILL.md:367-376`, and `skills/verified-execution/SKILL.md:438-451` omit actor context; route authenticated recording through the invoking stage instead of rejecting the command.
`internal/domain/selfverify/contract.go:10-14` defines runtime `self_verification_summary` version **7**; this is not the rubric's scoring version.
The rubric's v2 additions (`:162-222`) coexist with older five-dimension formulas/records (`:224-326,383-409,437-459`). Reconcile that precedence in the rubric owner, not in every skill.
Fresh-context isolation, calibration and holdout rules are evaluator procedures; the signature proxy is not semantic proof, and quality inspection does not execute those procedures.

## Acceptance boundary

A later implementation should check body-only standalone use, cycle-specific invocation, and optional reference resolution through a host symlink and a copied skill.
Confirm unchanged local labels and exceptions; verify any replacement command against actual dispatch before publishing it.
This report is a source-based design decision, not a benchmark result. No skill or production edits, installs, commits, or repository test runs were performed.
