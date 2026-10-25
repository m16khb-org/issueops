# issueops 저장소의 self-verify 배터리

issueops 하네스 저장소 자체를 바꾸는 사이클에서만 읽는다.

이 issueops 저장소의 self-verify 배터리를 실행할 때는 먼저
`issueops status --id "$ISSUEOPS_ID" --json`을 읽고, 해당 사이클의
`branch_prepare.base_sha`를 `BASE_SHA`로 사용한다. 커밋 후 worktree가 깨끗해도
기준을 전달한다. base가 없으면 검증을 중단하고 준비 기록을 확인하며, 무기준 실행이나
다른 사이클·origin/main 추론으로 우회하지 않는다.

```bash
./bin/issueops self-verify --base-ref "$BASE_SHA" --seed=100 --target-score=95 --llm-eval=false --json
```

MCP를 사용하면 같은 값을 `self_verify`의 `base_ref`에 전달한다. risk QA 결과의
`scope.base_sha`와 `scope.head_sha`를 실제 준비 기준과 HEAD에 대조하고, scope 오류가
없으며 요구된 vet/race 명령이 실행됐는지 확인한다. 범위는 기준 tree부터 관측 HEAD
tree까지의 변경과 staged·unstaged·untracked 경로의 합집합이다.
