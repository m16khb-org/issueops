# 게이트 완료 조건 관측과 계약 변경 기록

## 게이트 원장

게이트 중 적어도 하나는 이슈의 완료 조건, 곧 보고된 실패가 사라진 최종 상태를 직접 관측한다.
고치는 의존성을 fake로 바꾼 unit 테스트나 호출 순서만 확인하는 테스트는 그 게이트를 대신하지 못한다.
그 관측이 smoke 명령(실제 명령을 실행한 뒤 상태를 읽어 비교)이면 구현 단계나 측정 계획에만 적지 말고
게이트의 CHECK로 쓴다. 테스트로 관측하면 그 테스트가 무엇을 실제로 실행하고 무엇을 비교하는지 게이트
결과 칸에 적는다.

## 검토가 이슈 본문의 사실을 틀렸다고 판정할 때

검토가 이슈 본문의 사실(완료 기준, 범위, 원인 설명, 작업 구조)을 틀렸다고 판정하면
계획만 고치고 끝내지 않는다. 그 사실을 계약 변경으로 기록한다. 기록이 남으면 기존
게이트 `contract_feedback_issue_update`가 PR 준비를 막고, `sync-issue`로 본문을 고친 뒤
`feedback mark-issue-updated`를 기록해야 풀린다.

```bash
issueops feedback add --id "$ISSUEOPS_ID" --source plan-review --classification contract_change \
  --body "<이슈 본문에서 바뀐 사실>" $RECORD_ACTOR_FLAGS --json
```

## 아키텍처 검토의 `--reviewer-context`

`--reviewer-context`는 `subagent` 또는 `inline`이며 감사 필드다. 하네스는 자기신고를 검증할 수
없으므로 `inline`도 기록되지만, 저자 세션이 직접 검토한 기록은 리뷰가 아니라 규칙 위반이다.
독립 서브에이전트가 돌려준 판정만 `subagent`로 기록한다. 예외는 사용자가 서브에이전트 없이
inline 리뷰를 명시적으로 지시한 경우 하나다. 그때는 `inline`으로 기록하고 독립 리뷰가 아니라는
사실을 보고에 남긴다.
