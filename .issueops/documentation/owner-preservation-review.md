# Shared skill ownership preservation review

Source: independent Astra review of the current files; task st_01a1007e.

**Recommendation:** Centralize evaluator policy, not actor-facing contracts. The upstream file was supplied at `.issueops/documentation/owner-design-review.md`; its ownership split is viable after the corrections below.

**You need:** 3/3 review tasks complete. Findings are grounded in current source, CLI help probes, and realpath checks. No edits, installs, commits, or evaluations were run.

### Actionable findings

1. **Correct the false command rejection.** The upstream report says `issueops feedback add` does not exist (`owner-design-review.md:52-54`). It does: `./bin/issueops feedback --help` exited **0**. Root dispatch additionally registers lifecycle commands (`cmd/issueops/issueopsapp/root_command_facade.go:70-73`), including feedback (`internal/contract/cli/issueops_catalog.go:124`). The skills’ examples omit actor flags; that is the defect, not command absence. Preserve recording duties and delegate authenticated recording to the invoking stage.

2. **Resolve contradictory isolation instructions before declaring one evaluator owner.** The rubric requires a **new session** when subagents are unavailable (`.issueops/operations/pioneer-skill-quality-rubric.md:88-100`); the fixture document permits the **existing main session** (`.issueops/operations/pioneer-skill-rerun-fixtures.md:23`). Choose one normative policy and make consumers refer to it. Merely linking the rubric leaves competing executable instructions.

3. **Make evaluator templates satisfy v2.** New measurements require v2, including proportionality, subcriteria, case-type weights, and discovery (`.issueops/operations/pioneer-skill-quality-rubric.md:169-222`). The rerun template still has only five scores and omits those fields (`.issueops/operations/pioneer-skill-rerun-fixtures.md:25-47`). Update active templates or explicitly label historical v1 records; centralizing prose alone does not preserve the required result contract.

4. **Bound the claim about refusal preservation.** Algorithm Optimization explicitly omits its evidence block for refused/speculative optimization (`skills/algorithm-optimization/SKILL.md:20-33`). The runtime signature checker requires all five clauses for a targeted algorithm fixture, without a refusal exception (`internal/domain/issueopsbenchmark/issueops_pioneer_checks.go:40-46`); only an absent target makes that dimension N/A (`issueops_benchmark_score.go:153-157`). A valid refusal therefore need not pass that proxy. Preserve the refusal rule and document this existing boundary; rubric ownership changes cannot fix runtime applicability.

### Preservation checklist

- [ ] Keep activation, output labels, no-change/no-input exceptions, proportionality, and stopping rules inside each independently loaded `SKILL.md`.
- [ ] Keep actor execution independent of the evaluator reference. Load the rubric **before evaluation/scoring**, not before ordinary skill use; missing evaluator material means incomplete evaluation, not blocked execution.
- [ ] Resolve optional references from the **real skill-file location**, never target-repository cwd or the unresolved host link. Both skills resolved correctly through Codex, Claude, and Omo links; all six lexical `../../.issueops/...` paths were absent. Preserve home-only linking (`AGENTS.md:120`).
- [ ] Preserve metrics baseline JSON, target card, identical-command remeasurement, and gate results (`skills/code-quality-metrics/SKILL.md:238-360`). Keep runtime snapshots in ignored/state/temporary storage; distinguish them from the committed cycle report required by `skills/issueops-slop-clean/SKILL.md:123-125`.
- [ ] Preserve cycle timing and artifacts: metrics before/after cleanup; algorithm choice, invariants, and benchmark evidence in cycle records/descriptions (`skills/code-quality-metrics/SKILL.md:365-374,473-484`; `skills/algorithm-optimization/SKILL.md:559-569`).
- [ ] Preserve evaluator prompt packets, raw artifacts, result records, calibration, holdouts, evidence grades, and gate caps. Do not treat label matching or `quality inspect` as semantic evaluation.

**Coverage boundary:** This recommendation covers evaluator ownership, the two named skills’ contracts, referenced evaluator consumers, CLI availability, and installed three-host reference resolution. It does not certify every shared skill or benchmark outcome.

**Now:** No task remains. **Next:** None.
