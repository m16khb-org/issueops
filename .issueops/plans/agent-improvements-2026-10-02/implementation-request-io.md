# I6 구현 증거: 요청 내 Git·history 재사용

기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e` (commit/push 없음).
이 문서는 I6 노드의 증거이며 전체 저장소·호스트 검증을 주장하지 않는다.

## 실행 환경

이 노드의 `tool.bash`로 확인한 값이다. 환경 변수는 바꾸지 않았다.

```text
PI_MODEL=claude-sonnet-5-5
PI_REASONING_LEVEL=high
```

부모가 알린 `gpt-6-astra/high`와 모델 값이 다르다. 이 노드 세션이 실제로 본 값은
위와 같고, effort는 high로 일치한다. 모델 구성 문제는 고치지 않고 불일치만 보고한다.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/adapter/preflight/preflight.go` | `GitObserver`에 주입 가능한 `Run` 필드(기본 `GitCmd`)를 추가했다. 네 번 읽던 history를 `git log -10 --format=%h%x00%s%x00%B%x00` 한 번으로 줄였다. 나머지 Git 사실(root, branch, HEAD, upstream, status, remote, ahead/behind)은 따로 읽는다. |
| `internal/adapter/preflight/helpers.go` | NUL 3필드 파서 `parseHistory`와 `last/commits(limit)/bodies` projection을 추가하고 `recentCommits`를 제거했다. `listRemotes`는 주입된 reader를 받는다. 불완전한 마지막 tuple은 버린다. |
| `cmd/issueops/issueopsapp/issueops_next_wiring.go` | `Next` 호출마다 만드는 `nextGitObservation`을 추가했다. cycle 선택의 `CurrentBranch(cwd)`와 `WorktreeState(root)`가 같은 root 문자열이면 `branch --show-current`를 한 번만 읽는다. toplevel·HEAD 읽기는 매번 새로 실행하고, 호출이 끝나면 memo도 사라진다. 전역 cache는 없고 `observeIssueOpsWorktreeState`는 메서드로 대체했다. |
| `internal/adapter/preflight/preflight_history_test.go` (신규) | recording runner 테스트와 실제 Git parity 테스트. |
| `cmd/issueops/issueopsapp/issueops_next_git_observation_test.go` (신규) | `TestNextGitObservation*` 4개. |
| `internal/adapter/issueops/readiness_git_observation_test.go` | ref·HEAD probe가 매번 새로 실행됨을 고정하는 테스트 1개 추가. |

`internal/adapter/issueops/readiness_git.go`는 **수정하지 않았다**. 기존 memo는 exact root+argv, 호출 범위 수명,
fetch 시 폐기라는 계약을 이미 지키고 있어서 이번 변경이 필요한 중복이 없었다. 새 테스트는 현재 동작을
고정하는 특성화 테스트이며 RED 단계에서도 통과했다.
`git diff`에 보이는 `cmd/issueops/issueopsapp/hook_facade.go`는 다른 노드의 변경이라 건드리지 않았다.

## RED

명령과 결과는 변경 전(production 코드 없음)에 기록했다.

```sh
go test ./internal/adapter/preflight ./internal/adapter/issueops -count=1
go test ./cmd/issueops/issueopsapp -run "TestNextLocalReadinessObservation|TestNextGitObservation" -count=1
```

| 명령 | exit | 결과 |
|---|---|---|
| preflight + issueops | 1 | `internal/adapter/preflight [build failed]`: `unknown field Run in struct literal of type GitObserver` (4곳). `internal/adapter/issueops`는 ok 213.2s. |
| issueopsapp 대상 테스트 | 1 | `[build failed]`: `undefined: newNextGitObservation` (7곳). |

두 RED는 컴파일 실패다. 테스트 seam이 없어서 동작 단언까지 가지 못했고, 동작 수준 RED는 만들지 못했다.

## GREEN

| 명령 | exit | 결과 |
|---|---|---|
| `go test ./internal/adapter/preflight ./internal/adapter/issueops -count=1` | 0 | preflight ok 4.571s, issueops ok 286.568s |
| `go test ./cmd/issueops/issueopsapp -run "TestNextLocalReadinessObservation\|TestNextGitObservation" -count=1 -v` | 0 | 새 `TestNextGitObservation*` 4개와 기존 `TestNextLocalReadinessObservation*` 3개 PASS |
| `gofmt -l` (변경 파일 전부) | 0 | 출력 없음 |
| `go vet ./internal/adapter/preflight ./internal/adapter/issueops ./cmd/issueops/issueopsapp` | 0 | 출력 없음 |
| LSP diagnostics | - | preflight 디렉터리 0건, wiring 파일 0건, readiness 테스트 0건. 새 next 테스트 파일은 심볼 정의 전 stale 보고가 있었으나 같은 시점의 `go vet`·테스트는 통과했다. |

테스트에는 sleep이나 시간 단언이 없다. 순서·횟수는 recording runner와 실제 임시 Git repo로 확인한다.

### 테스트가 고정하는 것

- 성공한 preflight에서 history 호출 4→1. 인자는 정확히 `log -10 --format=%h%x00%s%x00%B%x00`이고 root에서 실행된다.
- LastCommit, RecentCommits(5), StyleCommits(10), CommitBodies 10개의 순서·형식·한도, 탭이 든 subject.
- 커밋 1개·빈 body, git log 실패, 빈 출력, 잘린 마지막 tuple, git repo가 아닐 때(log를 읽지 않음).
- 실제 Git 12커밋 repo에서 한도 5/10/10, 순서, Lore body 5개.
- Next: 같은 root에서 `branch --show-current` 1회(중복 2→1), 총 probe 3회. root 문자열이 다르면(`/wt` vs `/wt/.`) 각각 관측. 실패·빈 root·비worktree 의미 유지.
- 새 `Next` 호출은 checkout과 새 commit 뒤의 branch·HEAD를 새로 읽는다(실제 Git).
- readiness: ref가 다른 `BaseAdvanced`와 `IsWorktree`는 재사용하지 않는다. fetch·exact root·argv 경계는 기존 테스트가 유지한다.

## 실제 사용 증거

임시 Git repo(12커밋, 짝수 커밋에 Lore body)에서 변경된 코드로 만든 임시 바이너리를 실행하고 `GIT_TRACE`로 호출 수를 셌다.

```text
$ issueops preflight --json <fixture>        # exit 0
git log 호출: 1, 전체 git 호출: 7
git rev-parse --show-toplevel / branch --show-current / rev-parse --short HEAD /
rev-parse --abbrev-ref --symbolic-full-name @{u} / status --porcelain=v1 --branch /
log -10 '--format=%h%x00%s%x00%B%x00' / remote -v
출력: ok=true, branch=main, last_commit="20ba1c0 fix: change 12", recent_commits 5개,
commit_style_hints: recent_count=10, conventional_subjects=10, lore_bodies=5
```

변경 전 구조(소스 기준)에서는 같은 호출이 history 4회를 포함해 전체 10회였다. 이는 소스에서 센 값이며 성능 향상 주장은 아니다.

`issueops next --cwd <fixture> --json`(빈 state, exit 0)도 실행했다. 이 경우 record가 없어서 cwd와 execution root가
겹치는 경로가 실행되지 않으므로 Next의 중복 제거는 이 실행으로 증명하지 못했다.
그 증명은 위 단위·실제 Git 테스트가 담당한다.

## 남은 통합 우려

- 원자적 snapshot이 아니다. 각 명령은 별도 subprocess이고 외부 변경이 사이에 끼어들 수 있다. Next memo는 한 호출 안의 branch 읽기 한 가지에만 쓰이며, 그 안에서 외부 checkout이 일어나면 이전 값이 재사용된다. 락·재시도는 추가하지 않았다.
- 파싱 차이: 이전에는 `%B%x1e`로 body를 나누고 전체 출력을 trim했다. 이제 body는 commit마다 `%B` 원문이다(끝 개행 유지, 빈 history는 `[""]`). Lore 판정은 줄 단위 trim이라 결과가 같음을 확인했다. 제목이 빈 commit은 예전에는 window 마지막일 때만 trim으로 사라졌지만 지금은 그대로 남는다. 일반 Git 사용에서는 영향이 없다.
- `GitObserver{}`는 기존 호출처(`basic_wiring.go`, `verify_work_wiring.go`, `mcp_facade.go`, 여러 테스트)와 컴파일 호환이다. 해당 파일은 수정하지 않았다.
- issueops 패키지 테스트가 약 215~290초 걸려 더 짧은 셀 예산 안에서는 완료되지 않는다. 통합 검증에서는 백그라운드로 실행해야 한다.
- 전체 저장소 테스트·race·build와 세 호스트 검증은 실행하지 않았고 부모 통합 단계의 몫이다.
