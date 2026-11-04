# Workflow details

Optional detail for the numbered stages in `../SKILL.md`. The body owns the stage
order, eligibility stops, posting thresholds, and the verdict rules.

## Units and packs (stage 0)

`workflow_args.json` already contains the finder units, the checkout path, the candidate caps,
the hunk ranges and prior lessons for the prescreen — never hand-build it. A **unit** is one
lens bundle × one shard of the changed files (bundles: behavior = logic/boundary/data/async,
contract = security/contract/rules, intent = tests/scope/intent); each unit has one
self-contained pack (`pack/<unit>.md`: that slice's cumulative diff, the definitions and
one-hop neighbours of the symbols it defines, matching rules, threads and prior lessons). One
finder applies its bundle's lenses separately over one pack read. Shards keep
packs under ~150 KB of diff; `defs.md` and `hunks/<file>.patch` serve skeptics.

## Remember refutations (stage 4b, team memory)

```bash
python3 <skill>/scripts/record_refuted.py --result <out_dir>/workflow-result.json --context <out_dir>/context.json
```

Appends evidence-backed refutations (skeptic confidence ≥ 80) to `<repo>/.issueops/pr-review/refuted.jsonl`
— commit it with the repo. The next run's prescreen drops a same-file candidate whose title/what overlap
≥ 0.5 with a recorded refutation; `security`/`data` candidates are never suppressed. Prescreen kills are
not recorded (they carry no evidence).

## Re-review of a moved head (stage 4c)

Run preflight with `--incremental`: when `<out_dir>` already holds `workflow-result.json` and
`context.prev.json` from the previous head, only units whose files changed since that head are
re-inspected and findings on untouched files are carried (`carried_from`). Findings on changed
files are dropped and re-found or not. `post_review.py` still validates every line against the
new diff.

## xhigh sweep pass (stage 2, step 4)

After every unit, take one more pass over the diff as a fresh reviewer holding the current
list, looking **only** for defects not already on it — moved/extracted code that dropped a
guard, setup/teardown asymmetry in tests, config defaults flipped. Add nothing you already
have; return nothing rather than padding.
