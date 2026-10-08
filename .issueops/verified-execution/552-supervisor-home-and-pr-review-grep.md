# #552 Linux supervisor HOME 불일치와 pr-review grep 대체 경로 결함: verified-execution report

- lifecycle: `io-7426b49dc042`, direct execution generation 2(generation 1 holder가 release한 뒤 새 Claude 세션이 replace·claim으로 인수)
- issue: https://github.com/m16khb-org/issueops/issues/552
- branch: `552-supervisor-home-and-pr-review-grep`, base `main` @ `3a5383281cba84f3ffdff18448e164a48c52ae83`(구현 진입 시 sync-base preview `merge_needed=false`)
- 계획: `.issueops/issues/552/plan.md`, 게이트 원장: `.issueops/issues/552/gates.md`
- 상태: 5단계 정리에서 확정했다. 6단계 문서 반영과 7단계 검증 결과는 이 파일에 이어 적는다.

## 수용 기준별 결과

| 기준 | 결과 | 근거 |
|---|---|---|
| HOME이 user manager와 다른 Linux HTTP 설치가 쓰기 전에 원인을 밝힌 오류로 실패(stdio 자동 전환 없음) | 단위 테스트 충족, 실제 runner는 PR CI로 확인 | G4, CI 회귀 단계(G9, G10) |
| 같은 HOME의 Linux HTTP 설치와 macOS launchd 동작 불변 | 충족 | G4의 첫 설치·XDG·UnitPath·조회 실패·HOME 부재·이스케이프·launchd subtest |
| pr-review source 표시가 실제 검색 도구와 일치 | 충족 | G5 `test_source_line_names_the_tool_that_ran`, `test_source_line_lists_codegraph_and_its_fallback` |
| rg가 없을 때 git 저장소에서 `.gitignore` 무시 파일 제외 | 충족 | G5 `test_git_grep_skips_ignored_files`, `test_git_grep_finds_definitions_under_lock_named_directories` |
| 결함마다 실패 테스트로 재현 후 수정, 정적 검사·race·Python·self-verify·PR CI 통과 | 로컬 게이트는 원장, PR CI는 push 뒤 확인 | G1~G7, G10, G8·G9 |

## S1. supervisor 사전 감지

- `supervisor` 인터페이스에 `checkUnitPath(ctx)`를 추가했다. launchd는 plist 경로를 직접 넘기므로 항상 통과한다. systemd는 `systemctl --user show-environment`로 실행 중인 user manager의 `HOME`·`XDG_CONFIG_HOME`을 읽고, 기대 디렉터리(`$XDG_CONFIG_HOME/systemd/user`, 없으면 `$HOME/.config/systemd/user`)와 쓰려는 unit 디렉터리를 `filepath.Clean` 기준으로 비교한다. 다르면 `systemctl --user show -p UnitPath --value`에 그 디렉터리가 있는지 본다. 둘 다 아니면 unit 디렉터리, manager HOME, UnitPath, 해결 방법(같은 HOME에서 설치 또는 `--mcp-transport=stdio`)을 담은 오류를 낸다.
- `Service.Prepare`는 `requireSupervisor` 바로 뒤, `EnsureBearer`·log·unit 쓰기 전에 이 검사를 호출한다.
- 구현 리뷰 1라운드(revise): `issueops mcp service start`는 `Prepare` 실패를 모두 `credential_failed`로 보고하므로, 새 거부가 이전의 `supervisor_failed` 대신 `credential_failed`로 나왔다. 오류를 sentinel `errUnitPathNotSearched`로 감싸고 `Start`가 그 경우만 `supervisor_failed`로 분류한다. RED: `TestStartReportsUnitPathRefusalAsSupervisorFailed`가 `ErrorCode:credential_failed`로 실패 → GREEN. 오류 문구는 그대로다.
- 구현 리뷰 2라운드(revise): Linux 배선 테스트 `TestSupervisorUnitRunsAbsoluteBinaryWithExplicitRootAndState`가 임시 HOME에서 실제 `systemctl`로 `Prepare`를 부르므로, user manager가 있는 CI runner에서는 새 거부에 걸려 실패한다. 로컬 게이트는 darwin(launchd 경로)에서만 돌아 잡지 못했다. RED: `GOOS=linux GOARCH=arm64 go test -c`로 만든 테스트 바이너리를 linux 컨테이너에서 runner와 같은 응답(`show-environment`가 `HOME=/home/runner`)을 내는 가짜 `systemctl`로 실행해 같은 거부로 실패를 재현했다. 수정: 이 테스트가 manager에 닿지 않는 가짜 `systemctl`(항상 exit 1)을 PATH 앞에 둬 검사를 판정 불가로 만든다. 그 결과 runner의 실제 user manager에 `stop`을 보내던 부작용도 없어졌다. GREEN: 같은 컨테이너와 darwin에서 통과. 직접 `Prepare`를 부르는 Linux 테스트는 이것뿐이다(실제 install 경로 테스트는 dry-run이거나 stdio).
- 판정할 수 없으면 막지 않는다: 조회 실패, manager 환경에 HOME 없음, 값이 `$'...'`로 이스케이프됨. 그때는 기존 load 단계가 명령·출력을 담은 오류로 실패한다(ADR 2026-10-02, AGENTS.md readiness gate 금지).
- RED: 거부 subtest가 `Prepare succeeded; commands=[]`로 실패(수정 전 코드는 조회하지 않음). 이스케이프 subtest는 첫 구현에서 거짓 거부로 실패한 뒤 guard를 넣어 GREEN. 최종 8 subtest(G4).
- CI: `.github/workflows/ci.yml`에 "HTTP MCP install refuses a HOME the user systemd manager does not search" 단계를 추가했다. 임시 HOME에서 `--mcp-transport=http`로 설치해 실패해야 하고, 출력에 `is not searched by the running systemd user manager`가 있어야 하며, 임시 HOME에 bearer와 unit이 없어야 한다. `scripts/ci_workflow_test.py`의 `test_http_home_mismatch_block_requires_refusal_before_writes`가 거부(통과)와 성공·다른 오류·unit 씀·credential 씀(모두 실패) 다섯 경우의 전파와 임시 HOME 정리를 고정한다. 단계의 grep 검사와 쓰기 검사를 각각 무력화한 변형에서 해당 subtest가 실패하는 것을 확인했다.
- 계획 리뷰 비차단 정정: 실제 조회는 `systemd-analyze`가 아니라 `systemctl --user show-environment`와 `show -p UnitPath`다. G4는 계획의 "4종"이 아니라 8 subtest다(조회 실패·HOME 부재·이스케이프를 나눔).

## S2·S3. pr-review 검색 도구 표시와 `.gitignore`

- `_rg_symbol`이 `(rows, tool)`을 돌려준다. rg가 없으면(`FileNotFoundError`) `git rev-parse --is-inside-work-tree`가 참일 때 `git grep --untracked -n -w -F -I --max-count 3`과 glob magic 제외(`**/*lock*`, `**/*.min.*`, `**/node_modules/**`, `**/dist/**`)로 찾는다. git grep 종료 코드가 1보다 크면(옵션 미지원 등) 기존 grep으로 넘어간다. git이 없거나 작업 트리가 아니면 기존 grep이다.
- `collect_defs`가 기호마다 `source`를 기록하고, `render_defs`는 source 줄에 실제로 쓴 도구를 처음 쓰인 순서대로 중복 없이 나열한다(기호가 없으면 `none`). `render_defs`의 `codegraph` 인자는 쓰이지 않게 되어 뺐고 호출부 두 곳을 맞췄다. source 줄을 파싱하는 소비자는 없다.
- RED: 새 테스트 4개와 기존 grep 테스트의 source 단언이 `source: rg`로 실패 → GREEN. pr-review 스위트 101개 통과(G5).

## 하위 호환성과 side effect

- 같은 HOME의 Linux HTTP 설치: manager 기대 디렉터리와 같으므로 조회 한 번만 늘고 동작은 같다. macOS: 변경 없음.
- HOME이 다른 Linux HTTP 설치: 이전에는 credential·log·unit을 쓴 뒤 enable에서 실패했고, 이제는 아무것도 쓰지 않고 실패한다.
- `mcpservice.Status` DTO, error_code, CLI·MCP JSON 출력 변경 없음. 원격·record 계약 변경 없음.
- CI에 단계 하나가 추가된다. 쓰기 전에 실패하므로 runner 계정의 user manager를 바꾸지 않는다.
- pr-review `defs.md`의 source 줄 문구가 바뀐다.

## ai-slop-clean

- 범위: 이번 diff의 파일과 직접 관련된 파일.
- 단순화: `_rg_symbol`의 `p = None` 센티널과 중첩 try를 단계별 조기 반환(rg → git grep → grep)으로 평탄화했고, 출력 정리 두 줄을 `_hits`로 한 번만 둔다. 동작은 같다. 손댄 git grep 실패 분기에 테스트가 없어 `test_failing_git_grep_falls_back_to_grep`를 먼저 추가했다.
- 주장 정리(unsupported-claim): git grep 종료 코드 주석이 "1보다 크면 git이 이 옵션을 모르는 것"이라고 원인을 단정했다. 실제로는 어떤 실패든 같은 경로를 타므로 "git grep이 실패한 것(예: `--max-count`가 없는 2.38 미만)"으로 고쳤다.
- 유지: `checkUnitPath`의 주석(manager가 자기 HOME으로 경로를 정한다, `systemd-analyze`를 쓰지 않는 이유, UnitPath가 없는 디렉터리를 빼므로 일치 확인에만 쓴다, `$'...'` 이스케이프)과 CI 단계 주석, git pathspec glob magic 주석은 코드만으로 드러나지 않는 결정이다.
- 측정(코드 파일 `*.go *.py *.sh *.yml`의 base 대비 추가 줄, 주석·print 줄을 잡음으로 분류): 정리 전 SNR 0.939(signal 278, noise 18, total 296, 중복 줄 48), 정리 후 0.940(signal 283, noise 18, total 301, 중복 줄 46). 늘어난 signal은 새 테스트 한 개다.
- 범위 밖 발견: 없음.

## 성능 측정

- S1: HTTP 설치의 `Prepare`마다 `systemctl --user show-environment` 한 번, 기대 디렉터리와 다를 때만 `systemctl --user show -p UnitPath --value` 한 번이 늘어난다. 설치 경로이며 hot path가 아니다(측정하지 않음, 성능 주장도 하지 않는다).
- S3: rg가 없을 때만 돈다. 이 저장소에서 기호 `checkUnitPath` 검색 5회(load average 30 안팎): git grep 중앙값 0.349초(최소 0.227초), 기존 grep 중앙값 0.669초(최소 0.557초). git 작업 트리 판정 `git rev-parse` 한 번이 기호마다 더해진다.

## 프로젝트 문서 반영

- 문서 → 구현: CONSTITUTION(근본 원인 제거), CONVENTIONS, ARCHITECTURE의 mcpservice 경계, ADR 2026-10-02(명시 오류, stdio 자동 전환 금지), AGENTS.md(readiness gate 금지)와 대조했다. 위반 없음. 조회 실패를 차단 사유로 쓰지 않는다.
- 구현 → 문서: `.issueops/operations/install.md`의 Linux HOME 문단을 고쳤다. 이전 문단은 "`supervisor_failed`로 실패한다"고 설명했는데 이제는 쓰기 전에 unit 검색 경로 오류로 멈추고, 판정할 수 없을 때만 기존 load 실패를 탄다. CI의 새 회귀 단계도 적었다.
- 새 주의사항 `.issueops/cautions/2026-10-08-user-systemd-manager-base-ci.md`(`project_docs_append`): systemd-analyze는 manager에게 묻지 않는다, UnitPath는 없는 디렉터리를 뺀다, show-environment의 `$'...'` 이스케이프, base 커밋에서 거짓 통과하는 HEAD CI 게이트, stderr의 `skipped` 오탐. `.issueops/CAUTIONS.md` 색인에 한 줄을 넣었다(`project_docs_revise`는 문서 전체를 다시 보내야 해서 같은 한 줄을 직접 편집으로 넣었다).
- 계획의 `## 적용되는 결정과 주의사항`에 없던 항목: base 커밋 CI 게이트 거짓 통과, golangci-lint 캐시 재발(#550 주의사항에 이미 있어 새로 적지 않음), `$'...'` 이스케이프.
- 검증: `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update -count=1` → ok, golden 변화 없음. docs checker(`skills/project-docs-optimize`) 위반 1건은 base부터 있던 `.issueops/adr/roadmap.md` 253줄(이번 변경과 무관).

## 남은 일

- 설치 HOME과 manager HOME이 symlink로만 다른 경우(같은 디렉터리, 다른 문자열)는 거부된다. 일반 로그인 세션에서는 두 값이 같은 출처라 문제가 되지 않지만, 보고가 오면 디렉터리 동일성 비교를 검토한다.
- launchd는 임시 HOME 설치도 실제 gui 도메인에 서비스를 올린다(격리 비대칭). 이번 범위에서 바꾸지 않는다.

## 검증 기록

게이트 실행 결과는 원장 `.issueops/issues/552/gates.md`의 EVIDENCE가 소유한다. 실행 중 바로잡은 것:

- `internal/architecture`의 `TestDDDResponsibilityInventoryMatchesSource`가 새 메서드 `launchd.checkUnitPath`, `systemd.checkUnitPath`를 인벤토리에서 찾지 못해 실패했다(G6·G7). `-update-ddd-inventory`로 갱신했고 diff는 그 두 심볼 추가뿐이다.
- G3: golangci-lint 캐시가 삭제된 #550 worktree 경로의 결과(`../550-ci-install-and-followups/...` SA1012)를 재사용했다. 코드 문제가 아니라 캐시 문제라 `golangci-lint cache clean` 뒤 darwin·linux 모두 0건을 확인했다.
- G5: CHECK가 stderr 전체에서 `skipped`를 찾아, 테스트 이름과 driver 로그의 같은 단어에 걸렸다. unittest 요약의 `(skipped=` 표기만 보도록 CHECK를 고쳤다. 코드는 바꾸지 않았다.
- G8: 첫 실행에서 HEAD가 아직 base 커밋이라 main의 CI run을 읽고 `CI_GREEN`으로 충족됐다. 이 변경의 증거가 아니므로 체크를 풀고 G9와 함께 abandon했다. PR CI verify job과 HTTP 회귀 단계의 통과는 push 뒤 PR 발행 전에 run URL로 확인한다.
- `gates check`는 이미 충족되고 증거가 있는 게이트를 다시 실행하지 않는다(`internal/domain/gates/check_decision.go`의 `ShouldRun`). 그래서 5단계 정리 뒤와 Start 수정 뒤의 두 실행은 아무것도 재실행하지 않고 끝났다. 정리 직후 pr-review 스위트는 별도로 실행해 통과를 확인했지만, Go 게이트는 정리·수정 뒤 증거가 없었다. 충족 게이트 8개의 체크와 EVIDENCE를 pending으로 되돌리고 전부 다시 실행했다(아래 최종 실행).
- 환경: darwin/arm64, load average 13~151(G7 실행 중 최고). 로컬 Python 단계는 scratchpad uv venv(Python 3.13, `scripts/python_test_requirements.txt`)를 PATH 앞에 두고 실행했다.
- 최종 실행(2026-10-08 01:59~02:14 UTC, Start 수정 뒤): `issueops gates check --write`가 G1~G7·G10 8개를 모두 새로 실행해 충족했다. G8·G9는 abandon(PR CI run URL로 확인). 별도로 `self-verify --base-ref <base> --seed=100 --target-score=95 --llm-eval=false`가 27/27 단계 통과(Start 수정 전, 403초)했다.
- 최종 실행 2(2026-10-08 02:22~02:37 UTC, 배선 테스트 수정 뒤): 충족 게이트 8개를 다시 pending으로 되돌려 전부 새로 실행했고 모두 충족했다. Linux 배선 테스트는 이 원장 밖에서 linux 컨테이너로 확인했다(위 S1 절).
