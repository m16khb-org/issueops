# 편집 대상 확인 상세

```bash
pwd
git branch --show-current
git rev-parse HEAD
# 물리 경로로 비교한다(/tmp와 /private/tmp처럼 같은 디렉터리의 다른 표기가 있다).
test "$(pwd -P)" = "$(cd "$EXPECTED_WORKTREE" && pwd -P)"
git -C "$SOURCE_CHECKOUT" status --short
git status --short
```

patch·edit·생성 도구의 루트를 `$EXPECTED_WORKTREE`로 둔다. 도구가 다른 체크아웃에
썼으면 멈추고, 자기 변경만 canonical worktree로 옮긴 뒤 두 status를 다시 확인한다.
worker 프롬프트에는 exact lifecycle ID, generation, branch, worktree, 허용 경로,
수락 기준, 중단 규칙을 넣는다.
