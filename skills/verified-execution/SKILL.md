---
name: verified-execution
description: "Use when the user requests verified delivery, an evidence-led execution loop, or durable goal tracking with measurable completion criteria and real usage evidence."
---

# Verified Execution

## Activation and Routing

The main agent executes goals, fixes code, tests, and drives observable QA.
This body owns the complete actor contract, including isolated/body-only use;
no sibling skill, project-doc tree, or evaluator bundle is required. Resolve
optional references from the real, symlink-resolved skill directory, not cwd.

Decide risk first: low-risk reversible work uses the proportionate one-line
ledger/CLI evidence exception below; user-facing, hard-to-reverse or multi-criterion
work uses the full loop. Keep real evidence, cleanup receipts and honest outcomes
in either mode. Measure cycle time/rework/coverage, not confidence in your own work.
For a cycle, follow the lifecycle lane and authenticated owner; otherwise skip
the cycle-only fences and use State and criteria → Per-Criterion Loop → Standalone Gate.

For repository-local symbol discovery, use CodeGraph first when `.codegraph/` exists; otherwise use local `rg` and direct reads only. Never use web search for local repository symbols. Run verification and inspection commands as separate calls; never chain them with `echo` or `printf` banner markers.

First-party hosts are exactly Codex, Claude Code, Omo native, omp, and opencode.

## IssueOps Benchmark Artifact Contract

When Verified Execution contributes to an IssueOps artifact or benchmark response, include a compact labeled evidence block. Scale the evidence weight to the risk, but keep the labels so the artifact proves the method was applied.

```text
Success criteria: <criterion ids and binary pass/fail definitions>
Evidence artifact: <path, transcript, stdout, screenshot, or parsed dump>
Cleanup receipt: <runtime/temp state removed and verified, or "none spawned">
Verification mode: <full loop or proportionate lightweight mode, with rationale>
Skipped checks: <checks skipped with explicit reason; "none" if all ran>
```

## Cycle Lane: Authority and Startup (Only When a Cycle Exists)

Render every acceptance criterion as a binary observation. Use the exact
13-field owner report below, including evidence paths, HEAD, changed files,
artifact URL, verification and the `issueops execution complete` receipt.

Key rules (full text in [cycle lane details](references/cycle-lane.md); read it before starting, replacing a holder, running verification, or publishing):
- The source checkout is observation-only (non-mutating reads only); tests, builds, formatting, commits and publication run only in the canonical worktree. Never bypass CLI/MCP generation, actor, or cwd denials.
- Never cite stale positional `state write` forms; inspect liveness with `issueops execution status --id "$ISSUEOPS_ID" --json`.
- A hook-trust, usage-limit, rate-limit, reset, or model-selection prompt is a user decision boundary; never automate it or inject raw terminal input into a native owner.
- Any possible writer blocks another writer; a stable diff is not lease evidence. Follow preview → revoke → finalize-preview → finalize.
- Run startup evidence and verification as independent commands; a targeted Go test is GREEN only when the intended test names ran (`-v`).
- A yielded execution cell is unfinished evidence; a completed execution is never a new mutation lease.
- The report path is a safe relative path inside the canonical worktree, committed, with a clean worktree before completion.


## Execution Mode and Evidence

Choose risk before execution. Full mode is for user-facing, hard-to-reverse, or
multi-criterion work: goals, append-only ledger, per-criterion artifacts, metrics,
and a binding adversarial reviewer. For trivially reversible docs, wording,
single-file validation, or config work, a one-line pass/fail ledger and command
stdout/diff are enough; goals/metrics are optional. Record the risk rationale and
review skip explicitly. Both modes require observable evidence, honest results,
and verified cleanup (or "none spawned"). Tests alone never prove completion.

For each full-mode criterion, actually run one channel: HTTP (status + headers +
body), terminal/tmux (transcript), browser (actions + screenshot), or computer use
(actions + screenshot; AppleScript on macOS, `xdotool` on Linux only). Use available
host tools. CLI stdout, DB diffs and parsed config are valid auxiliary evidence
for CLI/data criteria and the low-risk exception, never substitutes for a
user-facing channel. Dry-run, printed commands, speculation and "looks correct"
are not usage evidence. Artifacts must be nonempty, untruncated, reproducible,
make pass/fail binary, and name goal + criterion + channel. If output exceeds
32 KB, record the truncation point and retain/read the complete artifact before PASS.

### State and criteria (before implementation)

Prefer IssueOps state; otherwise use local files, never invented state elsewhere:
- IssueOps: `.issueops/verified-execution/`; fallback: `.verified-execution/`.
- Each root contains `goals.json`, append-only `ledger.jsonl`, and `evidence/<goal>-<criterion>-<channel>.<ext>`.
- Read availability with `issueops state read --key verified-execution-goals-<repo-hash>`;
  checkpoint with `issueops state write --key verified-execution-goals-<repo-hash> --input goals.json --json`.

Read the user brief, plan, or cycle intent; no usable goal means clarify before
execution, not fabricated criteria. Plan TODOs map 1:1 to criteria. Goal fields:
`id`, `title`, `objective`, `status`, `successCriteria`. Each criterion has unique
`id`, exact `scenario` (tool, steps, inputs, binary outcome), `channel`,
`expectedEvidence` path, `status`, `capturedEvidence`, `cleanupReceipt`, and
applicable `ultraqaClasses`. Initial status is pending and captured evidence/receipt
are null. Pick relevant adversarial classes: `malformed_input`, `prompt_injection`,
`cancel_resume`, `stale_state`, `dirty_worktree`, `hung_command`, `flaky_test`,
`misleading_success`, `repeated_interruption`. Do not proceed to a criterion without
a concrete expected artifact path.

The checkpoint is a JSON object with a top-level `goals` array and sibling `metrics` object, not a bare goal or array. Initialize all five metrics to 0.0. Example: [checkpoint schema](references/goals-checkpoint-schema.md).


For web usage evidence, use the current host's available browser tool.
In this repository, `skills/issueops/references/execution.md` provides supplemental
cycle details; it is not a prerequisite for this body-only standalone contract.

### Delegation boundary

The main agent implements, fixes, tests and drives QA. Delegate only these
net-positive patterns: high-volume exploration, devil's advocate, parallel
independent research, cross-verification, isolated-worktree edits, model
specialization, tool-gated read-only exploration, background long-running work,
plan/execute separation, forked-context exploration, independent task fan-out,
and triage to a specialist. No nested sub-agents. Small/single-file tasks, work
requiring full conversation context, cross-cutting architecture, and safety/
reversibility/alignment decisions stay with the main agent; overhead must pay off.

Every dispatch includes goal, exact files, baseline characterization test when
changing existing behavior, constraints/project rules, verification commands,
one QA channel and exact artifact path. Workers lack interview context. Use only
current host tools; if dispatch is unavailable/disallowed, record the limitation
and work directly. A required independent review remains unfulfilled, not an
invented approval. Verify every worker diff, tests, diagnostics and evidence yourself.

## Per-Criterion Loop

Cap one goal at 5 cycles and identical criterion failures at 3; checkpoint the
diagnosis at either limit. After 2+ failures use systematic root-cause diagnosis
(with the debugging skill if available), not another unchanged attempt.

1. **Plan:** read scenario, expected evidence and ledger; identify independent
   wave tasks. Register atomic todos: path, action, criterion, verification.
   Serialize only on a named dependency.
2. **Execute:** for existing code behavior, first pin current behavior with a
   passing characterization test. RED must fail for the intended requirement,
   not syntax/import errors. Capture it, then make the smallest GREEN change
   (roughly under 20 lines); a larger step calls for a finer test. Pure prose
   changes use validation/diff evidence, not prose-pinning tests.
3. **Integrate:** inspect your diff, run relevant tests and changed-file LSP
   diagnostics. Fix scope drift, hollow tests, or missing evidence. Re-verify
   isolated worker output; fix it directly or return specific failure context.
4. **Run scenario:** personally execute the named QA surface; a heavy browser/
   computer channel may use a dedicated QA-only specialist. On failure, fix the
   cause and rerun the same criterion. Capture transcript/stdout/screenshot/
   assertion/status+body/diff/dump at the expected path; missing artifact is BLOCKED.
5. **Clean before recording:** remove every spawned runtime resource and verify
   removal: PIDs (kill, failed kill -0), terminal sessions (kill and inventory),
   browser contexts (close), containers (remove), ports (empty lsof), temporary
   mktemp paths (remove), QA-only environment variables (unset). Retain evidence.
   Record resource → action → verification, or "none spawned"; leftovers are BLOCKED.
6. **Record once:** PASS needs artifact + receipt; FAIL needs captured output +
   diagnosis; BLOCKED needs evidence + blocker. Ledger fields are `ts` (ISO8601),
   `goal`, `criterion`, `status`, `evidence` (path and cleanup receipt), `rework`.
   Full evidence also records exact channel command, summary, attempts, rework,
   cycle (of 5), and timestamp. Update criterion state and checkpoint.
7. **Measure in full mode:** recompute `evidenceCoverage = passed_with_evidence / total_criteria`,
   `reworkRate = self_corrections / total_criteria`,
   `cycleEfficiency = completed_criteria / total_attempts`,
   `parallelizationRatio = total_tasks / waves_used`,
   `cleanupCompliance = cleanup_receipts / completed_scenarios`.
   Start metrics at zero. Targets: ≥95% coverage, ≤15% rework, ≥80% efficiency,
   ≥4x parallelization where independent work permits, 100% cleanup and durable
   cross-session survival. Comparative target is ≥20% improvement over the
   inherited ulw-loop baseline, not a claim of measured improvement. These
   targets never justify unnecessary delegation.
8. **Complete goal:** every criterion must pass with evidence; append `ts`,
   `goal`, `event: goal_complete` and current `metrics`. Only after all goals pass,
   enter the appropriate final gate below.

## Final Gate: Standalone Lane

Inside a cycle use its clean → docs → verify stages once, not this extra sequence.
The prepared-worktree confirmation sets the authorized endpoint; finishing this
skill creates no additional approval stop. Standalone order:

1. Targeted verification of changed behavior.
2. If cleanup is in scope, remove lazy/duplicated/unused diff residue without
   changing behavior (use the cleanup skill when available). Harness health
   `issueops self-verify` is not a generic cleanup substitute.
3. Re-verify after cleanup.
4. Full mode/user-facing/hard-to-reverse work requires a fresh adversarial reviewer
   given goal, criteria, artifacts and full diff. Its gate is binding: verify
   every concern, fix valid ones, return disconfirming evidence for invalid ones,
   rerun full scenario QA after fixes, and resubmit to the same reviewer until
   unconditional approval. "Looks good but" or LGTM without evidence review fails.
   Proportionate low-risk work may skip with its specific recorded rationale.
5. Record `aiSlopCleaner` (status/evidence), `verification` (status/commands/evidence),
   `codeReview` (status/recommendation/evidence), `criteriaCoverage` (totalCriteria/passCount),
   and `metrics` (all five fields above). Reviewer pass is `APPROVE`; a risk-based
   skip is `status: skipped, recommendation: null` with rationale, never fake approval.
   Record other inapplicable steps honestly rather than claiming they ran.

A failed step in an ordered verification sequence restarts from step 1; do not
reuse partial passes as a completed sequence. Cycle stages own valid evidence
reuse/resealing and their bounded review loop, not this standalone review loop.

## Cycle Recording and Ordered Completion

Use `issueops next --id "$ISSUEOPS_ID" --json` for lifecycle routing; implement
owns this execution loop, then clean → docs → verify → pr → completion. Never
jump directly from criterion PASS to pr. The owning stage records START/scenario,
PASS/evidence/cleanup and progress feedback with source `verified-execution`.
`issueops feedback add` and `issueops status` are supported aliases, not authority.
For authenticated recording, obtain `record_actor_flags` and `claim_actor_flags`
from `issueops execution whoami --json`, for records and lease operations
respectively; retain exact lifecycle ID, generation, native actor, canonical cwd.
Use the owning stage's rendered commands, not invented flags or direct phase jumps.
If it is absent, leave recording pending; standalone execution still works.

Preserve this order and the existing user-confirmed endpoint:
1. Record cleanup category and rerun verification; finish the report/implementation
   diff before entering ai-slop-clean with the sealed command.
2. Review project docs. If updates are needed, edit first, verify/reseal, then
   record `updated` with changed paths; otherwise record evidenced `no-change`.
   Never relabel old evidence as execution against a new fingerprint.
3. For migration/entity/SQL schema changes only, observe real DB indexes and target
   row counts (catalog/estimates, not large scans); record sources or justified waiver.
4. After acceptance and authorized verification pass, read current `next.review`
   model/effort/tier/lenses immediately before fresh independent review. Missing
   routing is blocked, not a fallback to prepare-time defaults. Record actual
   reviewer model/effort/verdict/findings; follow the stage's bounded escalation
   and override rules. Revise means fix/reverify/review; stop blocks publication;
   only pass proceeds. An owner-model choice is not a reviewer override.
5. Commit/push the sealed diff only within existing authorization. Changed diff
   requires renewed cleanup/verification/review. Verify clean/synced branch, enter
   pr, observe final HEAD separately, and preview the governed draft request with
   explicit branches, labels, assignee, actor/cwd/generation. Confirmation repeats
   the identical request with only `--confirm`; never infer remote-write approval.
6. Read back URL, target, labels, assignee and Korean body. Only then complete from
   pr with committed relative report, final HEAD, exact artifact URL and repeatable
   verification results. Verify completion receipt and lease release with status,
   then stop: no automatic merge, close, or worktree/branch cleanup.

On lease/session/generation denial: one status read, at most one exact rendered
next command, then release a claimed lease and return blocked if still denied.
Digest drift is mutation-free. Preserve safe WIP; reconcile ambiguous external
intent rather than retrying creation. A completed execution grants no new lease.

### Fixed IssueOps v1 Owner Report

Return these 13 fields exactly once, in this order (bounded evidence, no secrets,
claim-token text, private reasoning, or raw transcript):

~~~text
Status: <completed | blocked>
Lifecycle: <exact lifecycle ID>
Host/model: <host / model (effort)>
Worktree/branch/final HEAD: <exact values>
Lease generation/completion: <generation + receipt or blocker>
Commits: <ordered SHA + subject>
Changed files: <exact paths>
Acceptance evidence: <AC-ID → test/command/result and artifact paths>
Verification: <every command + PASS/FAIL, explicit skips>
AI-slop clean: <removed duplication/legacy/noise or none>
Draft PR/MR: <URL or none>
Deviations: <issue/code or intent/issue mismatch + file:line or none>
Blockers: <exact state/error/next command or none>
~~~

Completed requires draft readback, all required verification PASS and completion
receipt; blocked preserves safe state without further unauthorized mutation.

Read [portable cycle evidence](references/portable-cycle-evidence.md) when recording domain, API, runtime-matrix, feedback or ready/done evidence in a cycle, and [steering and collaboration](references/steering-and-collaboration.md) before changing goals mid-run (record steering events in the ledger; reject free-form steering).


## Stop Rules

- All goals and criteria pass with artifacts/receipts and the applicable final gate
  is clean: DONE at the authorized endpoint.
- Three identical criterion failures or five goal cycles: checkpoint failed and
  surface diagnosis, not endless retries.
- Destructive commands, secret exfiltration or production writes: block at the
  safety boundary and offer a safe substitute; no hidden expansion of authority.
- Leftover QA state: clean, verify and append receipt before proceeding; never PASS.
- User /cancel: release in-progress state cleanly; do not auto-resume.

## Optional Reference

[Evidence examples](references/evidence-contract.md) and
[historical comparisons](references/comparison-context.md) add no runtime requirement.
No evaluator reference is needed to execute this skill. `quality inspect` is not
semantic skill evaluation; `issueops skill-bench` is unsupported.
