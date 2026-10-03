# Research skill contract changes

This is the initial worker snapshot. The lead restored `requirements-analysis/SKILL.md`
byte-for-byte to the baseline after R1 found missing pre-edit OCR authoring evidence.
No formal improvement/evaluation claim is made for that unchanged skill. Final counts
and subsequent preservation corrections are recorded in `quality-audit-2026-10-03.md`
and `quality-after-2026-10-03.json`, not the initial table below.

## Decision and scope

Keep executable actor contracts in each `SKILL.md`; extract only examples/background.
The alternative, mandatory shared/evaluator reads, breaks body-only and copied-skill use.
Read both owner reviews first, all four original bodies, their relevant references,
the IssueOps router, and the requirements report validator before editing.
Only four entrypoints, three new same-skill Markdown references, and this report changed.
Frontmatter `name`/`description`, scripts, tests, agent YAML, global docs, and historical results were not edited.

## Size and movement

Counts include frontmatter; before is the working-tree content captured before editing.

| Skill | Before | After | Movement |
|---|---:|---:|---|
| issueops-debugging | 378 | 173 | Stack commands, cross-language patterns, failure lookup, and lesson example → `references/debugging-examples.md` (80 lines). |
| prompt-engineering | 465 | 198 | Software 2.0 background and five reusable patterns → `references/prompt-patterns.md` (122 lines). |
| requirements-analysis | 265 | 279 | No outward extraction: report/schema, probe decisions, and regression obligations summarized into the root from existing references; dated evidence stays untouched. |
| web-research | 374 | 228 | Query recipes, public-platform command table, and metadata examples → `references/retrieval-examples.md` (55 lines); existing report template unchanged. |

Entrypoints total **1,482 → 878 lines** (604 fewer, 40.8%).
Including the 257 new reference lines, the owned Markdown content decreases by 347 lines, excluding this report.
Requirements Analysis intentionally exceeds the 250-line soft target: its exact headings,
JSON fields, confidence/status vocabulary, coverage gates, and authoring obligations
must remain available without a reference bundle. No other entrypoint exceeds 250.

## Preserved critical contracts

### IssueOps Debugging — `SKILL.md:8-168`

- Diagnostic-only scope; reproduce before diagnosis; exact input/command/stdout/stderr/exit/signature/repeatability; unavailable reproduction is a recorded blocker, not confirmed cause.
- All six evidence labels unchanged: Reproduction, Failure signature, Root cause hypothesis, Isolation, Minimal fix boundary, Verification. Trivial syntax/import/path failures retain the short-form exception.
- Seven-step order; `lint_diagnose` hypothesis-only and all skip/redaction cases; bisect, halve, ten-run trace divergence, and clean-target golden diff strategies; hermetic snapshot inputs and user-work protection.
- Falsifiable hypothesis before fixes; five-cycle cap/differential diagnosis; 20-line isolation/fix boundary; architecture escalation; reproduction/full-suite/regression verification; durable lessons; production data/secrets stop.
- Implement/feedback timing; findings recorded by owner; `contract_change` before plan update/review and implementation; Verified Execution receives diagnosis for its fix/channel QA.

### Prompt Engineering — `SKILL.md:8-193`

- Production versus one-shot activation; five phases; success criteria before drafting; at least five production cases (three happy/two edge), expected outputs/regressions, versus 1–2 sanity checks.
- All five labels unchanged: Input/output contract, Test suite, Adversarial cases, One-variable iteration, Privacy/tool truth; lightweight evidence retains contract/privacy/tool checks.
- Constraint-top/format-bottom layout, context budget, selective techniques, model calibration, all adversarial categories, bounded rationale instead of hidden reasoning, verified host names/schemas/arguments and explicit action permissions.
- Diagnose before changing; exactly one variable per iteration; baseline/A-B accuracy/latency/token-cost/failure measurements; versioned prompt/test-suite/result paths; generated-code metrics are not prompt metrics.
- Production publication requires tests/adversarial/metrics; one-shot storage/ceremony exception; three unimproved iterations stop; safety/performance trade-off escalation; changed requirements return to Specify; missing checks are not passes.

### Requirements Analysis — `SKILL.md:8-279`

- Required absolute source; missing package destination asks once without writes; optional inputs/defaults; source/comparison/fixtures read-only; temporary artifacts; separate install/reconfigure/upload/remote/commit/push authorization; no download/cache deletion by default.
- Untrusted document instructions, masking, private-reasoning boundary, four independent evidence channels, original conflicts, actual schema discovery, current-host probes and one-time direct-image fallback.
- OCR functionality versus directly evidenced backend identity; source hashes/format/bytes/page count; native structure/warnings; all-page rendering/parity; image-only OCR and repeat; targeted crops/table context; actual visual review.
- All seven fact statuses, five comparison statuses, three page statuses/verdicts; product/backend/payment/privacy/flow omissions; required comparison access; all ten report sections and JSON keys now explicit in the body.
- All twelve coverage gates, zero unreviewed material, unchanged source and successful validator before high coverage; validator does not prove visual truth; blockers/downgrades and no unsupported certainty.
- RED before workflow edits; same-fixture GREEN; independent visual ground truth; one-rule iterations; full fixture matrix, helper success/failure paths, metrics, metadata/YAML consistency; no new sessions/delegation without permission.
- Analysis does not start an IssueOps cycle or authorize implementation; those require explicit user direction.

### Web Research — `SKILL.md:8-228`

- Research-only file scope; actual fetched/read URL/date evidence; full research versus one-fact inline lookup exception; 2–4 angles, 3–5 sources per delegate, host-capability fallback, anti-duplication, main-agent synthesis.
- All four labels unchanged: Source fan-out, Source index, Claim verification, Access boundary; harness response evidence fields retained.
- Public routes first; direct boundary probe before parallel alternatives; access-control stops override stale “escalate” wording; no impersonation/cookie/referrer bypass; sidecars only after clean probe/primary failure, with provenance.
- Ordinary SPA rendering only; public network discovery/pagination; one Retry-After retry for plain rate limits; response/content minimums, metadata-only salvage, geo limits, no silent/global installs.
- Independent-source counting/authority, confirmed/single-sourced/disputed/unverified labels, critical-claim refutation, confidence thresholds, all report fields, unresolved evidence, three-round checkpoint and exhaustion stops.
- Research precedes grill; feedback retains slug/source/confirmed counts; owner links the report under External Research; research alone does not authorize committing it.

### Shared execution boundaries

Each body retains ordinary activation, exceptions, output obligations, and stop conditions without sibling/evaluator reads.
Same-skill links resolve from the real skill location, not cwd or unresolved home symlinks.
Cycle recording belongs to the routed stage; preserve exact lifecycle ID, native actor,
generation, canonical cwd, gate/artifact order, blocked/mismatch stops, and existing approval scope.
No implicit publication/commit/push/cleanup or extra approval for already-authorized work.
Destructive cleanup still needs target/fingerprint preview and separate confirmation.
Valid `issueops feedback add` / `issueops status` aliases are not rejected; actor flags are not invented.
`issueops skill-bench` remains unsupported; `quality inspect` is not advertised as semantic evaluation.

## Verification commands and results

Executed from `/Users/m16khb/Workspace/issueops`:

```bash
python3 scripts/validate-skill.py skills/issueops-debugging skills/prompt-engineering skills/requirements-analysis skills/web-research
git diff --check -- skills/issueops-debugging skills/prompt-engineering skills/requirements-analysis skills/web-research
git diff --numstat -- skills/issueops-debugging skills/prompt-engineering skills/requirements-analysis skills/web-research
git diff --name-only -- skills/issueops-debugging skills/prompt-engineering skills/requirements-analysis skills/web-research
git status --short
wc -l .issueops/documentation/research-contracts-changes.md
```

Validator: **4/4 “ok: Skill is valid!”, exit 0**. Scoped whitespace check: **exit 0**.
Report budget: **113 lines**, below the 120-line limit.
Metadata compared byte-for-byte against captured originals: **4/4 unchanged**.
Local links: **10/10 resolve across all 11 owned Markdown files**, all contained within their own skill.
Installed links: **12/12** realpaths match source (Claude, Codex, Omo).
Exact JS link-check operations used `read(file)`, stripped fenced templates with
`/^```[^\n]*\n[\s\S]*?^```\s*$/gm`, matched `/\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g`,
skipped `/^(?:https?:|mailto:|#)/`, then called `fs.stat(path.resolve(path.dirname(await fs.realpath(file)), dest.split('#')[0]))`.
Copy containment checked `path.relative(skillRoot, target).startsWith('..')` was false.
Home checks called `fs.realpath` for each skill's `SKILL.md` under
`/Users/m16khb/{.claude/skills,.codex/skills,.omo/agent/skills}`.
The first probe incorrectly treated fenced `{URL}` as a path and guessed `.omo/skills`;
both probe errors were corrected before the passing checks; no skill link was broken.
The three labeled evidence blocks were compared to their originals and are identical.
The requirements headings/fields/gates were checked against its existing schema and validator.

## Limits and ownership

Preservation is a static body-only contract audit, not a semantic actor score or live OCR benchmark.
No OCR fixture runs, prompt A/B runs, fresh agents, prose-pinning tests, or self-verify suite ran:
the current request bounds writes to Markdown and asks for existing skill validation/link checks.
The authoring RED/full-regression duties remain documented, not falsely reported as executed.
Other workers changed global docs and sibling skills during this run; those changes were left untouched.
This task's apply_patch write log is limited to the eight Markdown files listed above.
