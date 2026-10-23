# Structured Steering and Collaboration

Moved from SKILL.md "Structured Steering and Collaboration". Read before changing goals mid-run or when optional collaborator skills apply.

Reject free-form steering in place of structured, evidence-backed state changes.
Record every steering event in the ledger:

Reject free-form steering in place of structured, evidence-backed state changes.
Record every steering event in the ledger:

| Kind | Required fields |
|---|---|
| add_subgoal (real blocker) | --title, --objective, --evidence, --rationale |
| split_subgoal (too large) | --goal-id, --children, --evidence, --rationale |
| reorder_pending (dependency) | --order (IDs), --evidence |
| revise_criterion (no observable PASS) | --goal-id, --criterion-id, --scenario, --evidence |
| mark_blocked_superseded (replacement) | --goal-id, --replacements, --evidence |

These are record fields, not invented executable CLI flags. Optional collaborators:
planning supplies criteria; debugging diagnoses; algorithm work supplies redesign
and benchmarks; DB work supplies before/after EXPLAIN ANALYZE; research supplies
reviewed reports; code-quality metrics add SNR/entropy/redundancy; self-verify
health scores feed evidence coverage; repeated failures yield recorded lessons
(with self-augment if available). Authorized code/evidence commits stay atomic.
Use current file/patch/search/shell/async tools with explicit cwd on Codex, Claude
Code, Omo, omp, and opencode; never name unavailable host tools as executable.
