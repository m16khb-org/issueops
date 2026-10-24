# Child 위임 세부 규칙

```bash
issueops child start --parent "$ISSUEOPS_ID" \
  --branch "$CHILD_BRANCH" --title "$TITLE" \
  --scope "$SCOPE" --acceptance "$CRITERION" \
  --host "$HOST" --session-id "$SESSION_ID" --cwd "$WORKER_PATH" --json
issueops child status --parent "$ISSUEOPS_ID" \
  --host "$HOST" --session-id "$SESSION_ID" --cwd "$WORKER_PATH" --json
```

- verdict는 `accept`·`reject`·`drop` 셋뿐이다. child scope를 고치는 amend 명령은
  없으므로 찾거나 발명하지 않는다.
- accept 전 rubric: 위임한 scope·expected worktree 준수, acceptance별 증거,
  선언한 검증 명령의 실행 결과, 무관한 diff·secret·stale scaffold 없음. 하나라도
  모호하면 accept하지 않는다.
- parent는 child record를 대신 수정하지 않는다. prompt에는 parent/child ID·generation·
  branch·canonical path·허용 scope·수락 기준·검증·중단 규칙을 넣고, child가 pwd·Git root·
  branch·HEAD·native process receipt·whoami를 확인한 뒤 쓰게 한다.
  출력은 변경 파일 요약, 검증 명령/결과, blocker/scope drift다. 새 migration·remote write·
  미계획 검증·parent 결정도 scope drift다. parent는 증거를 직접 검증한 뒤 accept/reject/drop을
  기록한다. 고칠 수 있는 미완료는 reject, 범위를 제거하거나 대체할 때만 사유를 적고 drop한다.
  Omo durable child는 canonical worktree에 cwd가 결속된 native team을 사용한다.
  prompt의 `cd`나 worktree 결속 없는 task로 대체하지 않는다. team 실행 불가면 사유를 보고하고
  같은 worktree의 독립 native `omo -p`로 같은 identity 게이트를 확인한다.

## 소유권과 scope drift

- child의 branch·worktree·lease는 child cycle 자신의 `branch prepare`와
  `execution prepare`가 소유한다. worktree provisioning은 `execution prepare`
  몫이며, legacy `worktree prepare` 계열 명령은 v1 카탈로그에서 제거되었다.
  parent가 child worktree를 직접 만들지 않는다.
- parent의 `branch_link_verified`는 child로 넘어가지 않는다. child 이슈와 child 브랜치의
  링크는 별개의 provider 사실이므로 child도 prepare 스킬대로 `branch await-link`로 읽고
  `--link-verified`를 기록한다. child는 자기 PR(base는 parent 브랜치)을 완료해야 `done`이 되고,
  그 뒤에 parent가 `child accept`한다.
- child가 scope drift를 보고하면 child를 조용히 넓히지 않는다. 사용자가
  승인해도 경로는 두 가지뿐이다: 새 scope를 **새 child**로 분리하거나, plan을
  개정하고 plan hash에 묶인 리뷰 freshness를 다시 확인한다.
