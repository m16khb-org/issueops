# self-verify regression report

The nightly gate started failing after yesterday's merge. The CI step is:

```bash
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json > sv.json
jq -e '.passed == true' sv.json
```

Last three runs:

```text
run 1201  self-verify exit 0  jq exit 1   wall 412s
run 1202  self-verify exit 0  jq exit 1   wall 405s
run 1203  self-verify exit 0  jq exit 1   wall 418s
```

Excerpt of `sv.json` from run 1203:

```json
{
  "ok": true,
  "termination_eligible": true,
  "summary": {"termination_eligible": true, "goal_scores": {"contract": 98, "docs": 97, "tests": 96}},
  "failed_steps": []
}
```

The developer suspects yesterday's merge (a refactor of `internal/application/selfverify`) broke scoring and
asked for the regression to be found and fixed.
