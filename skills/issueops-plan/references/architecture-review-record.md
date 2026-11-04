# 아키텍처 검토 승인 기록

`SKILL.md`의 `## 아키텍처 검토`가 가리키는 기록 명령이다. 본문은 `pass` 판정일 때만 승인을 기록한다는
조건과 승인 규칙을 소유하고, 이 문서는 명령 전문을 소유한다. `$HOST`, `$REVIEWER_MODEL`,
`$REVIEWER_EFFORT`는 리뷰어 서브에이전트의 실제 host·모델·effort다.

```bash
issueops architecture review --id "$ISSUEOPS_ID" \
  --problem-summary "<문제>" --proposed-architecture "<아키텍처 결정>" --refactor-plan "<리팩터 계획>" \
  --alternative "<기각한 대안과 이유>" --risk "<위 문서 확인에서 온 제약>" \
  --verification "아키텍처 검토로 대안과 위험을 확인했다" \
  --reviewer-context subagent --reviewer-host "$HOST" --reviewer-model "$REVIEWER_MODEL" --reviewer-effort "$REVIEWER_EFFORT" \
  --approved $RECORD_ACTOR_FLAGS --json

issueops record-routing --id "$ISSUEOPS_ID" --phase plan --skill implementation-planning \
  $RECORD_ACTOR_FLAGS --json
```

- 승인은 스테이징한 아키텍처 문서의 sha256에 묶인다. 문서가 없거나 필수 절이 비면 기록이
  거부된다.
- `--verification`에는 "아키텍처"와 "검토"가 함께 들어가야 한다(영어면 "architecture"와
  "review"). 그 두 단어가 없으면 `architecture_review_evidence`가 통과하지 않는다.
- `--reviewer-context`는 `subagent` 또는 `inline`인 감사 필드다. 독립 서브에이전트가 돌려준 판정만 `subagent`로 기록한다. 저자 세션의 검토는 사용자가 inline 리뷰를 명시적으로 지시했을 때만 `inline`으로 기록하고, 그 외에는 리뷰가 아니라 규칙 위반이다. 이유는 [세부 규칙](gates-and-review-details.md)을 연다.
- `--refactor-plan`, `--alternative`, `--risk`는 각각 하나 이상 필요하고, open question은
  0이어야 승인된다. 열린 질문이 남았으면 그것을 먼저 닫는다.
- `revise`면 문서를 고쳐 다시 스테이징하고 다시 검토한다. `stop`이면 승인을 기록하지 않고
  이슈 본문의 사실 변경을 `feedback add --classification contract_change`로 남긴 뒤 이슈를
  동기화한다.
