# 편집 대상 확인 상세

```bash
pwd
git branch --show-current
git rev-parse HEAD
test "$PWD" = "$EXPECTED_WORKTREE"
git -C "$SOURCE_CHECKOUT" status --short
git status --short
```

patch·edit·생성 도구의 루트를 `$EXPECTED_WORKTREE`로 둔다. 도구가 다른 체크아웃에
썼으면 멈추고, 자기 변경만 canonical worktree로 옮긴 뒤 두 status를 다시 확인한다.
worker 프롬프트에는 exact lifecycle ID, generation, branch, worktree, 허용 경로,
수락 기준, 중단 규칙을 넣는다.
