# 준비 뒤 base 바꾸기

준비가 끝난 뒤 base를 바꿔야 하면(예: main → develop) 단계에 따라 길이 다르다.

- 링크 브랜치를 만들기 전: 새 `--base-branch`·`--base-sha`로 `branch prepare`를 다시 기록하면 된다.
- 링크 브랜치를 만든 뒤, PR/MR 전:
  1. lease를 인수하기 전에 `git fetch origin <새 base>`를 하고 `git merge-base --is-ancestor <옛 base_sha>
     origin/<새 base>`로 새 base가 옛 base를 포함하는지 본다. 포함하지 않으면(main과 갈라진 develop) 병합이
     옛 base의 커밋을 PR에 싣게 되어 `branch prepare`가 거부한다. 사용자에게 이 사이클을 폐기하고 새 base에서
     새 사이클을 시작하자고 제안한다.
  2. 포함하면, 링크 브랜치가 옛 base에서 만들어졌으므로 옛 base 그대로 `execution prepare`까지 마친다.
  3. lease holder가 worktree에서 `branch prepare`를 새 base로 다시 기록하고 `execution sync-base --preview`로
     병합한다([sync-base 규칙](../../issueops-implement/references/sync-base.md)). 손으로 rebase하지 않는다.
- PR/MR을 만든 뒤: provider에서 대상 브랜치를 바꾸고 `issueops branch retarget --id ID --base-branch REF
  --reason TEXT $RECORD_ACTOR_FLAGS`로 기록한다.
