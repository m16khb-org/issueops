# Max fan-out: Omo adapter and recovery

Optional detail for the `max` fan-out stage in `../SKILL.md`. The body owns the
level choice, the workflow entry points, the verdict contract, and the rule that
failed children, incomplete coverage, or abstention are never a clean review.
Read this file before running the Omo adapter or recovering a failed max run.

## Omo adapter

The Omo adapter pins `zai/glm-5.3-flash`, checks `--no-model-fallback`, and
sets `SENPI_NO_FALLBACK=1`; never silently switch providers. It owns rate-limit
backoff: 429 is not a format error/provider switch. `omo-flash` uses 24 finder
turns, 18 skeptic turns, cap floor 40, concurrency 10, high thinking;
`standard` uses Workflow budgets. Finder/tracer permissions are read-only;
only the reproducer may create throwaway tests. The adapter supplies temporary
read-only inputs and schema-constrained `submit_pr_review_*` output, enforces
budgets, recovers validated submissions from empty final replies, and retries
format failures with original input and only the submit tool enabled.
Remove the temporary input directory after all phases; retain diagnostic artifacts.
Retain raw stdout/stderr, exit, timeout, denial, parse/schema diagnostics and
`failure_counts` (`parse_failure`, `schema_failure`, `timeout`, `rate_limited`,
`low_confidence_abstain`, `coverage_gap`). Missing/duplicate `reviewed_files`,
failed children, or abstention mean non-zero/degraded, never a clean review.

## Recover failed units

Missing finders or incomplete/duplicate coverage block verify. Recover only failed units:

```bash
python3 <skill>/scripts/omo_driver.py --args <out_dir>/workflow_args.json \
  --phase verify --retry-failed-units
```

This repairs failed-unit results/diagnostics, rebuilds whole-stage dedup/prescreen/
coverage and persists `find-stage.json`. No tracers until coverage is complete;
failure exits non-zero. Never combine with skeptic-abstention retry below.

## Retry degraded skeptics

To rerun only candidates that were kept as `skeptics unavailable (abstain)` in a degraded run:

```bash
python3 <skill>/scripts/omo_driver.py --args <out_dir>/workflow_args.json \
  --phase verify --retry-degraded-from <out_dir>/workflow-result.json
```

This leaves the original result untouched and writes `<out_dir>/workflow-retry-result.json`.

## Other hosts: sub-agent budget

Other hosts use current sub-agent tools with `workflow.js`'s finder/skeptic
prompts (or the contracts in the body when the asset is absent). Budget:
one finder per unit, then tracer and only if needed reproducer per candidate;
10 finder messages and 8 skeptic messages, batched reads.
