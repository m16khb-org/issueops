# 사용자가 이미 커밋해 둔 작업 이어받기

사용자가 다른 이름의 로컬 브랜치(예: `feat-sign`)에 커밋해 둔 작업을 사이클로 이어받을 때의 순서다.
사이클 브랜치는 `<이슈 번호>-<slug>` 이름과 봉인한 base에서 시작해야 하므로 그 브랜치를 그대로 쓰지 않는다.
사용자 브랜치와 그 커밋은 지우거나 고치지 않는다.

1. `git fetch origin <base>` 뒤 `git merge-base origin/<base> <사용자 브랜치>`를 base SHA로 봉인한다.
   사용자 커밋이 이 SHA 위에 있으므로 다음 단계가 fast-forward로 끝난다.
2. 평소대로 `branch prepare`, 연결 확인, 계획, `execution prepare`를 마친다. 계획에 "사용자 커밋 `<SHA>`를
   가져온다"를 적는다.
3. implement 단계 worktree에서 `git merge --ff-only <사용자 브랜치>`로 커밋을 가져온다. 새 커밋을 만들지 않으므로
   "implement에서는 커밋하지 않는다" 규칙에 걸리지 않는다. fast-forward가 안 되면(사용자 브랜치가 다른 base에서
   갈라졌으면) 멈추고 사용자에게 묻는다. 손으로 rebase하지 않는다.
4. base가 그 사이 앞서 나갔으면(사용자 브랜치를 만든 뒤 base에 커밋이 쌓였으면) `execution sync-base --preview`로
   합친다. 같은 파일을 고쳤으면 충돌이 나므로 [sync-base 규칙](../../issueops-implement/references/sync-base.md)의
   충돌 절차를 따른다.
