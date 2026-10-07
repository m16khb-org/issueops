# 계획: #550 main CI install 실패와 #548 후속 정비 후보

- lifecycle ID: `io-7426b49dc042`
- issue: https://github.com/m16khb-org/issueops/issues/550
- branch: `550-ci-install-and-followups` (base `main`, 봉인 SHA는 record의 `branch_prepare.base_sha`)
- 요청 범위: 사용자가 `/issueops`로 사이클 진행을 요청했다. 종료점은 draft PR 발행과 `execution complete`다. merge·cleanup은 범위 밖이다.
- 세션: `execution prepare --mode direct`로 워크트리를 만든 뒤 환경별 자동 세션 인계(Orca ready면 같은 worktree의 새 세션, owner model `claude-opus-5-5`, effort `high`)를 적용한다.

## 목표와 비목표

목표: main CI의 verify job이 install·self-verify까지 통과하게 하고, P2 후보마다 수정·유지·분리 결론을 근거와 함께 남긴다.

비목표: CLI·MCP 공개 출력 계약 변경, `issueops*` 패키지 병합, PROJECT_AUDIT의 CP3·W6 재개, Linux 기본 transport 정책 변경(아래 결정 참조).

## 사실 확인 결과

- CI 실패 경로: `.github/workflows/ci.yml:134`가 임시 HOME에서 `./scripts/install-native.sh --skip-build --path-mode=skip`을 실행한다. Linux 기본 transport는 http(`cmd/issueops/issueopsapp/install_wiring.go:59-64`, `scripts/install-native.sh:176-179`)이고, `cmd/issueops/installcli/install_mcp_transport.go:66-69`가 `EnsureRunning` 실패를 `mcp http service is not ready ...`로 돌려준다. supervisor는 `systemctl --user daemon-reload/enable/start`를 실행한다(`internal/adapter/mcpservice/supervisor.go:137-144`). service load 실패 지점은 `internal/adapter/mcpservice/service.go:338`.
- 같은 메시지가 main CI에서 2026-10-03(shared HTTP MCP 도입 `1c75cf65` 직후)부터 반복됐고, PR #549 CI에서도 같았다.
- 수용된 ADR `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md:15`: "supervisor가 없거나 실행되지 않으면 명시적 service 오류를 내고 stdio로 조용히 전환하지 않는다."
- self-verify 안의 install 확인은 `install --dry-run --project-local`이다(`internal/adapter/verification/probe/installdryrun/validation_install_dry_run.go:45`). dry-run은 supervisor를 실행하지 않고 credential만 읽는다(`install_mcp_transport.go:54-58`).
- P2 후보 사실(서브에이전트 조사, 구현 시 해당 파일을 다시 읽고 확인):
  - `internal/adapter/codex/install_hooks.go:145` `codexLifecycleHookEvents`는 업그레이드 시 예전 issueops 소유 hook 그룹을 지우는 목록이다(`:115-140`). 설치하는 이벤트는 SessionStart 하나(`:70-77`). Claude installer도 같은 구조(`internal/adapter/claude/install_hooks.go:90-98`). 테스트가 고정(`internal/adapter/codex/install_test.go:85-110`).
  - `cmd/issueops/issueopscli/benchmarkartifact/issueops_benchmark_artifact.go:101,103` 예시 문구의 경로는 존재하지 않는다. 채점은 `owns`, `worker|task` 키워드만 본다(`internal/domain/issueopsbenchmark/issueops_benchmark_score.go:46`, `issueops_quality_checks.go:19-37`). golden·digest 고정 없음.
  - readability 선형성 테스트(`internal/domain/artifactreadability/readability_test.go:282-319`)의 본문은 줄바꿈이 없어 줄 단위 경로(`addLineFindings`)가 커지지 않는다. `Check`에 이차 경로는 확인되지 않았다.
  - 직접 테스트가 없는데 분기가 많은 패키지: `internal/domain/issueopsorphancleanup`(211줄, 분기 37, 순수 함수), `internal/application/issueopsbodysync`(211줄, 분기 30), `internal/domain/nativehost`(24줄, 순수). 나머지 0% 패키지는 데이터·wiring이거나 간접 테스트가 있다.
  - quality inspect 커버리지 캐시(`internal/adapter/outbound/quality/coverage.go:108-200`)는 HEAD·diff·untracked·환경 fingerprint가 같을 때만 적중하고, 실패하면 캐시하지 않는다. self-verify 결과 재사용은 `.issueops/testing/self-verification.md:117`이 막는다.
  - Python skip 정책은 의도된 것이다(`.issueops/testing/unit-and-contract.md:115-118`, `scripts/python_suite_runner_test.py:70,114-129`). pydantic 로컬 설치는 문서화돼 있다(`.issueops/testing/unit-and-contract.md:123`).
  - production `os.IsNotExist` 69곳 vs `errors.Is(..., fs.ErrNotExist|os.ErrNotExist)` 40곳. `os.IsNotExist`는 wrapped error를 풀지 못한다. 컨벤션 문서에 규정 없음.
  - 대형 문서 둘은 ADR·증거 문서가 참조한다(`.issueops/adr/2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md:10`, `quality-audit-2026-10-03.md:15`).

## 작업

### P1. CI install (진단 커밋과 stdio 커밋을 순서대로 분리)

- 원인을 먼저 드러낸다(진단 커밋). `internal/adapter/mcpservice/supervisor.go`의 systemd·launchd `load`가 `Runner`의 combined output(`supervisor.go:13-14`, 실행은 `cmd/issueops/issueopsapp/mcp_service_wiring.go:51`)을 버리고 exit status만 남긴다(`supervisor.go:137-144`). 실패한 명령과 출력(UTF-8 경계를 지키는 길이 제한, 기존 `internal/domain/policy/text_bound.go`의 `TailBytes`/`TruncateBytes` 재사용)을 에러에 담는다. focused 테스트는 fake Runner가 출력과 에러를 돌려줄 때 에러 문자열에 명령과 출력이 들어가는지 확인한다(RED→GREEN).
- 진단 커밋을 먼저 단독으로 push하고, 그 push가 만든 CI run이 끝날 때까지 다음 push를 하지 않는다(`ci.yml`의 `concurrency.cancel-in-progress: true`가 진행 중인 run을 취소하므로). 이 run은 install 단계에서 실패할 것이며, 실패 로그에서 실패한 systemctl 명령과 출력을 읽어 원인을 판정한다. `gh run view <run id> --log-failed`에서 `load mcp service job` 뒤의 명령·출력을 PR 본문 근거로 남긴다.
- 실패 원인 후보는 둘이다. (a) runner에 user systemd manager가 없다. (b) CI가 임시 HOME에 unit을 쓰는데(`supervisor.go:122-123`, `mcp_service_wiring.go:47`의 `Home: native.Home`) user manager는 자기 HOME에서 unit을 찾는다. PR CI에서 위 진단 출력으로 어느 쪽인지 확인해 PR과 이슈 완료 기록에 남긴다. (b)로 확인되면 HOME을 바꿔 설치하는 모든 Linux 환경의 결함이므로 그 사실을 후속으로 기록한다(이번 범위에서 고치지 않는다).
- 원인을 기록한 뒤에야 stdio 커밋을 push한다. `.github/workflows/ci.yml:134`의 install 호출에 `--mcp-transport=stdio`를 붙인다. 주석은 원인을 특정하지 않는다: "임시 HOME의 CI runner에서는 HTTP MCP supervisor가 unit을 띄울 수 없고, ADR 2026-10-02는 stdio 자동 전환을 금지하므로 명시적으로 stdio를 고른다".
- `scripts/ci_workflow_test.py`에 그 플래그를 고정하는 단언을 추가한다.
- `.issueops/operations/install.md`의 transport 절에 "supervisor가 unit을 띄울 수 없는 Linux 환경(user systemd manager가 없거나 HOME을 격리한 설치 등)에서는 `--mcp-transport=stdio`를 명시한다"를 추가한다. PR CI에서 원인이 확인되면 문장을 그 원인에 맞춰 좁힌다.
- 진단 결과(2026-10-07, run 37595246857): `systemctl --user daemon-reload`는 성공하고 `enable`이 `Unit file issueops-mcp.service does not exist`로 실패했다. 판정은 (b)이며 `.issueops/issues/550/ci-diagnosis.md`에 기록했다. stdio 커밋 뒤의 run 37596145318에서는 install이 통과했다.
- 그 run에서 install에 가려져 있던 self-verify의 Python 단계가 처음 실행돼 `skills/pr-review/tests/test_context_pack.py`의 `test_rg_fallback_lists_definition_and_callers` 하나가 실패했다. GitHub runner에는 ripgrep이 없고, `skills/pr-review/scripts/mr_context.py`의 `_rg_symbol`은 `rg` 실행이 `OSError`로 실패하면 빈 목록을 돌려준다. 그래서 정의와 호출부가 "not found"로 보고된다. rg가 없는 PATH로 로컬에서 같은 실패를 재현했다. 리뷰 대상 저장소의 정의를 놓치면 pr-review 결과가 조용히 나빠지므로 테스트를 skip하거나 CI에 rg를 설치하는 대신, `rg`를 찾을 수 없을 때 `grep -rnwFI`로 찾게 한다. 명시 glob(node_modules, dist, `*lock*`, `*.min.*`)과 `.git`·바이너리 제외는 rg와 맞추고, `.gitignore`와 숨김 경로 규칙은 맞추지 않는다(rg가 없을 때 무시 대상 산출물이 결과에 남을 수 있는 것은 받아들인 한계다). focused 테스트는 `rg` 실행이 `FileNotFoundError`를 내는 상황에서도 정의와 호출부가 나오고, node_modules·`.git`·바이너리 파일의 행이 섞이지 않는지 확인한다(RED→GREEN).
- self-verify는 fail-fast라 이 run에서는 harness invariants, gofmt, Python 단계만 실행됐다. 그 뒤의 Go 테스트, golden, risk QA, build, QA smoke, install dry-run, MCP·state, native integration 단계도 2026-10-03 이후 처음으로 Linux·임시 HOME에서 돈다. G11에서 새 실패가 나오면 원인을 확인한 뒤 고친다.
- 검증은 PR CI의 verify job이다. Linux runner를 로컬에서 재현할 수 없으므로, PR CI가 install·self-verify 단계를 통과하는 run URL을 증거로 남긴다. 실패하면 로그의 실제 원인을 다시 조사한다(고정 대책으로 덮지 않는다).

### P2. 후보 결론

| 후보 | 결론 | 작업 |
|---|---|---|
| codex hook 이벤트 목록 | 유지 | `codexLifecycleHookEvents` 위에 "예전 설치본의 issueops 소유 hook을 지우는 목록이며 설치는 SessionStart만 한다"는 주석을 단다. 동작 변경 없음. |
| benchmark 예시 경로 | 수정 | `:101,103`의 경로를 실제 경로로 바꾸고 `owns`·`Worker` 키워드는 유지한다. 변경 전후 `issueops benchmark run` 점수가 같음을 기록한다. |
| readability CPU 전용 회귀 | 유지 | 할당 없이 CPU만 늘어나는 회귀를 잡으려면 시간 측정이 필요하다. `.issueops/cautions/2026-10-03-http.md`가 기록한 대로 이 테스트의 시간 단언은 전체 race 부하에서 넘쳤고 #548에서 할당량 단언으로 바꿨다. 또한 `Check`의 구성 요소(RE2 정규식, 단일 순회, map 기반 중복 검사)에 이차 경로가 없다. |
| readability 테스트 fixture | 보강 | 위 결론과 별개로, 반복 문장 끝에 줄바꿈을 넣어 줄 단위 경로(`addLineFindings`)도 크기에 비례해 실행되게 한다. 할당량 비율 단언은 유지한다. |
| 직접 테스트 없는 패키지 | 수정 | `domain/issueopsorphancleanup`(`NormalizeRequest`, `validGitOID`, `InspectInventory`), `application/issueopsbodysync`(CAS 불일치·child 검증 거부), `domain/nativehost`에 focused 테스트를 추가한다. 나머지는 데이터·wiring이라 유지한다. |
| quality inspect 비용 | 유지(방향 기록) | 정확한 fingerprint 캐시(`coverage.go:108-200`)는 증거 재사용 금지(`self-verification.md:117`)와 맞는 설계다. 전체 재실행이 반복되는 것은 실패한 결과를 캐시하지 않기 때문(`coverage.go:32`)이고, 실패가 해소되면 적중한다. 방향: self-verify 결과 공유는 하지 않고, 비용 문제가 다시 생기면 실패 패키지만 재실행하는 증분 방식을 별도로 검토한다. |
| Python skip 정책 | 유지 | 문서화된 정책이다. 변경 없음. |
| `os.IsNotExist` | 수정 | production 69곳을 `errors.Is(err, fs.ErrNotExist)`로 바꾼다. `sort.Slice`→`slices`는 스타일 차이라 유지한다(파일을 손댈 때만 바꾼다). |
| 대형 에이전트 문서 | 유지 | ADR·증거 문서가 참조한다. 변경 없음. |
| 원자적 쓰기 통합 | 유지(방향 기록) | 4종은 각자 소유 capability 안에서 정합하고, capability 간 공유 금지(`isSameCapabilityAdapter`)는 의도된 계층 규칙이다. 방향: 공용화는 중립 foundation 패키지를 두는 ADR이 먼저 있어야 하며, 구현 간 동작 차이로 결함이 생길 때 다룬다. |
| 분기 많은 함수 | 유지(방향 기록) | 상위 함수는 dispatcher 성격이고, 분기 수 지표는 case를 합산하는 구조적 신호라 PROJECT_AUDIT가 informational로 분류했다. 방향: 해당 함수를 기능 변경으로 손댈 때 함께 나눈다. |

### P3. 리뷰·재계획 상한 3→5 (사용자 추가 지시, 2026-10-07)

- 근거: 사용자 원문 "5회까지로 늘려주면 좋겠어 3회는 너무 아슬아슬해". 선택은 리뷰와 재계획 모두 5, #550 범위에 추가. 실제로 최근 두 사이클(#550 포함)이 3라운드째에 간신히 통과했다. contract_change feedback으로 기록했고 이슈 본문을 동기화했다.
- `internal/domain/issueopsreview/devils_advocate.go:11`의 `reviseRoundCap`을 5로, `internal/domain/issueopsreview/regress.go:10`의 `regressCap`을 5로 바꾼다. 거부 문구는 상한 숫자가 아니라 실제 횟수를 보고하므로(`planning_recorder.go:71`, `regress.go:36`) 문구는 그대로 두고, `ApplyReview` 주석의 "fourth"를 "sixth"로 고친다.
- 경계 테스트를 먼저 바꿔 RED를 본다: 도메인(`devils_advocate_test.go`, `regress_test.go`), adapter(`devilsadvocate/revise_round_cap_test.go`, `issueops/issueops_regress_cap_test.go`). revise는 다섯 번째까지 기록되고 여섯 번째가 거부된다. regress는 이미 4회면 다섯 번째가 허용되고 5회면 여섯 번째가 거부된다.
- `skills/issueops-review/SKILL.md`의 루프 규칙(:137, :159-170, :174, :224 근처)을 최대 5라운드, 비-waived revise 다섯 번까지·여섯 번째 거부로 고친다. effort 상승은 3라운드부터 유지해 3~5라운드를 한 단계 높은 effort로 띄운다고 적는다. `README.en.md:199-200`("at most three unwaived `revise` verdicts", "The fourth is refused")도 five·sixth로 고친다. 같은 숫자를 쓰는 다른 스킬·문서는 `rg`로 찾아 함께 맞춘다. 과거 기록인 ADR 본문, 연구 자료, 날짜가 붙은 증거 문서는 고치지 않는다. 상한과 무관한 `skills/issueops-implement/SKILL.md:182-185`의 focused test 실패 카운터("세 번째")도 고치지 않는다.
- revise 상한 3은 `.issueops/adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md`의 Decision (4)가, regress 상한 3은 `.issueops/adr/2026-07-02-issueops-regress-round-cap.md`가 정했다. 두 ADR의 나머지 결정(RegressEvents 감사 기록, 사람 결정 에스컬레이션, stop→reflect→regress 탈출 경로 등)은 그대로 유효하므로, 새 결정 기록은 두 ADR의 상한 숫자만 부분 대체한다고 밝힌다. 새 기록은 `project_docs_append`로 추가하고, `.issueops/ADR.md` 색인의 두 행에 저장소 관례("superseded in part")대로 부분 대체를 표시하는 수정은 `project_docs_revise`로 한다. 2026-09-24 Claude 역할 모델 ADR의 3라운드 effort 상승은 유지하므로 대체하지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md:15`: supervisor 부재 시 stdio로 자동 전환하지 않는다. 그래서 P1은 기본 transport 정책을 바꾸지 않고 CI가 명시적으로 stdio를 선택한다.
- AGENTS.md 철학: 하네스는 외부 도구 설치를 대행하거나 readiness gate로 요구하지 않는다. CI에서 systemd를 준비하는 대안은 이 원칙과 무관한 CI 환경 작업이지만, runner 권한·서비스 관리가 필요해 기각한다.
- `.issueops/testing/self-verification.md:117`: 다른 실행의 검증 결과를 재사용하지 않는다(quality inspect 분리 근거).
- `.issueops/testing/unit-and-contract.md:115-118`: Python suite skip 정책(유지 근거).
- `.issueops/cautions/2026-10-03-http.md`: 고정 시간 테스트는 부하에서 넘친다. readability 테스트에 시간 단언을 다시 넣지 않는다.
- `.issueops/CONSTITUTION.md` 제2장: CI 실패를 임시 우회로 덮지 않는다. P1은 정책(ADR)에 따른 명시적 선택이며 원인은 runner 환경이다.
- `.issueops/cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md`: race·self-verify 게이트는 부하가 낮을 때 실행한다.

## 재사용하는 기존 구현

- CI 단언은 기존 `scripts/ci_workflow_test.py`의 run 블록 추출 방식에 추가한다.
- 새 테스트는 대상 패키지의 기존 테스트 헬퍼와 `internal/testsupport`를 쓴다. 새 헬퍼 패키지를 만들지 않는다.
- `os.IsNotExist` 치환은 저장소가 이미 쓰는 `errors.Is(err, fs.ErrNotExist)` 관용구(40곳)를 따른다.

## 성능 영향

hot path 변경 없음. `errors.Is`는 wrap chain을 한 번 순회하므로 비용 차이는 무시할 수준이다. CI install이 stdio를 쓰면 서비스 기동 대기가 사라진다.

## 하위 호환성과 side effect

- CLI·MCP 출력, record schema 변경 없음. 기본 transport 정책 변경 없음.
- `os.IsNotExist`→`errors.Is`: wrapped not-exist 에러를 이제 "없음"으로 판정한다. 각 호출부에서 wrapped 에러가 들어올 수 있는지 확인하고, 판정이 바뀌는 곳은 테스트로 고정한다. 동작이 바뀌어선 안 되는 곳이 발견되면 그 호출부는 유지하고 이유를 남긴다.
- benchmark 예시 문구: 점수 불변을 전후 실행으로 확인한다.
- 상한 3→5: 같은 plan phase에서 waive하지 않은 revise를 4·5번째까지 받고, stop→재계획도 4·5번째까지 허용한다. 기존 상한 아래에서는 3회를 넘을 수 없었으므로 새로 풀리는 것은 3회에서 멈춰 있던 record뿐이다. record·출력 schema는 바뀌지 않는다.
- pr-review `mr_context.py`: rg가 없는 환경에서 정의·호출부를 grep으로 찾는다. rg가 있으면 동작이 같다.
- 롤백: P1 단독 커밋, P2 후보별 커밋, P3 커밋이라 개별 revert 가능.

## 게이트(G1..Gn)

| ID | 종류 | 기준 |
|---|---|---|
| G1 | CHECK | `gofmt -l $(git ls-files '*.go')` 출력 없음 |
| G2 | CHECK | `go vet ./...` 종료 코드 0 |
| G3 | CHECK | `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`(v2.12.2) 0건, `GOOS=linux`도 0건 |
| G4 | CHECK | `go test -race ./... -count=1` 전체 통과 |
| G5 | CHECK | `python3 scripts/ci_workflow_test.py` 통과, `rg -n -- '--mcp-transport=stdio' .github/workflows/ci.yml` 결과 있음 |
| G6 | CHECK | `shellcheck install.sh scripts/*.sh` 0건 |
| G7 | EXPECT | production `os.IsNotExist(` 0건(유지 사유가 기록된 곳 제외) |
| G8 | CHECK | `go test -count=1 ./internal/domain/issueopsorphancleanup ./internal/application/issueopsbodysync ./internal/domain/nativehost` 통과하고 각 패키지에 `_test.go`가 있다 |
| G9 | EXPECT | benchmark 예시 문구 변경 전후 점수 동일 |
| G10 | CHECK | `issueops self-verify --json` 모든 단계 통과 |
| G11 | EXPECT | PR CI verify job이 install과 self-verify 단계까지 통과(run URL 기록) |
| G12 | CHECK | `go test -count=1 -run Load ./internal/adapter/mcpservice` 통과(실패 에러에 명령과 출력 포함) |
| G13 | EXPECT | PR CI에서 supervisor 실패 원인((a)/(b)) 확인 결과가 PR 본문에 기록 |
| G14 | CHECK | python으로 감싼 `go test -v -run`(패턴은 `chr(124)`로 조립)이 세 패키지에서 통과하고, 출력에 `no tests to run`이 없으며 `TestRecordRejectsSixthUnwaivedReviseRound`가 실행됐고, `reviseRoundCap = 5`·`regressCap = 5`다 |
| G15 | CHECK | `skills/issueops-review/SKILL.md`가 최대 5라운드와 여섯 번째 거부를 말하고, 현행 스킬과 `README.en.md`에 "최대 3라운드"·"세 번까지"·"네 번째는"·"at most three unwaived"·"The fourth is refused"가 남지 않는다 |
| G16 | CHECK | 2026-07-02와 2026-09-08 ADR의 상한 숫자를 부분 대체하는 결정 기록이 있고, 행 머리의 날짜와 record 경로로 특정한 ADR 색인의 두 기존 행이 "superseded in part"를 표시한다(날짜만으로 찾으면 기존 superseded 행 때문에 헛통과한다) |
| G17 | CHECK | rg가 없는 PATH에서도 `skills/pr-review`의 `DefsFallbackTest`가 통과한다 |

원장은 4단계 진입 때 G1~G13으로 만들었고 EVIDENCE가 모두 pending이다. `gates init`은 기존 파일을 거부하므로 G1~G17로 원장 파일을 다시 만든다.
