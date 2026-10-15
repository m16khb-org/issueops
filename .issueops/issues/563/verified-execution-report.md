# #563 verified-execution report

- lifecycle: `io-9e1b5fa96747`, branch `563-main-ci-lint-test-fixes`, base `bac123aac302fd423673e60be0268c06696f9e13`
- 계획: `.issueops/issues/563/plan.md`(봉인 원본 sha256 `88b8466c542e3c513f4d5f9aa177c140b3a8c55d26cfbfac57217d7d27f3a874`)
- 게이트 원장: `.issueops/issues/563/gates.md`

## 목표와 수용 기준

main CI의 golangci-lint v2.12.2 지적 90건(errcheck 62, govet inline 25, unused 3, 모두 `*_test.go`)을 설정 제외·`//nolint` 없이 테스트 코드에서 고친다.
게이트 결과는 원장의 EVIDENCE가 정본이다.

| 기준 | 게이트 | 결과 |
|---|---|---|
| CI와 같은 `GOOS=linux`, go.mod toolchain, golangci-lint v2.12.2 `run ./...`가 지적 없이 끝난다 | G1 | `0 issues.` |
| gofmt, `go vet ./...`, `go build ./cmd/issueops`, `go test -race ./... -count=1`이 통과한다 | G2 | `ALL_PASS` |
| `.golangci.yml` diff, 추가된 `nolint`, 변경된 production Go 파일이 없다 | G3 | `NO_SUPPRESSION` |
| self-verify의 모든 단계가 통과한다 | G4 | `SELF_VERIFY_OK=True` |
| `internal/adapter/update` 테스트가 Linux 컨테이너에서 `text file busy` 없이 통과한다(CI pull_request run 실패 대응) | G5 | `ETXTBSY_FREE` |

base에서 같은 lint는 `90 issues: errcheck 62, govet 25, unused 3`이었다(구현 전 재현, `/tmp/io-lint-main.txt`).

## 구현 요약

- 반환값을 버리던 단언 helper를 반환 없는 단언과 값을 꺼내는 helper로 나눴다. 판정과 `t.Fatalf`는 그대로다.
  - `internal/adapter/orca`: `requireOrcaCode`는 반환값이 없고, 반환값을 쓰던 33곳은 `requireOrcaError`(내부에서 `requireOrcaCode` 호출)를 쓴다.
  - `internal/application/issueopsowner`: `requirePlanRequired`를 더하고 버리던 7곳을 바꿨다. `requiredPlanError`는 값을 쓰는 4곳이 계속 쓴다.
  - `internal/adapter/outbound/issueopslease`: `persistenceFailure`를 반환 없는 `requirePersistenceFailure`로 바꿨다.
- `internal/adapter/mcpservice/instance_test.go`의 `defer instance.Release()`는 오류를 `t.Errorf`로 보고한다.
- `//go:fix inline` helper 4개(`i64`, `orcaTestUint64`, `handoffTestUint64`, `durationRef`)는 `GOTOOLCHAIN=go1.26.6 go fix -inline ./...`로 호출 49곳을 인라인하고 정의와 directive를 지웠다.
- unused 3건(`explicitActor`, `phaseFlow.prefetches`, `stepLabels`)은 선언을 지웠다.
- 바뀐 파일은 `*_test.go` 19개이며(lint 지적 18개와 아래 `update` 패키지 수정 1개) production 파일, `.golangci.yml`, CLI/MCP 계약은 바뀌지 않았다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/cautions/2026-10-03-native-http-validation-and-lint-toolchain.md` | "실제 errcheck·deprecated API·unused 지적은 코드와 테스트에서 해결한다." | 설정 제외나 `//nolint` 없이 helper 분리·인라인·선언 삭제로 고쳤고 G3이 억제 추가를 막는다. |
| `.issueops/cautions/2026-10-13-mutation-tests-must-stay-on-the-consumer-boundary.md` | "단언은 변이가 들어가면 assertion으로 실패해야 하고, panic에 기대지 않는다." | 나눈 helper는 `t.Fatalf` 단언을 그대로 유지한다. |
| `.issueops/cautions/2026-10-13-mutation-tests-must-stay-on-the-consumer-boundary.md` | "테스트, 헬퍼, 픽스처 이름에는 `mut` 같은 작업 출처를 붙이지 않고 동작을 쓴다." | 새 helper 이름은 `requireOrcaError`, `requirePlanRequired`, `requirePersistenceFailure`처럼 동작을 쓴다. |
| `.issueops/testing/unit-and-contract.md` | "CI runner가 Linux이므로 같은 명령에 `GOOS=linux`를 추가해 실행한다" | G1 lint와 별도 `GOOS=linux go vet ./...`를 Linux 대상으로 실행했다. |
| `.issueops/cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md` | "CI 재현은 `GOOS=linux golangci-lint run ./...`와 `GOOS=linux go vet ./...`를" | 두 명령을 모두 실행했다(G1, 별도 vet). |

`.issueops/ARCHITECTURE.md`, `.issueops/CONVENTIONS.md`, `.issueops/ADR.md`는 대조했고 적용할 규칙이 없었다. 테스트 파일만 바뀌어 layer 의존 규칙에 걸리지 않는다.

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/CONSTITUTION.md` | "Go 코드 변경은 영향 범위에 맞게 `go test ./... -count=1`, `go test -race ./...`, `go build ./cmd/issueops`를 실행한다." | 지킴 | 원장 G2(race 전체 테스트, build)와 G4(self-verify의 `go test ./... -count=1`) |
| `.issueops/CONSTITUTION.md` | "임시 우회, 출력 문구 보정, 테스트 기대값 완화, 문서만 변경하는 대응은 근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거가 있을 때만 허용한다." | 지킴 | `.golangci.yml`·`//nolint`·`_ = ` 억제 없이 helper 분리·인라인·선언 삭제로 지적 원인을 제거했다(G3 `NO_SUPPRESSION`) |
| `.issueops/cautions/2026-10-03-native-http-validation-and-lint-toolchain.md` | "실제 errcheck·deprecated API·unused 지적은 코드와 테스트에서 해결한다." | 지킴 | `internal/adapter/orca/client_validation_test.go`의 `requireOrcaCode`·`requireOrcaError`, `internal/application/selfverify/coverage_extra_test.go`의 `stepLabels` 삭제 등 |
| `.issueops/conventions/go-and-packages.md` | "production 진입점에서 호출하지 않는 테스트용 위임 함수·타입·상수는 `*_test.go`에 둔다." | 지킴 | 새 helper 세 개 모두 같은 `*_test.go` 파일 안에 있고 production 파일은 바뀌지 않았다(G3) |

`.issueops/CONVENTIONS.md`와 `.issueops/ARCHITECTURE.md` 색인은 열어 대조했고 테스트 helper 형태나 이름, layer 의존에 걸리는 규칙이 없다. `.issueops/CAUTIONS.md` 색인에서도 이번 diff가 어기는 항목이 없다.

## ai-slop-clean

변경 범위는 지적된 `*_test.go` 18개와 CI에서 드러난 `internal/adapter/update/runtime_test.go`다. 정리 대상은 구현이 만든 것과 lint가 지목한 선언에 한정했다.

| 분류 | 제거한 것 |
|---|---|
| dead-code | 호출이 0이 된 `//go:fix inline` helper 4개와 directive, 읽거나 쓰는 곳이 없던 `explicitActor`·`phaseFlow.prefetches`·`stepLabels` |

- 측정: `git diff --numstat` 기준 추가 94줄, 삭제 106줄. 새 helper(`requireOrcaError`, `requirePlanRequired`, `requirePersistenceFailure`)는 기존 판정 본문을 옮긴 것이라 중복 판정 로직이 없다. 정리 뒤 `git diff --check`는 통과했다.
- 남긴 것: `requireOrcaError`와 `requiredPlanError`의 `errors.AsType` 재호출은 단언 판정을 한 곳(`requireOrcaCode`, `requirePlanRequired`)에만 두려는 계획 설계라 합치지 않았다.
- 범위 밖 발견: 없음.
- 적용한 프로젝트 규칙: `.issueops/CONVENTIONS.md`와 `.issueops/conventions/go-and-packages.md`를 대조했고 테스트 helper 형태나 이름에 관한 규칙이 없어 이 정리를 정한 문서 규칙은 없다.

## CI에서 드러난 update 패키지 결함

PR #564의 pull_request CI(run `38706548073`)가 `Race test all packages`에서 두 번 연속 실패했다.
`internal/adapter/update`의 `TestRuntimeInstall/runs_script_with_args,_env,_stdin_in_root`가 `fork/exec .../install-native.sh: text file busy`로 끝났다.
같은 head의 push run은 통과했다. 이 결함은 lint 실패에 가려 있던 것이고, #563 성공 기준(lint 이후 단계까지 CI success)을 지키기 위해 이 사이클에서 고친다.

- 원인: `writeScript`가 `os.WriteFile`로 스크립트를 쓴 직후 같은 테스트가 exec한다. 그 사이 다른 병렬 테스트의 fork가 쓰기 fd를 상속하면 Linux가 exec에 `ETXTBSY`를 돌려준다. macOS에서는 재현되지 않는다.
- 재현: Linux arm64용 test binary를 `ubuntu:24.04` 컨테이너에서 `-test.count=300 -test.cpu=4 -test.shuffle=on`으로 실행했다. 수정 전에는 `TestProcessTableRead`, `TestRuntimeInstall`, `TestRuntimeList`가 `text file busy`로 실패했다. CI가 지목한 한 곳만이 아니라 스크립트를 쓰고 곧바로 실행하는 테스트 세 개가 모두 같은 패턴이다.
- 수정: 이 세 테스트와 하위 테스트에서 `t.Parallel`을 뺐다. 직렬 테스트는 병렬 테스트가 재개되기 전에 끝나고, 병렬로 남은 테스트(`TestParseMCPProxyProcessSnapshot`, `TestRuntimeProcessBasics`)는 fork하지 않으므로 창이 사라진다. `writeScript`에 이유를 주석으로 남겼다.
- 확인: 같은 컨테이너 실행을 `-test.count=1500 -test.cpu=2,8,16 -test.shuffle=on`으로 늘려 `text file busy` 없이 통과했다. 원장 G5가 이 실행을 고정한다.
- 같은 패턴을 다른 패키지에서 찾아본 결과: 실행 파일 권한(0o755)을 쓰고 `t.Parallel`을 쓰는 테스트 파일이 `internal/adapter/issueops`와 `internal/adapter/hostprobe`에도 있다. `hostprobe` 테스트 binary를 같은 컨테이너에서 돌렸더니 `TestChildHostSmoke*`가 거의 매번 실패했지만 `text file busy`가 아니라 컨테이너에 저장소 파일이 없어서 생긴 실패로 보여 ETXTBSY 여부는 판정하지 못했다. 이 사이클은 `update` 패키지만 고쳤다.

## 성능 측정

production hot path가 없다. 인라인한 `new(int64(250))` 같은 표현은 helper 호출과 같은 할당을 하므로 측정하지 않았다.

## Side effect

- 파일: `*_test.go` 19개 수정, `.issueops/` 아래 caution 한 건과 색인 한 줄, `.issueops/issues/563/` 아래 추적 사본과 이 report.
- 원격·durable state: 구현 단계에서는 없다. 커밋·push·draft PR 발행은 pr 단계에서 한다.
- #560(Orca 연동 제거)이 `internal/adapter/orca/*_test.go`를 지울 때 base 동기화에서 modify/delete 충돌이 날 수 있고 삭제 쪽을 택하면 된다.

## 검증 환경 메모

- 이 머신의 python3에 pydantic·typer가 없어 임시 venv(`uv venv --python 3.13` + `scripts/python_test_requirements.txt`)를 PATH 앞에 두고 G4를 실행했다.
- `gates check`는 `--cwd`와 `--workspace-root`를 주지 않으면 `cwd_required; workspace_root_required`로 거부된다. G1이 `go install`로 golangci-lint를 받으므로 `--network`를 줬다.
- 구현 리뷰 전에 기준 커밋을 지정한 `self-verify --base-ref`를 한 번 더 실행했다. 27단계가 모두 통과했고 risk QA는 `go test -race ./... -count=1`과 `go vet ./...`을 재사용 없이 실행했다(316.6초).

## 원격 확인

- 커밋 `38d76e01`을 push한 뒤 PR을 발행하기 전에 HEAD 커밋의 CI run `38705119478`을 `gh run list --commit`으로 찾아 끝까지 확인했다. `verify` job의 Format check, Vet, Lint, `.golangci.yml` schema 검증, Shell lint, 스킬 frontmatter 검증, Build, Race test, 결정적 self-verify, HTTP MCP install 단계가 모두 success다. 이 run은 이 PR의 브랜치 push이며 main의 base 커밋에서는 Lint 단계에서 멈췄던 job이다.
- draft PR #564를 `remote create-pr`로 만들고 `remote verify-artifact`로 확인했다. head `563-main-ci-lint-test-fixes`, base `main`, 라벨 `bug`, 담당자 `m16khb`, 본문 `Closes #563`이 provider readback과 일치한다.
- PR을 만든 뒤 `pull_request` run `38706548073`이 `Race test all packages`에서 두 번 연속 실패했다(위 `CI에서 드러난 update 패키지 결함`). 수정 커밋 `87ba7cab`을 push하고 같은 head의 두 run을 모두 끝까지 확인했다: `push` run `38720840743`과 `pull_request` run `38720847359`가 모두 success이고 실패하거나 건너뛴 단계가 없다.
- PR 본문은 `remote sync-pr`(preview의 `expected_body_sha256`을 넘긴 confirm)로 갱신했고 readback에서 `Closes #563`, 두 run 번호가 보이며 `in_sync`다. 이후 `remote verify-artifact`로 draft 상태, head, 라벨 `bug`, 담당자 `m16khb`, base `main`을 다시 확인했다.

## 실패와 재시도

- 첫 `gates check`는 `--cwd`·`--workspace-root` 누락으로 정책에 거부되어 4개 게이트가 실행되지 않았다. 두 플래그를 주고 다시 실행해 4개 모두 met이 됐다. 코드 변경은 없었다.
- 재개 뒤 G1~G5 재실행에서 G4가 `SELF_VERIFY_OK=False`였다. 원인은 이 변경이 아니라 이 머신의 `~/.omo`가 사라져 `native integration` 단계가 `missing ~/.omo/agent/skills/...`로 실패한 것이다(첫 실행 때는 있었다). CI와 같은 방식(임시 HOME에 `install-native.sh --skip-build --path-mode=skip --mcp-transport=stdio` 뒤 self-verify, GOCACHE·GOMODCACHE·GOPATH는 실제 값)으로 G4 CHECK를 바꿔 다시 실행했고 통과했다.
- 재개 뒤 첫 G1 증거가 `go: downloading ...` 줄이어서 원장을 다시 열고 G1을 재실행했다. 같은 명령이 `0 issues.`를 출력했다.
- `gates check`는 이미 met인 게이트를 다시 실행하지 않는다. 테스트 코드가 바뀐 뒤에는 G1~G4의 체크박스와 EVIDENCE를 되돌려야 실제로 다시 돈다.
- `.issueops/TESTING.md`의 "다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 첫 게이트부터 전체 재실행한다."에 따라 G1~G5를 모두 되돌려 한 번의 `gates check`로 다시 실행했다. 다섯 게이트가 한 run에서 모두 met이고 원장 EVIDENCE는 그 run의 출력이다. 위 부분 재실행 결과는 보고에 쓰지 않는다.
- 구현 리뷰 전의 `self-verify --base-ref`(임시 HOME)가 한 번 `risk QA tier`의 race 테스트에서 실패했다. 출력이 잘려 실패한 테스트는 알 수 없었다. 같은 `go test -race ./... -count=1`을 임시 HOME에서 다섯 번 더 돌렸더니 네 번은 통과했고 한 번은 `cmd/issueops/issueopscli`의 `TestCLIIssueOpsPhaseAdvanceToPRBlockedByChildren`(`native session process receipt is not in the local process ancestry`)과 `cmd/issueops/issueopscli/remotecmd`의 `TestChildCLIStaleOperationRequiresCurrentHolderReconcile`(`context deadline exceeded`)가 실패했다. 두 테스트는 이 변경이 건드리지 않은 파일이며 프로세스 조회·시간 제한에 기대므로 이 머신의 부하에서 확률적으로 실패하는 것으로 보이지만 원인은 확정하지 못했다. 이 사이클은 고치지 않았다. 같은 명령 재실행의 `self-verify --base-ref bac123aa`는 27단계 모두 통과했다. GitHub Actions(Linux)에서 이 사이클 PR의 race 단계가 실패한 것은 `update` 패키지 한 곳뿐이었다.
