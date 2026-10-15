# 계획: main CI lint 실패 90건을 테스트 코드에서 고친다 (#563)

- lifecycle ID: `io-9e1b5fa96747`
- 이슈: https://github.com/m16khb-org/issueops/issues/563
- 브랜치: `563-main-ci-lint-test-fixes`, base `main` @ `bac123aac302fd423673e60be0268c06696f9e13`
- 사용자 요청 범위: main의 lint 부채를 별도 사이클에서 고쳐 main CI를 다시 통과시킨다. #561 사이클의 sync-base·reseed·머지는 이 사이클이 머지된 뒤 #561 쪽에서 따로 진행하며 이 계획의 범위가 아니다.
- 인계: 계획 리뷰 통과 뒤 `execution prepare --mode direct`로 워크트리를 만들고 환경별 자동 세션 인계 규칙을 적용한다.
- 승인된 종료점: PR 생성과 `execution complete`까지다. 머지는 `issueops-merge`로 따로 확인받는다.

## 문제

`GOOS=linux`, toolchain `go1.26.6`, golangci-lint v2.12.2로 `run ./...`를 실행하면 base 커밋에서 90건이 나온다(로컬 재현 `/tmp/io-lint-main.txt`, CI run 38562720520과 결과가 같다). 지적은 모두 `*_test.go`에 있다.

| 종류 | 파일 | 건수 | 원인 |
|---|---|---|---|
| errcheck | `internal/adapter/orca/client_validation_test.go` | 45 | `requireOrcaCode`가 `*port.OrcaError`(error 구현, `internal/port/orca.go:25`)를 돌려주는데 반환값을 버린다 |
| errcheck | `internal/adapter/orca/execution_inspect_test.go` | 4 | 같음 |
| errcheck | `internal/adapter/orca/execution_stage_test.go` | 3 | 같음 |
| errcheck | `internal/application/issueopsowner/coverage_test.go` | 7 | `requiredPlanError`(`coverage_test.go:49`)가 `*domain.OwnerPlanRequiredError`를 돌려주는데 버린다 |
| errcheck | `internal/adapter/outbound/issueopslease/resume_repository_guards_test.go` | 2 | `persistenceFailure`(`:44`)가 `*leasecontract.Failure`를 돌려주는데 버린다 |
| errcheck | `internal/adapter/mcpservice/instance_test.go:122` | 1 | `defer instance.Release()`가 error를 버린다 |
| govet inline | `internal/adapter/trace/host_usage_test.go`, `host_usage_boundary_test.go` | 14 | `//go:fix inline` helper `i64`(`host_usage_test.go:27-28`) 호출 |
| govet inline | `internal/adapter/orca/execution_test.go`, `execution_stage_test.go` | 5 | `orcaTestUint64`(`execution_test.go:1604-1605`) 호출 |
| govet inline | `cmd/issueops/issueopsapp/*_wiring_test.go` 3개 파일 | 5 | `handoffTestUint64`(`handoff_delivery_wiring_test.go:812-813`) 호출 |
| govet inline | `internal/adapter/outbound/sqlstore/span_observer_test.go` | 1 | `durationRef`(`:93-94`) 호출 |
| unused | `cmd/issueops/issueopscli/executioncmd/execution_commands_test.go:24` | 1 | `explicitActor` var. 같은 파일은 `explicitActorArgs`만 쓴다 |
| unused | `internal/application/issueopscycle/phase_flow_test.go:23` | 1 | `prefetches` 필드. 패키지에서 읽거나 쓰는 곳이 없다 |
| unused | `internal/application/selfverify/coverage_extra_test.go:150` | 1 | `stepLabels` 함수. 호출하는 곳이 없다 |

`requireOrcaCode` 호출은 함수 정의를 빼고 85곳이다. 이 중 52곳이 반환값을 버리고(errcheck 52건) 33곳이 `.Detail`·`.Invoked` 등 반환값을 쓴다(예: `client_validation_test.go:183`, `runner_failure_test.go:16`). `requiredPlanError`는 11곳 중 7곳이 버리고 4곳(`coverage_test.go:73,84,138,713`)이 쓴다. `persistenceFailure`는 2곳 모두 버린다.

## 설계

1. **반환값을 버리는 단언 helper는 반환하지 않는 단언으로 나눈다.**
   - orca: `requireOrcaCode(t, err, code)`를 반환값 없는 단언으로 바꾼다. 반환값을 쓰는 33곳은 새 `requireOrcaError(t, err, code) *port.OrcaError`를 부른다. `requireOrcaError`는 `requireOrcaCode`로 단언한 뒤 `errors.AsType`으로 값을 꺼내 돌려준다. 그래서 판정 로직은 `requireOrcaCode` 한 곳에만 있고 반환값을 버리는 호출은 남지 않는다.
   - issueopsowner: 반환값 없는 `requirePlanRequired(t, err)`를 더하고 버리던 7곳을 그쪽으로 바꾼다. `requiredPlanError`는 `requirePlanRequired` 뒤에 값을 꺼내 돌려주는 형태로 바꾸고 4곳은 그대로 쓴다.
   - issueopslease: 반환값을 쓰는 곳이 없으므로 `persistenceFailure`를 반환 없는 `requirePersistenceFailure`로 바꾸고 2곳 이름을 고친다.
2. **`defer instance.Release()`** 는 같은 파일 `:51`, `:78`, `:98`처럼 오류를 검사한다. defer 안에서 `t.Errorf`로 보고한다.
3. **`//go:fix inline` helper 4개**는 `GOTOOLCHAIN=go1.26.6 go fix -inline ./...`로 호출 49곳(i64 38, orcaTestUint64 5, handoffTestUint64 5, durationRef 1)을 `new(int64(250))`, `new(uint64(0))`, `new(time.Duration(0))` 형태로 인라인한다. 그 뒤 호출이 0이 된 helper 정의와 directive를 지운다. 이 directive는 저장소 전체에서 이 4곳에만 있다(`grep -rn "go:fix inline"`). base에서 `go fix -inline -diff ./...`를 실행하면 바뀌는 파일은 위 표의 테스트 파일 8개뿐이다.
4. **unused 3건**은 선언을 지운다.
5. `.golangci.yml`과 `//nolint`는 건드리지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제5장 하네스 검증 불변식: "Go 코드 변경은 영향 범위에 맞게 `go test ./... -count=1`, `go test -race ./...`, `go build ./cmd/issueops`를 실행한다." → G2가 vet·build·race 테스트·gofmt를, G4의 self-verify가 `go test ./... -count=1`을 실행한다.
- `.issueops/TESTING.md:28-29`: "단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." 및 `.issueops/testing/self-verification.md:70`: self-verify step 목록이 "`go test ./... -count=1`"을 포함한다 → G4가 self-verify를 실행한다. CI의 self-verify 단계는 7ad771aa 이후 lint 실패로 한 번도 돌지 않았으므로 숨은 실패가 있으면 이 게이트에서 드러난다.
- `.issueops/cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md:11`: "게이트 CHECK 한도는 명령 정책이 900초로 막으므로(--timeout-seconds가 그보다 크면 timeout_exceeds_15m으로 거부) go test ./...·race처럼 긴 게이트는 머신 부하가 높으면 실패 테스트 없이 check timed out으로 미충족된다." → race 전체 테스트(G2)와 self-verify(G4)를 다른 게이트로 나누고 `gates check`에 `--timeout-seconds 900`을 준다. 부하로 시간이 초과되면 부하가 가라앉은 뒤 같은 원장으로 다시 실행한다.
- `.issueops/ARCHITECTURE.md`: 대조했으나 없음. 이 변경은 테스트 파일만 바꾸며 layer 의존 규칙(`internal/architecture/dependency_test.go`)은 production import graph만 본다.
- `.issueops/CONVENTIONS.md`: 대조했으나 없음. 테스트 helper 형태에 관한 규칙이 없다.
- `.issueops/cautions/2026-10-03-native-http-validation-and-lint-toolchain.md`(`.issueops/CAUTIONS.md` 색인이 가리키는 기록): "실제 errcheck·deprecated API·unused 지적은 코드와 테스트에서 해결한다." → 설정 제외·`//nolint` 없이 코드에서 고친다(G3).
- `.issueops/cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md`: "CI 재현은 `GOOS=linux golangci-lint run ./...`와 `GOOS=linux go vet ./...`를 함께 돌린다" → G1이 `GOOS=linux`로 lint한다.
- `.issueops/cautions/2026-10-07-ci-home-runner-rg-worktree-lint-cache-binary.md`: "결과에 다른 worktree 경로가 보이면 golangci-lint cache clean 뒤 다시 실행한다." → 워크트리에서 lint 결과에 다른 경로가 보이면 cache를 지우고 다시 실행한다.
- `.issueops/cautions/2026-10-13-mutation-tests-must-stay-on-the-consumer-boundary.md`: "단언은 변이가 들어가면 assertion으로 실패해야 하고, panic에 기대지 않는다." 및 "테스트, 헬퍼, 픽스처 이름에는 `mut` 같은 작업 출처를 붙이지 않고 동작을 쓴다." → 나눈 helper는 `t.Fatalf` 단언을 그대로 유지하고 이름은 동작(`require...`)으로 짓는다.
- `.issueops/ADR.md`: 대조했으나 없음. lint·테스트 helper에 관한 결정이 없다(`adr/`에서 `golangci|errcheck|lint`를 grep해 확인).
- `.issueops/TESTING.md` → `.issueops/testing/unit-and-contract.md:33-36`: "로컬 린터도 `go.mod`의 toolchain을 사용한다" 및 "CI runner가 Linux이므로 같은 명령에 `GOOS=linux`를 추가해 실행한다" → G1이 `go list -m -f '{{.GoVersion}}'`로 읽은 toolchain과 `GOOS=linux`를 쓴다.
- `.issueops/cautions/2026-09-01-cleanup-deadlock-advanced-remote-branch.md`(route 추가): 대조했으나 없음. 머지·정리 단계 주의라 이 계획의 구현에는 걸리지 않는다.

## 재사용하는 기존 구현

- 단언 판정은 기존 helper 본문(`errors.AsType` + `t.Fatalf`)을 그대로 옮긴다. 새 판정 로직은 만들지 않는다.
- 인라인은 Go 1.26 `go fix`의 `inline` analyzer(`go tool fix help`에 `inline` 항목이 있다)를 쓴다. 호출 49곳을 손으로 바꾸지 않는다.
- `Release` 오류 검사는 같은 파일 `instance_test.go:51-53`의 형태를 따른다.

## 성능 영향

테스트 코드만 바뀌고 production hot path는 없다. 측정 계획은 없다. `new(int64(1))` 같은 인라인 결과는 helper 호출과 같은 할당을 한다.

## 하위 호환성과 side effect

- CLI JSON·MCP schema·golden·record schema·provider body 계약은 바뀌지 않는다. production `.go` 파일은 바뀌지 않는다(G3이 diff를 확인한다).
- `internal/adapter/orca/*_test.go`는 #560(Orca 연동 제거, 계획 단계, 워크트리 `issueops.worktrees/560-remove-orca-integration`)이 나중에 지울 파일이다. #560이 이 변경 뒤에 base를 동기화하면 이 파일들에서 modify/delete 충돌이 날 수 있고, 그때는 삭제 쪽을 택하면 된다.
- 롤백은 이 PR의 squash 커밋을 revert하는 것으로 끝난다. 데이터 변경은 없다.

## 게이트

- G1: CI와 같은 Linux·go.mod toolchain·golangci-lint v2.12.2 lint가 지적 없이 끝난다 | CHECK: python3 -c "import os,subprocess,sys,tempfile; d=tempfile.mkdtemp(); v=subprocess.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True,check=True).stdout.strip(); subprocess.run(['go','install','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2'],env=dict(os.environ,GOBIN=d),check=True); r=subprocess.run([d+'/golangci-lint','run','./...'],env=dict(os.environ,GOOS='linux',GOTOOLCHAIN='go'+v),capture_output=True,text=True); out=r.stdout.strip(); print(out.splitlines()[-1] if out else r.stderr.strip()[-300:]); sys.exit(r.returncode)" | EXPECT: 0 issues.
- G2: gofmt·go vet·go build·race 전체 테스트가 모두 통과한다 | CHECK: python3 -c "import subprocess,sys; fmt=subprocess.run(['gofmt','-l','cmd','internal'],capture_output=True,text=True).stdout.strip(); cmds=[['go','vet','./...'],['go','build','-o','/dev/null','./cmd/issueops'],['go','test','-race','./...','-count=1']]; bad=[' '.join(c[:2]) for c in cmds if subprocess.run(c,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode]; bad=(['gofmt'] if fmt else [])+bad; print('ALL_PASS' if not bad else 'FAIL '+','.join(bad)); sys.exit(1 if bad else 0)" | EXPECT: ALL_PASS
- G3: lint 설정·nolint 억제가 늘지 않고 production Go 파일이 바뀌지 않는다 | CHECK: python3 -c "import subprocess,sys; run=lambda a: subprocess.run(a,capture_output=True,text=True,check=True).stdout; base=run(['git','merge-base','HEAD','origin/main']).strip(); assert base; cfg=run(['git','diff',base,'--','.golangci.yml']); diff=run(['git','diff',base,'--unified=0','--','*.go']).splitlines(); sup=[l for l in diff if l.startswith('+') and 'nolint' in l]; prod=[p for p in run(['git','diff','--name-only',base,'--','*.go']).split() if not p.endswith('_test.go')]; bad=bool(cfg or sup or prod); print('SUPPRESSION_OR_PROD_CHANGE' if bad else 'NO_SUPPRESSION'); sys.exit(1 if bad else 0)" | EXPECT: NO_SUPPRESSION
- G4: self-verify의 모든 단계(`go test ./... -count=1` 포함)가 통과한다 | CHECK: python3 -c "import subprocess as s,json; b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True); p=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],capture_output=True,text=True); d=json.loads(p.stdout or '{}'); print('SELF_VERIFY_OK='+str(b.returncode==0 and bool(d.get('ok'))))" | EXPECT: SELF_VERIFY_OK=True

G1이 이슈의 완료 조건(CI lint 단계가 지적 없이 끝난다)을 실제 lint 실행으로 직접 관측한다. G2와 G4는 CI가 lint 뒤에 실행하는 Build·race 테스트·self-verify를 로컬에서 같은 명령으로 관측한다. `bin/`은 `.gitignore` 대상이라 G4의 빌드 산출물은 변경 집합에 들어가지 않는다. 로컬 `python3`에 self-verify의 Python script tests 의존성(`scripts/python_test_requirements.txt`)이 없으면 CI처럼 `uv venv --python 3.13`에 설치하고 그 `bin`을 PATH 앞에 둔 뒤 실행한다(#558 검증 기록과 같은 방식).

GitHub Actions `verify` job의 결과는 원장 게이트로 두지 않는다. 그 run은 커밋을 push해야 생기고, 결과를 원장 EVIDENCE에 적으면 head가 다시 바뀐다. 대신 PR 단계에서 push 뒤 PR을 발행하기 전에 `gh pr checks`와 run URL로 lint 이후 단계까지 모두 success인지 확인해 PR 본문과 완료 기록에 남긴다. 실패하면 구현 단계로 돌아가 원인을 고친다.

## 작업 순서

1. 워크트리에서 base 기준 lint 90건을 재현한다.
2. 설계 1~4를 적용한다.
3. `gofmt -w`로 바뀐 파일을 정리한다.
4. G1~G4를 `issueops gates check --write --timeout-seconds 900`으로 채운다.
5. 커밋은 `.issueops/COMMIT_POLICY.md`를 따른다.
