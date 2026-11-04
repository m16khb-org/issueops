---
name: 2026-11-04-project-doc-routing-stays-pull-based-and-lexical-the-catalog
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Project-doc routing stays pull-based and lexical; the catalog points to it

- Date: 2026-11-04
- Kind: `adr`
- Source: cli
- Summary: The SessionStart catalog now names project_docs_route and a grep of the record directories as the way to reach dated cautions and ADRs. No UserPromptSubmit hook is added and the route ranking rule is unchanged, because measured alternatives did not beat it.
- Context: A harness-engineering review (deepsense.ai, 2026-10-29) recommended a task-aware sidecar that injects context before planning. Baseline on the committed plan-citation fixture (24 plan titles, 101 cited records, scripts/route_docs_eval.py measure): recall@5 0.178, precision 0.228, 18 hits, 30-40 ms per call. Only 25 of the 101 cited records share two or more title/summary/source terms with their plan title, so a lexical ranker on those fields tops out near recall 0.25 against the 18 hits it returns now. Raw prompts of the 2026-11-04 session routed to nothing or to five unrelated records.
- Decision: Add one sentence to projectDocCatalogUsage (internal/adapter/projectdoc/catalog.go) telling the agent to pass the clarified task to project_docs_route and to grep .issueops/cautions and .issueops/adr for the subsystem it changes. Keep matchRecords as is. Rejected after measurement on the same fixture: (1) matching the whole record body at weight 0.5/0.25/0.1: recall 0.129-0.149, precision 0.116-0.134, latency about 65 ms; (2) keeping the Korean stem next to its mapped English term: recall 0.178, precision 0.188; (3) 62 more koreanTerms entries: recall 0.188 (one more hit), precision 0.202, and the words were partly chosen from the fixture titles; (4) English task text: recall 0.139-0.158, precision 0.136-0.145; (5) a UserPromptSubmit hook that runs route per prompt: it would inject the weak ranking on every turn, repeats the static-judgement approach the 2026-05-30 catalog decision rejected, and needs new adapters and a Codex trust approval (2026-08-27 and 2026-10-08 stand).
- Consequences: Every host gets the longer usage line in SessionStart and SubagentStart. A ranking change must beat 0.178/0.228 on route_docs_eval.py before it lands; lexical tuning has little room left, and a semantic ranker would need a decision on the no-model-call rule of the harness core.
