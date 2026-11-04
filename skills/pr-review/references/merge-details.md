# Merge details

Optional detail for stage 3 (Merge) in `../SKILL.md`. The body owns the
`findings.json` source rules, the thread-drop rule, the verdict, the Korean-prose
and polish requirements, and the posting bars.

- The workflow's `partial` list (refuted candidates where a skeptic still stood at ≥ 70)
  becomes `open_questions`: one line each — the residual risk in the standing skeptic's words,
  the location, and what the author should confirm. Inline levels have no `partial`; put the
  hops you could not complete there instead.
- `verified_ok` entries stay as `{concern, why_ok, loc, thread}` objects; pick at most 8,
  preferring ones that contradict an existing bot thread (`thread` set).
- Re-review of a moved head: `context.json → prior_review_threads` lists earlier threads
  this skill posted (either marker); resolved ones go to `verified_ok` as "이전 지적 Fn 해결 확인", unresolved ones are
  referenced, not re-posted.
- `verified_ok` = the risky-looking things inspectors traced and cleared, each with evidence.
- `rule_candidates` = each refutation that hinged on a project fact, as one Korean sentence
  naming the fact and the proving file.

## Korean polish pass

The polish pass covers every Korean prose field the agents produced — `summary`,
`verified_ok`, `open_questions`, `rule_candidates`, and each finding's
`title`/`what`/`why`/`how`. Write complete, direct Korean sentences, remove
filler/translationese, keep terminology stable, and preserve facts, evidence lines,
identifiers, and commands exactly.
