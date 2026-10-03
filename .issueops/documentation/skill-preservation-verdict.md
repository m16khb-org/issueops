# Skill compaction preservation verdict

**Current verdict: PASS after the second delta closure at the end of this file.**
The initial and first-delta findings below are retained review history, not open defects.

Reviewed 2026-10-03 against `345ae6187cd75b786985e65a6074e77f237544b1`.
**Initial review: NEEDS-FIX.** Smaller documents and passing structural validators do not establish invariant preservation.
`old` below means content at that exact revision; unqualified paths/lines mean the reviewed working tree.

## Lane verdicts

| Lane | Verdict | Skill-level result |
|---|---|---|
| Analysis | **PASS** | algorithm-optimization, code-quality-metrics, implementation-planning: no material preservation defect found. |
| Research | **NEEDS-FIX** | All four bodies preserve their inspected runtime contracts; requirements-analysis lacks its explicitly required authoring evidence (R1). |
| Execution | **NEEDS-FIX** | git-operations and sync-base pass; pr-review has conflicting effort instructions (E1); verified-execution loses an artifact schema boundary (E2). |
| Cycle | **NEEDS-FIX** | issueops and issueops-implement omit obligations behind newly optional dependencies (C1-C4); issueops-create-issue and issueops-cleanup pass their inspected stage contracts. |

These are source-based preservation/completion-evidence judgments, not measured fresh-actor task-success scores.

## Findings, ordered by consequence

### C1 — High: handoff no longer defines uncertain work as a writer

- Location: `skills/issueops/SKILL.md:51-58` and `skills/issueops-implement/SKILL.md:53-58,210-214`.
- Original obligation: old `skills/issueops/SKILL.md:48-50` required the session-choice protocol; old implement `:62-66` followed that router.
  Unchanged `skills/issueops/references/session-choice.md:145-156` explicitly treats builds, goldens, generators, formatters, fixtures and unclassified work as writers until command/cwd/output evidence proves otherwise.
  Only independent readers of immutable inputs outside shared state may continue; release requires zero pending writers.
- Current roots require writer/descendant termination but omit that classification rule while making references optional.
  A body-only actor can classify a test/build as a reader, release the lease and leave a process mutating shared state.
- Required correction: inline the conservative writer classification, reader exception and zero-pending-writer observation in both handoff-capable roots before release.

### C2 — High: mandatory delivery-observation gates became optional recipes

- Location: `skills/issueops/SKILL.md:60-65` and `skills/issueops-implement/SKILL.md:47-62`.
- Original obligation: old router `:48-50` required `session-choice.md`; its unchanged `:186-191,222-299` requires successful staged observation before the external launch and preserved attempt/lineage identity afterward.
  Its `:259-296` requires receiver PID/start-time/executable correlation and forbids manually manufacturing owner-claim evidence.
  Its `:384-389` requires event/state subscription before submission and native receipt evidence, not an idle prompt or launcher `done`.
- Current router says to record before/after but omits the failed-staging stop and receiver-correlation rule; implement omits the producer entirely.
  Neither body carries the observation object fields at reference `:231-252`, despite claiming body-only sufficiency.
- Required correction: restore the pre-launch success gate, submission subscription, native receipt/correlation and no-fabricated-claim rules inline.
  Keep the required observation fields/identity continuity locally available, or explicitly discover their schema before launch; do not make the old mandatory safety protocol optional.

### C3 — Medium: isolated router still needs a sibling to choose the base

- Location: `skills/issueops/SKILL.md:12-15,222-224`.
- Original obligation: old router `:239-241` required the base-selection order in `skills/issueops-prepare/SKILL.md:45-61`.
  That order is explicit user branch, issue-designated integration branch, project convention, observed recent merge targets, then repository default; release-flow exceptions must be explicit.
- The compacted router promises operation without siblings but retains only `../issueops-prepare/SKILL.md` for this decision.
  That link exists in this checkout but escapes a separately copied issueops directory. An executable next-command template does not supply the missing selection policy.
- Required correction: include the five-ranked decision and reporting/exception rules in the fallback body. Retain the sibling link only as optional elaboration.

### C4 — Medium: domain/live/completion evidence is absent from the router fallback

- Location: `skills/issueops/SKILL.md:171-188,208-214`.
- Original obligation: old router `:219-231` directed the actor to the current-stage references.
  Unchanged `skills/issueops/references/evidence-contract.md:7-19,35-45,49-70` requires distinct exact-mechanism/equivalent-behavior evidence, a pre-change environment matrix, feedback accountability fields and an inspectable draft completion record.
- The new body declares itself sufficient but only names these reference topics. It does not specify the matrix columns or completion-record fields.
  A body-only actor can obey its tests/live-evidence warning yet omit these required artifacts.
- Required correction: inline the applicable evidence schemas and timing: invariant/mechanism/equivalence/source; Environment/Repo-config evidence/Runtime evidence/Failure path/Remediation order; feedback classification/verification/thread reply/resolution; and completion diff/verification/labels/children/URL/thread status/cleanup/follow-ups.
  `skills/verified-execution/SKILL.md:307-332` already retains these duties; the router's no-sibling fallback must not silently waive them.

### E1 — Medium: all-level finder instructions contradict the low-effort contract

- Location: `skills/pr-review/SKILL.md:106-113`, especially “These five always apply.”
- Original obligation: old root `:40-46,108-118` restricts low to logic + boundary and applies only the generated unit's lenses.
  Current `:32-38` still promises that same two-lens low mode, but the newly inlined “all levels” section mandates logic/boundary/tests/rules/scope.
- A body-only low reviewer receives incompatible scope instructions and may perform undisclosed extra lenses.
- Required correction: make the generated effort-selected lens set authoritative; explicitly limit low to logic/boundary and qualify the five-lens default for other levels.

### E2 — Medium: goals.json loses its top-level schema

- Location: `skills/verified-execution/SKILL.md:131-148,199-208`.
- Original obligation: old `:233-267` specifies a JSON object containing `goals: [...]` and sibling `metrics: {...}`.
- Current body lists individual goal/criterion fields and five formulas, but never specifies that container or where metrics reside in goals.json.
  The listed fields permit a bare goal, bare array or differently nested checkpoint, breaking the original cross-session artifact contract.
- Required correction: restore a compact explicit goals.json schema with the `goals` array, top-level `metrics` object, original five metric keys and initial values; keep the existing criterion fields.

### R1 — Medium: requirements-analysis authoring gate has no execution evidence

- Location: `.issueops/documentation/research-contracts-changes.md`, “Limits and ownership”; `skills/requirements-analysis/SKILL.md:237-264`.
- Original obligation: old root `:94-106,248-256` requires observed RED before modification, same-fixture RED/GREEN, the full regression matrix and helper success/failure checks.
- The report explicitly says no OCR fixture runs occurred and that these duties were documented rather than executed. Metadata/link validation is not evidence for those gates.
  The obligations themselves remain in the compacted text; this is a completed-change verification gap, not a lost runtime clause.
- Required correction: supply contemporaneous RED and corresponding comparison/helper evidence if it exists.
  Otherwise rerun the bounded original/candidate fixture comparison, label any reconstructed baseline honestly, and obtain an explicit owner waiver for the missed pre-edit ordering before claiming this lane fully validated.
  Do not substitute historical fixture results or a documentation score.

## Preserved contracts checked

- **Analysis:** `algorithm-optimization/SKILL.md:14-43,48-118,130-158` keeps refusal/no-change thresholds, all five artifact labels, baseline/noise/scaling/proportional proof and concurrent verification, authenticated feedback and bounded stops.
  `code-quality-metrics/SKILL.md:14-40,124-202,210-233` keeps untracked inventory, no-input/N/A, snapshot keys, four metrics/targets, baseline-before-cleanup, identical-command gate and waivers.
  `implementation-planning/SKILL.md:12-64,91-237,239-302` keeps the routing exception, five labels, draft/clearance, full task/QA/commit template, standalone choice and cycle-specific authorization/order.
- **Research:** debugging retains six labels, reproduction/isolation/hypothesis order, five-cycle cap, minimal-fix/full-suite/regression and production-data stop (`:20-164`).
  Prompt engineering retains five labels, production versus one-shot rigor, test/adversarial cases, single-variable iteration, privacy/tool truth, artifact paths and stops (`:20-187`).
  Requirements retains ten headings, JSON keys/enums, twelve coverage gates, probe order, no-install/remote boundaries and unreviewed-material downgrade (`:24-264`).
  Web research retains four labels, fetched-source provenance, quick-lookup exception, independence/confidence, boundary-probe-before-alternatives, access stops and bounded research (`:22-220`).
- **Execution:** Git retains backup/confirmation/atomicity, bisect reset, recovery and shared-branch stops (`git-operations/SKILL.md:21-54,58-318`).
  Sync retains eight base ranks, ordered merge/rebase choice, zero-left-count no-op, explicit lease, first-conflict stop and preservation checks (`sync-base/SKILL.md:26-397`).
  PR retains candidate schema, coverage degradation, blind tracer/reproducer sequence, abstention and posting/readback gates (`pr-review/SKILL.md:121-328`), subject to E1.
  Verified execution retains five labels, proportionate mode, cleanup-before-PASS, ledger fields, nine adversarial classes, failure caps, authority and fourteen owner-report labels/order (`:26-109,111-129,168-305`), subject to E2.
- **Cycle:** router retains stage keys, lifecycle identity, actor/generation/cwd checks, holds/narrower endpoints, five benchmark labels and completion-versus-cleanup boundary (`issueops/SKILL.md:17-37,96-169,216-259`).
  Implement retains link-plan -> compatibility -> ledger -> implement, RED/GREEN failure counter, child acceptance and report-before-clean transition (`issueops-implement/SKILL.md:64-149,151-240`).
  Create-issue retains raw request, ambiguity branches, record ordering, score/label/assignee/hierarchy, reconcile and three completion labels (`issueops-create-issue/SKILL.md:23-238`).
  Cleanup retains exact consent, merged/released prerequisites, reflection-before-close-before-fresh-preview, fingerprint/process receipts, absent-worktree recovery and partial-failure output (`issueops-cleanup/SKILL.md:22-258`).

## Independent checks and limits

- Read all four lane reports and all 41 current owned Markdown files; compared against original Git content, including unchanged normative references and targeted dependency contracts.
- `quality-baseline-2026-10-03.json` names the requested revision. All 32 original owned Markdown entries matched its SHA-256 values.
  Its `checker.ok=true`, 611 checked documents and line/word metrics measure documentation structure, not actor success.
- Independently confirmed 15/15 frontmatters unchanged. Root line totals match reports: analysis 1547 -> 693; research 1482 -> 878; execution 1682 -> 1427; cycle 1155 -> 1024.
- Resolved all 48 non-fenced local Markdown links from real file locations: none missing in this checkout. Verified 45/45 installed Codex/Claude/Omo root paths resolve to the reviewed source.
  Nineteen link occurrences cross skill-directory boundaries, all in the cycle lane; therefore installed-link success does not prove isolated-copy closure. C3 identifies a required unresolved dependency, not merely an optional navigation link.
- Ran `python3 scripts/validate-skill.py` with the fifteen reviewed skill directories: 15 successes, exit 0.
  Scoped `git diff --check -- skills/<each-reviewed-skill>` exited 0. Re-read snapshot comparison found no owned Markdown changes during review.
- The fixed fourteen owner labels were checked against old `.issueops/prompt-engineering/prompts/issueops-v1-owner-execution-v1.md:262-282`; label preservation does not repair E2's separate checkpoint schema.
- `feedback add` and `status` are supported aliases, not defects. The absent skill-bench runner is not a verification option; quality inspect is not semantic evaluation.
- No fresh-actor success benchmark, OCR suite, prompt A/B, provider mutation, install or whole-repository test ran in this review.
  The cycle report's actor probes failed with HTTP 400 MissingSessionID before results; no equivalence evidence can be credited to that attempt.
  Existing contradictory examples were not automatically classified as new regressions; passing text preservation is not certification of every inherited runtime instruction.
- Only this verdict file was authored. Source, tests, configuration, lane reports and historical results were not changed.

## Delta closure review - 2026-10-03

The preceding findings and measurements are preserved as historical review evidence, not current line references.
This section supersedes their disposition after checking only the reported restorations against the same original revision.
**Current overall verdict: NEEDS-FIX.** C2-C4 remain partially open; no other unresolved material defect was found in this delta.

| Lane | Current verdict | Delta disposition |
|---|---|---|
| Analysis | **PASS** | Original preservation verdict retained; algorithm phrase compatibility restored. |
| Research | **PASS** | R1 closed by withdrawal of the changed target, not a waiver or successful OCR evaluation; named phrase fixes verified. |
| Execution | **PASS** | E1 and E2 closed; verified-execution phrase compatibility restored. |
| Cycle | **NEEDS-FIX** | C1 closed; C2-C4 restore substantial content but still omit the requirements below. |

### Closed findings

- **C1:** `skills/issueops/SKILL.md:76-99` and `skills/issueops-implement/SKILL.md:64-87` now conservatively classify uncertain work/builds/goldens/generators/formatters/fixtures as writers, retain the immutable-input reader exception, inventory work, and require observed writer/descendant termination and zero pending writers before release. This matches old `session-choice.md:145-156`; release-before-launch, scope and actor/generation fences remain in both bodies.
- **E1:** `skills/pr-review/SKILL.md:108-110` makes generated effort-selected lenses authoritative, explicitly limits low to logic/boundary, and prevents topic matches from adding lenses. This resolves the conflict with old `:40-46,108-118` and current `:32-38,94-95`.
- **E2:** `skills/verified-execution/SKILL.md:150-183` restores the top-level `goals` array and sibling `metrics` object. Parsed JSON is exactly equal to old `:237-267`, including all criterion fields, null evidence/receipt, and all five 0.0 metric values.
- **R1:** current `skills/requirements-analysis/SKILL.md` equals `git show 345ae6187cd75b786985e65a6074e77f237544b1:skills/requirements-analysis/SKILL.md` byte-for-byte. SHA-256 `47d251fce52ed5a9d0e73a13105141e59d6ff4dc4e9fd8348ed1e41e158e8a98` also matches the quality baseline. No changed authoring target remains; the historical missed RED is not retroactively satisfied. No OCR run or improvement claim is credited.

### Unresolved material preservation defects

**C2 - High, partial: receiver observation remains underspecified in both standalone roots.**
Locations: `skills/issueops/SKILL.md:87-99` and `skills/issueops-implement/SKILL.md:75-87`, "handoff observation and writer termination."
Staging-success-before-launch, pre-submission subscription, identity continuity, native receipts and no fabricated owner claim are restored.
However, preserving `processIncarnation` and comparing a PID informally does not require recording the second observation's `target.process_incarnation` and `target.process.{pid,started_at,executable}` or matching it to the claim holder's `session_process`.
Old, unchanged `skills/issueops/references/session-choice.md:259-299` requires those fields, input-only evidence when correlation is unavailable, a second producer call, and inspection of `observation.receipt.location` plus digest. The implement body still does not require that post-receipt producer call; neither body retains the audit readback.
**Required correction:** inline that receiver schema and correlation/failure branch, require post-receipt recording and audit readback, and preserve the native-turn-versus-input evidence distinction (old `:285-296`). Alternatively require discovery of the complete schema/protocol before launch; an optional recipe cannot supply these mandatory gates.

**C3 - Medium, partial: release-flow exceptions lost their explicit-user prerequisite.**
Location: `skills/issueops/SKILL.md:290-292`, "Execution ownership."
The five-ranked base precedence and reporting are restored, but "report release-flow exceptions" is weaker than old, unchanged `skills/issueops-prepare/SKILL.md:58-61`: use a base that bypasses the release flow only when the user explicitly requested it.
A body-only actor can report a departure without that authorization. Generic publication authority does not define this base-selection exception.
**Required correction:** state that release-flow bypass requires an explicit user-selected base; otherwise follow the observed release convention, and report any authorized departure.

**C4 - Medium, partial: evidence labels returned without all timing and completion duties.**
Location: `skills/issueops/SKILL.md:198-203`, "implementation and verification rules."
Domain fields, environment-matrix columns, feedback field names and eight completion-record topics are restored. The body still does not require domain evidence in the issue/plan before implementation, or an inspectable draft completion artifact before final reporting and before its remote write (old, unchanged `skills/issueops/references/evidence-contract.md:7-19,60-70`).
It also omits the feedback classification/resolution vocabularies (`:49-54`) and completion checks for source/target branch, remote-body freshness, copied/replaced labels, single-commit policy/justification, divergence and clean worktree (`:62-68`). Merely naming "labels" or "cleanup" does not preserve those checks.
**Required correction:** inline those placement/order requirements, exact feedback vocabularies and completion checks; keep source-versus-runtime evidence and distinct failure paths explicit (`:40-45`). These are mandatory body-only obligations, not optional background.

### Delta evidence and limits

- Re-read all four lane reports as claims, not proof; inspected the nine named changed/restored roots and three directly implicated original dependency documents. All 12 original contents matched their entries in `quality-baseline-2026-10-03.json`; the three dependency documents remain unchanged. Resolved 34 non-fenced local links in the scoped roots: none missing.
- Existing `internal/adapter/skillcontract/skill_contract_test.go` is unchanged from the baseline. All 24 scoped phrase checks pass across algorithm-optimization, issueops-debugging, prompt-engineering, web-research and verified-execution; the latter's sibling path is expressly supplemental (`:185-187`), not a new mandatory read.
- Ran `go test ./internal/adapter/skillcontract -run '^(TestP1PioneerCorrectnessContracts|TestKarpathySkillPinsPrivacyAndProportionalityContract|TestBernersLeeSkillPrefersHarnessWebFetchContract|TestActiveExecutionGuidanceMatchesCurrentRuntime)$' -count=1`: exit 0, package passed in 0.412s. This proves existing phrase compatibility, not task success or resolution of C2-C4.
- No corpus-wide re-review, live OCR, actor benchmark, external research, source edit or whole-repository test was performed. Supported feedback/status aliases and the absent skill-bench runner remain as previously established. Documentation quality and unmeasured task success remain separate judgments.

## Second delta closure - 2026-10-03

**Current overall preservation verdict: PASS.** This supersedes the preceding C2-C4 dispositions only; historical findings remain intact. No unresolved material preservation defect remains among these three findings.
**Lane verdicts:** Analysis **PASS**, Research **PASS**, Execution **PASS** (carried forward from the first delta, not re-reviewed); Cycle **PASS** (C1 closure retained; C2-C4 closed below).
- **C2 closed:** `skills/issueops/SKILL.md:84-111` and `skills/issueops-implement/SKILL.md:72-99` now require the second producer call, audit location/digest readback, `target.process_incarnation`, `target.process.{pid,started_at,executable}`, exact claim-holder `session_process` correlation, and input-only evidence without that correlation. First-call retry exclusion, native/input/Herdr distinctions, ambiguity types, claim-handler-only recording and generation/host/lineage fences match old `skills/issueops/references/session-choice.md:259-299`. Both identical inline blocks retain staging-success-before-launch, pre-submission subscription and attempt/lineage continuity.
- **C3 closed:** `skills/issueops/SKILL.md:324-328` preserves all five base ranks and now permits bypassing release flow only for an explicitly user-requested base, otherwise following the observed convention and reporting authorized departures; this matches old `skills/issueops-prepare/SKILL.md:45-61` without requiring that sibling.
- **C4 closed:** `skills/issueops/SKILL.md:210-237` restores pre-implementation issue/plan placement, exact-versus-equivalent distinction, matrix fields, source/runtime/network separation, distinct failure paths, feedback vocabularies and remote-body refresh before continuing. It requires branch/body/label/commit/divergence/worktree/cleanup checks before ready/done, plus the eight-field inspectable completion draft before reporting and remote write, matching old `skills/issueops/references/evidence-contract.md:7-19,35-72`. These are body obligations, not optional reference recipes.
- **Evidence:** compared original Git content directly at `345ae6187cd75b786985e65a6074e77f237544b1`; all four inspected originals match their SHA-256 entries in `quality-baseline-2026-10-03.json`. The three implicated dependency documents remain byte-identical to those originals. The cycle change report was treated as a claim, not proof.
- **Limits:** this second pass covers only the previously open C2-C4 corrections. No corpus re-review, live OCR, actor benchmark or whole-repository tests; no new task-success claim. Only this verdict was edited.
