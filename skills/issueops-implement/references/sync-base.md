# base가 앞서 나갔는지 본다

구현 `next`는 readiness 경고를 주지 않는다. **claim 뒤 active(self)**에서 직접 관측한다.
claimable은 `released_completion_authority`로 거부되므로 재개 세션도 claim부터 끝낸다.

```bash
issueops execution sync-base --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json
```

- `merge_needed`가 false면 그대로 진행한다. `push_retry_required: true`는 병합은 끝났지만 그 결과가
  아직 원격 브랜치에 올라가지 않았다는 뜻이다. preview가 준 apply 명령으로 push를 마친다.
- true이고 `conflict_files`가 비었으면 `--apply --confirm --fingerprint <preview의 값> $ACTOR_FLAGS`로
  반영한다. **이 apply는 지금, 즉 봉인이 하나도 없는 4단계 진입에서만 한다.** 반영하면
  변경 집합이 봉인된 `BranchPrepare.BaseSHA` 기준으로 잡히므로 base가 바꾼 파일이 이
  사이클의 diff·티어·리뷰 대상에 들어온다. 그 사실을 계획의
  `## 하위 호환성과 side effect` 절에 적는다. 이미 추적 중인 미커밋 변경이 있으면
  `worktree_clean`으로 거부되므로 변경이 없는 이 시점에 한다.
- `conflict_files`가 있으면 계획의 영향 범위를 다시 보고 사용자에게 알린다.
- rebase하지 않는다. 봉인된 base identity와 기존 증거를 보존한다.
