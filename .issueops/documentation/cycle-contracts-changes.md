# Cycle skill contract preservation

Initial worker snapshot. The lead subsequently restored the C1-C4 root obligations
found by independent review. Final measurements and closure evidence are recorded
in `quality-audit-2026-10-03.md` and `quality-after-2026-10-03.json`.

## Decision and scope

Compacted the four entrypoints as actor programs, not as evaluator summaries.
Kept authorization, outputs, evidence, exceptional paths and stopping rules in each root.
Rejected moving mandatory rules into a shared file: a body-only actor receives no reference bundle.
Only four owned SKILL.md files, two same-skill example files and this report were edited.
Existing unrelated workspace changes, global docs, code, tests and historical evaluations were left alone.
Frontmatter name/description bytes are unchanged in all four roots.

## Root sizes and deliberate soft-target exceptions

| Skill | Lines before → after | Bytes before → after | Why retain more than 250 lines |
|---|---:|---:|---|
| issueops | 281 → 259 | 18,292 → 18,328 | Full stage labels, handoff authority and five benchmark fields remain local. |
| issueops-implement | 274 → 251 | 17,420 → 17,182 | Ordered entry, shared failure counter and child/cancellation gates remain local. |
| issueops-create-issue | 290 → 256 | 16,593 → 15,339 | Recording order, publication identity and child reconciliation remain local. |
| issueops-cleanup | 310 → 258 | 15,116 → 12,731 | Destructive confirmation, process receipts and partial-failure recovery remain local. |

Total root lines: 1,155 → 1,024; bytes: 67,421 → 63,580.
Router bytes rose by 36 because previously external actor requirements are now explicit.
These are structural measurements, not a claim of faster or more accurate model execution.

## Moved and consolidated sections

- `issueops`: folded duplicate ten-stage navigation into the authoritative stage table plus a short collaboration map; consolidated repeated fencing, phase and session explanations. No reference was changed.
- `issueops-implement`: moved illustrative bad examples to `references/implementation-examples.md`; replaced the redundant flow diagram with its ordered text sequence.
- Unique rules formerly found in implement examples (schema estimates/waiver, no early cleanup/review, catalog-based command verification) remain explicit in the root.
- `issueops-create-issue`: moved template-source background, bad-input examples and optional operation-ID generation example to `references/issue-writing.md`.
- Unique create-example rules (closed parent refusal, executable verification, non-waived plan-prep) remain in the root.
- `issueops-cleanup`: consolidated repeated prose in place; moved no mandatory or optional section.

## Preserved critical contracts

### issueops

- Exact ID selection/persistence; none/draft/new-cycle, ambiguity, blocked/takeover/claim routing; unchanged stage keys and labels; no unsolicited cycle cleanup.
- Source preparation versus canonical implementation; direct preparation; Orca-ready → compatible live Herdr → current; latest user choice/hold and narrower endpoint win.
- Decision/readback, writer/descendant quiescence, sealed handoff evidence, release-before-launch, one receiver, digest validation, no recursive handoff or ambiguous relaunch.
- Native whoami flags, ID/generation/actor/cwd checks before mutation, wrong-root repair, reconcile rather than retry; hooks remain context-only.
- Reuse, compatibility/performance/side effects, RED→GREEN→SURFACE→CLEAN, API/live checks, cleanup recheck, docs/schema evidence before review/publication, real no-change evidence.
- Authenticated stage-owned feedback, body-of-record update, strict readiness, approved commit/push scope, separate target/fingerprint cleanup consent.
- Unchanged labels: `Durable state record`, `Phase routing`, `Flow evidence`, `Hook boundary`, `Cleanup/readiness evidence`; independent read-only judge, deterministic-first and strict JSON decoding remain.
- Only unresolved material ambiguity/authority needs user input; two unchanged recovery attempts stop; no infinite holder polling.

### issueops-implement

- Stage/claim routing; exact sealed claim once; active(self), canonical branch/HEAD/dirty checks; digest-aware handoff and native identity.
- Pre-seal sync-base preview/apply fingerprint, clean worktree, conflict reporting, no rebase, base-diff impact recorded.
- Four plan sections → link-plan (skip only already materialized linkage) → compatibility/rollback/verification → gates ledger → implement.
- Focused failing test; unexpectedly green RED stops; two GREEN failures trigger diagnosis and the same counter's third failure stops.
- Real commands/results and RED/GREEN ledger; tracked plan/intent/spec/review copies in first commit; reuse/scope/performance/side-effect rules; API and `## UI 판단` evidence.
- Recovery only by returned commands, no guessed flags/fingerprints or filesystem-based mutation retry.
- Implement plus three approved/waived reviews plus planned bounded delegation; own child identity/worktree/lease, no parent-record writes or invented amend.
- Drift requires new child or revised plan/fresh review; cancellation stops dispatch and writers/descendants; late callbacks cannot write or record pass.
- Acceptance rubric and accept/reject/drop semantics; all children accepted/dropped, focused evidence and in-worktree relative report before ai-slop-clean.
- No stage-4 commit, cleanup/review record or execution complete; continue only to the original authorized endpoint.

### issueops-create-issue

- Source-only start; explicit separate `--new`, never with `--branch`; preserve returned ID; lost response means list before retry.
- Raw request unchanged; document constraints/ambiguities extracted separately; code and conditional web evidence; resolved/deferred/blocking and one-question interview.
- Problem, urgency, observable success, non-goals, domain terms, surfaces and blocking decisions before publication; confirmed draft feeds interpreted intent.
- Start → intent → domain → split decision → score/label decision → four plan-prep evidence fields → grill → publication/readback → link only if missing.
- Template mapping, no-split default and `Large Issue Breakdown Gate`, independent delivery/rollback/ownership split criteria, native parent/child hierarchy and `[p]`/`[s]` dependencies/waves.
- Parent URL/umbrella branch gates, active parent, parent body update/readback, no raw-provider bypass or comments-only completion.
- Idempotent child operation IDs, unresolved-operation fence, known URL/recovery evidence, holder/generation-bound reconciliation without creating another child.
- Korean/body/privacy/label/concrete-assignee gates; preview critical/warning handling, identical confirm, live readback; no extra approval for already-authorized publication.
- Draft-only stop; issue-only output labels `ISSUEOPS_ID`, `issue`, `다음 단계`; full-cycle continuation retains exact ID.

### issueops-cleanup

- Exact record-backed done/released/merged cycle, source cwd, verified closed children; clean matching branch/HEAD/OIDs and provider evidence.
- Active task/pending intent/requester/source-root blockers; occupant/terminal inventory is consent evidence, not a reason for manual killing.
- Separate remote-branch choice, keep flag/audit/unreadable state; prepared-base equality, observed automatic-retarget exception and recorded deliberate retarget.
- Only completion_reflected/issue_closed may remain before confirmation; exact target/OID/process list and latest explicit consent, even after generic cleanup request.
- Reflect team progress (format/privacy/2,000-character gate) → verified close → fresh fingerprint/live checks → exact typed apply.
- Handle/PTY receipt → HUP/TERM/KILL/re-observation → Orca ownership → worktree → branch CAS → audit/record deletion; failure retains record.
- Absent-worktree recovery avoids dead cwd, proves prior removal and confirmed branch OID/absence; changed targets require reconfirmation.
- Final ok/record_deleted/worktree_removed/branch_deleted, stopped-process fields, local absence, provider closed readback, remote branch untouched; partial failure reports failed_step/next_command.

## Isolated-use boundary

All roots state the actor contract locally; sibling/evaluator absence does not trigger installation.
Installed specialist skills remain usable. Where fluent-korean is absent, the root now requires a direct Korean quality pass instead of an impossible sibling invocation; privacy/readability/confirmation gates are not waived.
Optional same-skill links resolve from real SKILL.md paths; copied-directory dependencies were reviewed in memory, without writing outside the authorized scope.
`feedback add`/`status` remain supported aliases; authenticated recording stays with the stage. No skill-bench implementation or semantic quality-inspect runner is claimed.

## Exact verification and limits

Commands ran from `/Users/m16khb/Workspace/issueops`:
```bash
python3 scripts/validate-skill.py skills/issueops skills/issueops-implement skills/issueops-create-issue skills/issueops-cleanup
git diff --check -- skills/issueops skills/issueops-implement skills/issueops-create-issue skills/issueops-cleanup
git diff --name-only -- skills/issueops skills/issueops-implement skills/issueops-create-issue skills/issueops-cleanup
git ls-files --others --exclude-standard -- skills/issueops skills/issueops-implement skills/issueops-create-issue skills/issueops-cleanup
```
Results: validator exit 0, four `ok: Skill is valid!`; diff check exit 0; four tracked roots plus two new references.
JS eval used `Bun.Glob('**/*.md').scan({cwd: base+'/skills/'+n, absolute:true})`, `read(p)`, `/\[[^\]]*\]\(([^)]+)\)/g`, and `fs.access(path.resolve(path.dirname(await fs.realpath(p)), target))`: 25 local links across 13 Markdown files, zero missing.
`fs.realpath('/Users/m16khb/'+host+'/'+n+'/SKILL.md')` for hosts `.codex/skills`, `.claude/skills`, `.omo/agent/skills` and all four names: 12/12 resolve to the owned source roots. `.agents/skills` candidates were absent; the actual Codex path was then verified.
Full before/after root review checked the contracts above, metadata equality and line/byte counts.
Fresh body-only actor probes (five hypothetical cases per root via `completion`, model `smol`, only root as system) failed before results: HTTP 400 `MissingSessionID`, missing `x-opencode-session`.
No semantic benchmark pass is claimed. Copied/body-only sufficiency is a source-level audit, not successful model/runtime evaluation.
No prose-pinning tests, whole self-verify, installation, remote mutation, commit or historical-result edit was performed.
