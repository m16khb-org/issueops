# Execution Contract Refactor

Initial worker snapshot. The lead subsequently corrected E1/E2 and contract-check
compatibility. Final measurements and closure evidence are recorded in
`quality-audit-2026-10-03.md` and `quality-after-2026-10-03.json`.

## Decision and Scope

Applied the existing owner-design-review and owner-preservation-review findings.
Actor instructions remain in each SKILL.md; evaluator material and sibling installs are not execution prerequisites.
Only Markdown in the four assigned skill directories and this report was written.
Frontmatter name/description remain byte-identical. No code, tests, global docs, or historical evaluation files were edited.

## Size and Moves

| Skill | Before → after lines | Consolidation / moved material | 250-line exception |
|---|---:|---|---|
| git-operations | 376 → 321 | Identity, repeated safety/commit rules and relationship tables consolidated; existing rebase/bisect walkthroughs made optional | Six operation protocols, recovery ladder and confirmation gates stay inline |
| sync-base | 445 → 403 | Decision rationale and recorded examples moved to `references/sync-rationale.md`; repeated prohibition list folded into gates/stops | Eight evidence ranks, decision table, merge/rebase invariants and explicit lease/recovery commands stay inline |
| pr-review | 344 → 331 | Repeated rationale/adapter explanation shortened; finder schema and max verdict promoted from references; references scoped to the root's effort/placement rules | Five effort levels, coverage/abstention contracts, recovery, schemas and publication safeguards stay inline |
| verified-execution | 517 → 372 | Repeated execution/critical-rules/host tables consolidated; comparison background moved to `references/comparison-context.md`; local evidence examples aligned with proportionality | Cycle authority/startup, ordered gates, portable evidence and exact 14-field report stay inline |

Total bodies: 1,682 → 1,427 lines (255 fewer). All four intentionally exceed the soft target; no mandatory protocol was moved to satisfy it.
New references are optional background, not new measurements. Existing optional Markdown references were read before modification.

## Preserved Actor Constraints

### Git Operations

- Activation covers rebase, bisect, conflicts, history/reflog, cherry-pick and worktrees; basic commit/base-sync routing remains conditional on available siblings.
- Exact labels remain: `Git state proof`, `Recovery path`, `Destructive confirmation gate`, `Atomic scope`, `Force-with-lease rule`.
- Read-only archaeology needs no backup ceremony; missing target/evidence is clarified rather than invented.
- Status/diff before and verification afterward; local-first/fetch-before-push/diff-before-merge; independently buildable, testable, revertible one-intent commits with reasons.
- Verified backup before rewrite; exact-command confirmation and discard scope before hard reset/clean/skip; shared-branch restrictions; no raw force or pruning during recovery.
- Rebase clean-tree/base/history checks and recovery ladder; abort after more than three repeated conflicts; bisect known-good/bad, automated read-only fixed-argv test, evidence/log and reset; flaky-test exception.
- Intent-based conflict resolution and structural/user decisions; stage meanings; cherry-pick provenance/mainline/scope checks; worktree inventory/main-worktree/shared-ref safety.
- Lost-data and decision stops remain. Cycle branch-tip updates and bisect feedback go through the authenticated owner, never an implicit commit or cleanup.

### Sync Base

- Active-cycle routing, protected-branch confirmation, optional Orca, no automatic conflict resolution, and command-evidenced reporting remain.
- All eight ranked base sources, absent-config exit 1, legacy parent/backup compatibility, SHA-only IssueOps base ownership, divergence/tie handling, weak-default disclosure and confirmation-before-recording remain.
- Fresh remote preference; already-current zero-left-count stop; ordered mode table, failed-check merge default, absent-PR exception and explicit rebase override confirmation remain.
- Clean-tree/in-progress/base validity checks; verified backup and pre-sync remote SHA; old divergence recovery, fresh-clone ambiguity, plain `--no-fork-point` versus `--onto` remain.
- First-conflict stop, semantic side labels, three choices, replayed commit identity, more-than-three conflict abort; merge ancestry/content and rebase range/count/content invariants remain.
- Push confirmation includes exact context/command; merge uses no force, rebase uses explicit expected-SHA lease; rejected pushes do not permit blind retry.
- Exact-command reset confirmation, legacy backup lookup, no same-run backup deletion, and verification/no-change/failure/optional-push stop outcomes remain.

### PR Review

- Missing-target and eligibility stops, large-review narrowing, provider detection, safe checkout reuse and cleanup ownership remain.
- Low/medium/high/xhigh/max lenses, caps, gate choices, xhigh sweep, max-only agents and honest level disclosure remain.
- Opened definitions, upstream/downstream hops, cumulative-diff/newly-reachable scope and concrete scenarios remain; deterministic gate measurements alone are hop-exempt.
- Finder fields, severity/category enums, exact-once file coverage, missing-hop cap, suggestion verification, verified_ok and rule constraints are in the body.
- Max keeps deterministic prescreen, security/data suppression exemption, blind tracer before reproducer, evidence-required refutation/severity adjustment, unavailable-verdict abstention, minimum-confidence and partial-question rules.
- Pinned Omo adapter/model/no-fallback, budgets/permissions, diagnostic/coverage degradation, failed-unit and separate abstention recovery remain; missing tools are not clean results.
- Moderator fields, gate metadata, prior-thread handling, ≤8 verified_ok items, verdict rules, Korean polish and secret protection remain.
- Dry run before authorized posting; severity-weighted placement, summary demotion, pre-existing/minor table-only handling, moved-head/closed/duplicate refusal and readback remain.
- Evidence-backed refutation memory, incremental re-review, throwaway cleanup, no thread resolution/alternate approval/merge and no padded findings remain.

### Verified Execution

- Exact labels remain: `Success criteria`, `Evidence artifact`, `Cleanup receipt`, `Verification mode`, `Skipped checks`.
- Proportionate low-risk CLI/one-line-ledger/reviewer-skip exceptions remain; full-mode channels, real nonempty complete artifacts, cleanup receipts and honest skips are mandatory.
- State fallback, goal/criterion/ledger/gate fields, nine adversarial classes, baseline/RED/GREEN, direct implementation, worker re-verification, twelve delegation patterns and no nesting remain.
- Five operational metrics, comparative targets, structured steering, three-identical-failure/five-cycle bounds, cancellation and safety stops remain.
- Cycle plan identity/scope, sealed digests, native actor/generation/process receipt, canonical-only mutation and observation-only source checkout remain; report-only runs cannot widen packet verification.
- Separate raw startup/inventory reads, exact dispatch identity, ambiguity-as-blocker, shell/secret precautions, current process authority, no injected steering and user-controlled host/model prompts remain.
- Prepare preview/identical confirmation, no post-mutation fallback, replacement preview→revoke→finalize-preview→finalize, sole-writer/quiescence and reconciliation remain.
- Ordered implement→clean→docs→verify→pr→completion, report/fingerprint readiness, docs/schema evidence, current reviewer routing, bounded cycle review, authorization and resealing remain.
- Canonical regular-file committed report, clean/synced HEAD, explicit draft metadata, readback, exact completion receipt/release and separate post-merge cleanup remain.
- All 14 fixed owner-report field names/order are now inline, as are domain/API/live-evidence/feedback-accountability/completion-hygiene obligations formerly delegated to a sibling reference.
- Standalone binding-review gate remains risk-calibrated; cycle gates are not duplicated. No-test-run output, unfinished sessions, version-skew or prompt-only LLM evidence never count as PASS.

## CLI Truth and Portability

`issueops feedback add` and `issueops status` remain supported; recording duties route to the owning stage with whoami-derived record/claim actor flags, exact ID/generation/actor/cwd.
`issueops skill-bench` is explicitly unsupported; `quality inspect` is not semantic evaluation. Valid Git/provider/adapter commands were not blindly replaced.
Manual body-only walkthrough covered missing inputs, read-only/no-change cases, destructive pressure, inline/max review, low-risk execution and report-only/cycle completion. No required instruction depends on reading an optional reference.
PR execution still needs its own executable assets/provider access; their absence is a truthful blocker, not a requirement to install a sibling.

## Exact Verification and Results

Executed from `/Users/m16khb/Workspace/issueops`:

```text
python3 scripts/validate-skill.py skills/git-operations       → exit 0; Skill is valid!
python3 scripts/validate-skill.py skills/sync-base            → exit 0; Skill is valid!
python3 scripts/validate-skill.py skills/pr-review            → exit 0; Skill is valid!
python3 scripts/validate-skill.py skills/verified-execution   → exit 0; Skill is valid!
git diff --check -- skills/git-operations skills/sync-base skills/pr-review skills/verified-execution → exit 0; no output
wc -l skills/git-operations/SKILL.md skills/sync-base/SKILL.md skills/pr-review/SKILL.md skills/verified-execution/SKILL.md → 321, 403, 331, 372; total 1427
```

Read-only JS eval checks used `tool.read` for every owned Markdown file and these exact operations:

```js
const fs = await import('node:fs/promises'), pathmod = await import('node:path');
// markdownPaths: Bun.Glob('**/*.md').scan({cwd:`skills/${n}`}) for each owned skill.
// afterTexts: Map(path, (await tool.read({path})).text); beforeBodies: pre-edit reads.
for (const [p,text] of afterTexts) {
  const source = await fs.realpath(p);
  for (const m of text.matchAll(/\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)/g)) {
    const target=m[1].split('#')[0]; if(!target||/^[a-z]+:/i.test(target)) continue;
    await fs.access(pathmod.resolve(pathmod.dirname(source),target));
  }
}
// front=x=>x.match(/^---\n[\s\S]*?\n---/)[0]; compare before/after per skill.
// Hosts: fs.realpath(`/Users/m16khb/${host}/${n}/SKILL.md`), then fs.access of each relative target.
// host ∈ ['.codex/skills','.claude/skills','.omo/agent/skills']; n ∈ the four owned names.
// Copy model: pathmod.relative(skillRoot,pathmod.normalize(pathmod.join(pathmod.dirname(file),target)));
// require no '..' escape and afterTexts.has(pathmod.join(skillRoot,relative)).
```

Results: 7/7 local links exist and stay within their skill; 12/12 host paths resolve to this source and all their links pass; 7/7 isolated-copy path checks pass; 4/4 frontmatters unchanged.
Report checks: `wc -l .issueops/documentation/execution-contracts-changes.md` initially returned 113 (before this result line); `git diff --no-index --check /dev/null .issueops/documentation/execution-contracts-changes.md` returned 1 with empty stdout/stderr when comparing the new file to `/dev/null`, with no whitespace diagnostic.
Scope probes: `git status --porcelain=v1 -uall` and `git diff --binary`, compared with initial resident snapshots, found concurrent changes to `AGENTS.md` and `.issueops/TECH_STACK.md`. Neither was written by this task; no unrelated changes were reverted.
Uncertainty: metadata/link checks and manual contract review do not measure fresh-actor semantic equivalence. No semantic benchmark, prose-pinning tests, install, or whole self-verify suite was run. Copy checks model relative paths without writing a copied tree; host-symlink checks used actual installed paths.
