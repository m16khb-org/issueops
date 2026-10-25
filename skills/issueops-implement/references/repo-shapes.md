# 저장소 형태별 worktree 준비

저장소에 `.gitmodules`가 있으면 worktree에서 `git submodule update --init`을 먼저 한다. worktree는 submodule을
빈 디렉터리로 만들므로, 그대로 두면 테스트가 엉뚱한 이유로 실패한다. 초기화해도 `git status`는 깨끗하다.
source checkout이 sparse checkout이면 worktree도 같은 범위만 받는다. 계획이 그 밖의 경로를 건드리면 worktree에서
`git sparse-checkout disable`을 한다(source checkout의 설정은 그대로다).
