# Child 위임 세부 규칙

```bash
issueops child start --parent "$ISSUEOPS_ID" \
  --branch "$CHILD_BRANCH" --title "$TITLE" \
  --scope "$SCOPE" --acceptance "$CRITERION" \
  --host "$HOST" --session-id "$SESSION_ID" --cwd "$WORKER_PATH" --json
issueops child status --parent "$ISSUEOPS_ID" \
  --host "$HOST" --session-id "$SESSION_ID" --cwd "$WORKER_PATH" --json
```

- 이슈 단계의 `remote create-child`는 provider의 child 이슈를 만들고, `child start`는 로컬 child
  사이클을 만든다. `--child-issue-url`로 둘을 잇는다. 이슈 없는 child는 parent 이슈의
  `.issueops/issues/<n>/` 파일을 같이 써서 sibling child PR끼리 충돌한다.
- `child start`는 parent의 domain·architecture·compatibility 리뷰를 상속하고 plan-prep과
  devil's-advocate를 parent 판정으로 waive한다. child 계획이 parent 계획 범위 밖이면 scope drift다.
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

## 한 세션이 child까지 직접 할 때

한 세션은 lease를 하나만 쥔다. `child start`는 parent lease가 active여야 하고, child의
`execution prepare`는 그 세션이 다른 lease를 쥐고 있지 않아야 한다. 그래서 순서는 다음과 같다.

1. parent lease를 쥔 채 `child start`로 child 사이클을 만든다(child마다).
2. 각 child의 `branch prepare`·계획 스테이징을 마친다(lease 전 단계라 parent lease와 무관하다).
3. `execution release`로 parent lease를 내려놓는다.
4. child 하나씩 `execution prepare` → 구현 → PR(base는 parent 브랜치) → `execution complete`.
5. parent의 `next`가 주는 `replace --preview` → `--reseed` → `claim` 체인으로 parent lease를 되찾는다.
6. child의 증거를 확인하고 `child accept`로 판정을 기록한다.
7. accept한 child PR을 parent 브랜치에 머지하고(`issueops remote merge-pr --id <child> --confirm`,
   승인 범위 안일 때), parent 워크트리를 원격 parent 브랜치로 fast-forward한다.

child PR은 기본 브랜치가 아닌 parent 브랜치로 머지되므로 PR 본문의 `Closes #n`이 child 이슈를
닫지 않는다. 정리 단계의 `cleanup close-children`이 그것을 닫는다(`child_tasks_closed`).
