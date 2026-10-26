# 호스트 QA 실행 방법

에이전트 QA를 실제 호스트에서 돌릴 때 쓰는 명령과 sandbox 구성이다. 값은 예시이며
계정에서 쓸 수 있는 모델로 바꾼다.

## sandbox 구성

<!-- skill-shell: destructive recovery="verify S is a throwaway /tmp sandbox before rm -rf; the sandbox is rebuilt from ./seed, never a user repository or state directory" -->
```bash
S=/tmp/qa-example
rm -rf "$S" && mkdir -p "$S"
cp -R ./seed "$S/seed"
git -C "$S/seed" init -q -b main
git -C "$S/seed" add -A
git -C "$S/seed" -c user.name=qa -c user.email=qa@example.test commit -qm seed
git init -q --bare -b main "$S/origin.git"
git -C "$S/seed" push -q "$S/origin.git" main
git clone -q "$S/origin.git" "$S/repo"
```

provider stand-in(가짜 `gh`, `glab`)은 `$S` 밖 한 디렉터리에 두고 상태 파일 경로를 환경
변수로 받게 만든다. origin URL이 실제 호스트 형태(`git@github.com:owner/repo.git`)여야
provider 판정이 실제와 같다. 이때 가짜 `ssh`를 `GIT_SSH_COMMAND`로 지정해서 그 URL이
로컬 bare 저장소를 가리키게 한다.

stand-in이 지원해야 하는 동작은 다음과 같다.

- 이슈: 생성, 조회, 목록, 수정, 닫기, 다시 열기, 라벨 목록
- PR/MR: 생성(draft), 조회(`--json`, `--jq`), 목록, ready, 병합(squash 포함), base 변경, 리뷰와 리뷰 스레드
- CI: 체크 목록, 실패 로그 조회
- 장애 플래그: 응답 유실, 호출자 종료, 첫 head CI 실패. 플래그는 한 번 쓰면 사라진다
- 상태 파일 잠금: 병렬 호출에서 갱신을 잃지 않게 `flock`으로 잠근다

## 환경 접두사

프롬프트에 "모든 셸 명령을 아래 줄로 시작하라"고 적는다. 일부 호스트는 셸마다 `PATH`를
다시 정하므로 한 번 export해서는 유지되지 않는다.

```bash
export PATH=/tmp/fakegh:$PATH GIT_SSH_COMMAND=/tmp/fakegh/ssh FAKE_GH_ORIGIN=$S/origin.git FAKE_GH_STATE=$S/gh.json STATE_DIR=$S/state; hash -r; cd $S/repo;
```

## 호스트 실행

작업 세션의 식별 변수가 QA 에이전트에 상속되지 않게 비우고 실행한다. 상속되면 세션
식별이 모호해진다.

| 호스트 | 명령 |
|---|---|
| Claude Code | `env -u PI_SESSION_ID claude -p --model sonnet --dangerously-skip-permissions < prompt.md` |
| omp | `env -u PI_SESSION_ID omp -p --model anthropic/claude-sonnet-5-5 "$(cat prompt.md)"` |
| Codex | `codex exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox -m <model> "$(cat prompt.md)"` |
| opencode | `opencode run --standalone --model <provider/model> "$(cat prompt.md)"` |

- Claude Code의 `-p`는 프롬프트를 stdin으로 받는 편이 안전하다.
- opencode는 `--standalone` 없이 실행하면 공유 서비스 프로세스에서 도구가 실행되어
  세션 소유권 검사가 달라진다.
- 무료 모델의 한도가 걸리면 다른 모델로 바꾸지 않는다. 짧은 확인 프롬프트로 한도가
  풀렸는지 확인한 뒤 다시 실행한다.

## 보고서 요청

프롬프트 끝에 아래 내용을 넣는다.

- 작업이 끝나면 `$S/final.txt`에 한국어 보고서를 쓴다.
- 실패한 명령마다 명령, 출력 요약, 다음에 한 행동을 적는다.
- 안내가 모호했던 곳과 추측한 곳을 따로 적는다.

## 라우팅 평가

스킬 설명을 바꾸면 전체 카탈로그와 요청 목록으로 라우팅을 평가한다.

1. 저장소 루트에서 `skills/*/SKILL.md`의 `name`과 `description`으로 카탈로그를 만든다.
2. 요청마다 기대 스킬을 정한 목록을 준비한다.
3. 모델 세 개 이상에게 "요청마다 스킬 하나를 JSON으로 고르라"고 묻는다.
4. 정답률이 바뀌기 전보다 낮으면 설명 변경을 되돌린다.
