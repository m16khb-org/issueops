# 계획: #552 Linux supervisor HOME 불일치와 pr-review grep 대체 경로 결함

- lifecycle ID: `io-7426b49dc042`
- issue: https://github.com/m16khb-org/issueops/issues/552
- branch: `552-supervisor-home-and-pr-review-grep` (base `main`, 봉인 SHA는 record의 `branch_prepare.base_sha`)
- 요청 범위: "찾은 결함들을 다 해결하는 이슈를 만들고 /issueops 로 진행". 종료점은 draft PR 발행과 `execution complete`다. merge·cleanup은 범위 밖이다.
- 세션: `execution prepare --mode direct` 뒤 환경별 자동 세션 인계(Orca ready면 같은 worktree의 새 세션, owner model `claude-opus-5-5`, effort `high`).

## 목표와 비목표

목표: 이슈 완료 기준 다섯 개를 결함별 커밋으로 충족한다.

비목표: Linux 기본 transport 정책 변경, supervisor 실패 시 stdio 자동 전환, CLI·MCP 공개 출력 계약 변경, pr-review rg 경로 동작 변경, macOS launchd 경로 변경.

## 사실 확인 결과

- `internal/adapter/mcpservice/supervisor.go`: `systemd.unitPath()`는 `home/.config/systemd/user/<unit>`이다. `load`는 `systemctl --user daemon-reload` → `enable <unit 이름>` → `start <unit 이름>` 순서이고 첫 실패에서 멈춘다. 실패 에러에는 #551부터 명령과 출력 끝부분이 들어간다. `launchd.unitPath()`는 `home/Library/LaunchAgents/<label>.plist`이고 `launchctl bootstrap gui/<uid> <경로>`로 경로를 직접 넘긴다.
- `cmd/issueops/issueopsapp/mcp_service_wiring.go`: `Home: native.Home`(install의 HOME)으로 supervisor를 만든다.
- `internal/adapter/mcpservice/service.go:270-290` `Prepare`가 credential과 unit 파일을 쓰고, `EnsureRunning`(`:429`)이 load를 실행한다. install은 `Prepare` 실패를 `prepare mcp http service: ...`로 감싼다(`cmd/issueops/installcli/install_mcp_transport.go`).
- #550 진단(`.issueops/issues/550/ci-diagnosis.md`): GitHub runner에서 `daemon-reload`는 성공, `enable`이 `Unit file issueops-mcp.service does not exist`로 실패. user manager는 존재하고, 검색 경로가 install HOME과 다르다.
- `skills/pr-review/scripts/mr_context.py:563-580` `_rg_symbol`: rg가 없으면(`FileNotFoundError`) 고정 제외 목록의 `grep -rnwFI`로 찾는다. `.gitignore`를 따르지 않는다. `:581-589` `collect_defs`는 codegraph 결과가 비면 `_rg_symbol`로 넘어간다. `:592-595` `render_defs`는 `codegraph` 플래그만 보고 `codegraph explore` 또는 `rg`로 표시한다. 그래서 grep을 써도, codegraph에서 rg로 넘어가도 표시가 틀린다.
- 테스트: `skills/pr-review/tests/test_context_pack.py:71` `DefsFallbackTest`.
- systemd 동작(계획 리뷰 1라운드에서 문서로 확인): `systemd-analyze(1)`의 `unit-paths`는 "systemd-analyze 자체에 컴파일된 목록을 출력하고 실행 중인 manager와 통신하지 않는다". 호출 프로세스의 환경으로 경로를 계산하므로 HOME을 바꾼 설치에서는 임시 HOME 경로가 그대로 나와 감지에 쓸 수 없다. 실행 중인 manager가 쓰는 목록은 `systemctl --user show -p UnitPath --value`이지만 비어 있거나 없는 디렉터리는 빠진다(같은 HOME의 첫 설치에서 거짓 불일치). manager 환경은 `systemctl --user show-environment`로 읽는다.
- 미확인 가정(Linux에서 확인): user manager 환경 블록에 `HOME`이 들어 있다. 확인은 아래 S1의 CI 회귀 단계가 실제 runner에서 한다.

## 결정: supervisor는 불일치를 사전 감지해 명시 오류를 낸다

- 채택: `Prepare`가 credential과 unit을 쓰기 전에 systemd 어댑터가 실행 중인 user manager의 환경을 조회한다(`systemctl --user show-environment`). 기대 디렉터리는 `XDG_CONFIG_HOME`이 있으면 `$XDG_CONFIG_HOME/systemd/user`, 없으면 `$HOME/.config/systemd/user`다. 쓰려는 디렉터리(`filepath.Clean(filepath.Dir(unitPath()))`)가 기대 디렉터리와 다르고, `systemctl --user show -p UnitPath --value` 목록에도 없으면 아무것도 쓰지 않고 오류를 낸다. 오류는 쓰려는 경로, manager 검색 경로, 해결 방법(`--mcp-transport=stdio` 또는 manager와 같은 HOME에서 설치)을 밝힌다.
- 조회가 실패하거나 manager 환경에 `HOME`이 없으면 판정할 수 없으므로 차단하지 않고 기존 흐름을 따른다. 그러면 기존 load 단계가 명령·출력을 담은 오류로 실패한다. 조회 실패를 근거로 transport를 바꾸지 않는다.
- 기각: `systemctl --user enable <절대 경로>`로 unit을 manager 검색 경로에 연결하는 방법. 임시 HOME 설치가 실제 계정의 systemd 설정에 symlink를 만들고 실제 서비스를 띄워, 이슈의 격리 요구(임시 HOME 설치가 실제 계정 서비스에 영향을 주면 안 됨)와 충돌한다.
- 기각: 불일치 시 stdio로 전환. ADR 2026-10-02가 금지한다.
- launchd는 경로를 직접 넘기므로 같은 실패가 없다. 다만 임시 HOME 설치도 실제 gui 도메인에 서비스를 올린다는 격리 비대칭이 있다. 이 사실은 PR "남은 일"에 기록하고 바꾸지 않는다.

## 작업

### S1. supervisor 사전 감지 (Go)

- `supervisor` 인터페이스(`supervisor.go:36-44`, 패키지 비공개, 구현은 launchd·systemd 둘)에 쓰기 전 확인 단계를 추가한다. launchd 구현은 항상 통과, systemd 구현은 위 결정의 조회·비교를 한다. 경로 비교는 `filepath.Clean` 기준이다.
- `Service.Prepare`의 첫 단계에서 호출한다. 실패하면 credential과 unit을 쓰지 않는다.
- RED→GREEN 테스트(fake Runner): (1) manager HOME이 다르고 UnitPath에도 없으면 오류가 나고 credential·unit 파일이 없다, (2) manager HOME이 같으면 디렉터리가 아직 없어도(첫 설치) 기존과 같이 쓴다, (3) `XDG_CONFIG_HOME`이 있으면 그 기준으로 비교한다, (4) UnitPath에 쓰려는 디렉터리가 있으면 통과한다, (5) 조회 실패나 HOME 부재면 기존 흐름을 유지한다, (6) launchd는 조회하지 않는다.
- 실제 systemd 검증(영구 CI 회귀 단계): `.github/workflows/ci.yml`에 "임시 HOME에서 `--mcp-transport=http` 설치가 HOME 불일치 오류로 실패하고 임시 HOME에 credential·unit이 생기지 않는다"를 확인하는 단계를 추가한다. 쓰기 전에 실패하므로 runner 계정의 user manager에 영향이 없다. 이 단계가 위 미확인 가정(manager 환경의 HOME)을 실제 runner에서 확인한다. `scripts/ci_workflow_test.py`에 이 단계의 존재와 실패 전파를 고정한다.

### S2. pr-review 검색 도구 표시 (Python)

- `_rg_symbol`이 결과와 함께 실제로 쓴 도구(`rg`, `git grep`, `grep`)를 돌려주게 하고, `collect_defs`가 기호마다 도구(`codegraph explore` 포함)를 기록한다. `render_defs`의 source 줄은 실제로 쓴 도구를 쓰인 순서대로 중복 없이 나열한다.
- 테스트: rg를 PATH에서 뺀 환경에서 source 줄이 rg를 말하지 않는다. codegraph 결과가 비어 rg로 넘어간 경우 두 도구가 모두 표시된다.

### S3. grep 대체 경로의 `.gitignore` 준수 (Python)

- rg가 없고 checkout이 git 작업 트리면 `git grep --untracked -n -w -F -I --max-count 3 -e <sym> -- . ':(exclude,glob)**/*lock*' ':(exclude,glob)**/*.min.*' ':(exclude,glob)**/node_modules/**' ':(exclude,glob)**/dist/**'`로 추적·미추적 파일을 찾되 무시 파일은 뺀다(`--untracked`는 무시 파일을 기본으로 제외한다). magic 없는 pathspec의 `*`는 `/`까지 맞춰 디렉터리 이름에 lock이 든 경로 전체를 지우므로 반드시 glob magic을 쓴다. git이 없거나 작업 트리가 아니면 기존 grep을 쓴다. `--max-count` 지원 여부(git 2.38+)는 구현 시 확인하고, 없으면 결과를 파일당 3줄로 자른다.
- 테스트(rg 부재는 `mock.patch`로 결정적으로): (1) 임시 git 저장소에 `.gitignore`로 무시되는 파일과 추적 파일에 같은 기호를 두면 무시 파일이 결과에 없다, (2) `blocks/svc.ts`처럼 디렉터리 이름에 lock이 든 경로의 정의를 찾는다, (3) git 작업 트리가 아니면 기존 grep 경로를 쓴다(기존 `.git` 빈 디렉터리 테스트 유지).

## 적용되는 결정과 주의사항

- `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md:15`: supervisor를 쓸 수 없으면 명시 오류, stdio 자동 전환 금지. S1은 명시 오류를 더 일찍 낸다.
- AGENTS.md 철학: 외부 도구를 설치하거나 readiness gate로 요구하지 않는다. S1은 조회 실패를 차단 사유로 쓰지 않는다.
- `.issueops/CONSTITUTION.md` 제2장: 근본 원인 제거. S1은 HOME 불일치를 쓰기 전에 드러내고, 격리를 깨는 우회를 하지 않는다.
- `.issueops/testing/unit-and-contract.md:115-118`: 스킬 Python 스위트는 skip을 실패로 본다. S2·S3 테스트는 rg·git 유무에 따라 skip하지 말고 PATH를 조작해 결정적으로 만든다.
- `.issueops/cautions/2026-10-03-http.md`: 시간 단언을 쓰지 않는다.
- `.issueops/cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md`: race·self-verify는 저부하에서 실행한다.

## 재사용하는 기존 구현

- S1은 기존 `Runner`(combined output 반환)와 `supervisor` 인터페이스, #551의 명령·출력 오류 형식을 재사용한다. 새 패키지를 만들지 않는다.
- S2·S3는 `_rg_symbol`의 기존 제외 규칙과 subprocess 호출 패턴, `DefsFallbackTest`의 fixture 구성을 재사용한다.

## 성능 영향

S1은 HTTP 설치의 Prepare마다 `systemd-analyze` 한 번을 추가한다(설치 경로, hot path 아님). S3의 `git grep`은 rg가 없을 때만 쓰며 grep보다 빠르거나 비슷하다.

## 하위 호환성과 side effect

- 같은 HOME의 Linux HTTP 설치: 검색 경로에 unit 디렉터리가 있으므로 동작이 같다. macOS: 변경 없음.
- HOME이 다른 Linux HTTP 설치: 이전에는 credential·unit을 쓴 뒤 load에서 실패했고, 이제는 아무것도 쓰지 않고 실패한다. 실패 시 남는 파일이 줄어든다.
- `mcpservice.Status` DTO, error_code, CLI·MCP JSON 출력 변경 없음.
- pr-review 출력의 source 줄 문구가 바뀐다. 이 줄을 파싱하는 코드가 있는지 rg로 확인하고, 있으면 함께 맞춘다.
- 롤백: 결함별 커밋이라 개별 revert 가능.

## 게이트(G1..Gn)

| ID | 종류 | 기준 |
|---|---|---|
| G1 | CHECK | `gofmt -l $(git ls-files '*.go')` 출력 없음 |
| G2 | CHECK | `go vet ./...` 종료 코드 0 |
| G3 | CHECK | `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`(v2.12.2) 0건, `GOOS=linux`도 0건 |
| G4 | CHECK | `go test -race -count=1 ./internal/adapter/mcpservice/...` 통과(S1 테스트 4종 포함) |
| G5 | CHECK | `python3 -m unittest discover -s skills/pr-review/tests` 통과(S2·S3 테스트 포함, skip 0) |
| G6 | CHECK | `go test -race ./... -count=1` 전체 통과 |
| G7 | CHECK | `issueops self-verify --json` 모든 단계 통과 |
| G8 | EXPECT | PR CI verify job 통과(run URL 기록) |
| G9 | EXPECT | PR CI의 HTTP 임시 HOME 회귀 단계가 HOME 불일치 오류를 확인하고 통과 |
| G10 | CHECK | `python3 scripts/ci_workflow_test.py` 통과 |
