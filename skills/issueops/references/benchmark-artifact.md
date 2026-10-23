# IssueOps benchmark artifact contract

Benchmark 응답에는 의도나 계획만 쓰지 말고 다음 labeled evidence를 넣는다.

```text
Durable state record: <IssueOps id, phase, readiness gates, state path/tool output>
Phase routing: <problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup>
Flow evidence: <Issue, plan, TDD, sub-agent decision, feedback, PR/MR artifacts>
Hook boundary: <what hooks may suggest and what only the main agent/CLI owns>
Cleanup/readiness evidence: <strict readiness, merge/cleanup status, remaining choices>
```

semantic judge는 artifact 작성자가 아닌 fresh-context host agent가 맡는다.
deterministic pass를 먼저 실행하고, JSON-only judge map을 `--judge file`로
strict-decode한다. 외부 judge는 read-only이며 workspace나 remote를 수정하지
않는다.
