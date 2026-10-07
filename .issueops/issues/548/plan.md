# 계획: #548 코드베이스 점검 결과 정비

- lifecycle ID: `io-7426b49dc042`
- issue: https://github.com/m16khb-org/issueops/issues/548
- branch: `548-codebase-maintenance-sweep` (base `main`, 봉인 SHA는 record의 `branch_prepare.base_sha`)
- 요청 범위: 사용자 원문 "전부 /issueops로 진행 하나의 이슈로 묶어서 진행". 종료점은 draft PR 발행과 `execution complete`다. merge·cleanup은 범위 밖이며 별도 요청으로만 한다.
- 세션: `execution prepare --mode direct`로 워크트리를 만든 뒤 환경별 자동 세션 인계(Orca ready면 같은 worktree의 새 세션, owner model `claude-opus-5-5`, effort `high`)를 적용한다. 인계는 승인 범위를 넓히지 않는다.

## 목표와 비목표

목표는 이슈 완료 기준을 묶음 A~D로 나눠 각 묶음을 별도 커밋으로 처리하는 것이다.

비목표:

- 로컬 개발 환경에 golangci-lint v2를 설치하는 일. 정규 명령은 이미 `.issueops/testing/unit-and-contract.md:29-38`에 v2.12.2와 `GOTOOLCHAIN` 지정까지 문서화되어 있어 저장소 변경이 필요 없다.
- `issueops*` 패키지 병합, 분기 많은 함수 재설계.
- CLI JSON·MCP schema·도구 이름 변경. `--help` 텍스트와 usage golden은 줄을 추가하기만 한다.
- adapter capability를 가로지르는 원자적 쓰기 헬퍼 통합(아래 "재사용하는 기존 구현" 참조).

## 사실 확인 결과(점검 주장 중 정정한 것)

계획 작성 중 점검 보고의 세 주장을 코드로 다시 확인해 범위에서 뺐다.

1. `internal/adapter/outbound/quality/coverage.go:261-297` `writeCoverageCache`는 best-effort 캐시다. 모든 실패 경로가 조용히 return하고 `defer os.Remove(temporaryPath)`가 임시 파일을 지운다. `os.Rename` 에러 무시는 같은 정책의 일부이며 결함이 아니다.
2. `internal/application/issueopsremote/issue_create.go:94,118,126`의 `context.Background()`는 durable intent 기록(Begin·Complete·Outcome)이 호출자 취소 뒤에도 남도록 한 것이다. `:99-101`의 dry-run 분기는 preview wrapper를 제거한 리팩터링이 "cancellation ... behavior are preserved"로 기존 동작을 보존한 것이다. 바꾸면 provider가 이슈를 만든 뒤 receipt가 유실될 수 있으므로 손대지 않는다. 반면 `artifact_verification.go:50-52`의 `Validate`는 받은 ctx를 `_`로 버리고 읽기 전용 트랜잭션에 `context.Background()`를 쓴다. durable 기록이 아니므로 받은 ctx를 쓰도록 고친다(D5). 같은 파일 `:66-68`의 `Record`는 durable 쓰기에 `context.WithoutCancel(ctx)`를 써서 이 저장소의 올바른 패턴을 이미 보여 준다. 그래서 이슈 완료 기준을 "durable 기록이 아닌 경로는 호출자 context를 전파한다"로 좁히는 contract_change를 기록하고 이슈 본문을 갱신한다.
3. 로컬 lint v1/v2 불일치는 사용자 환경 문제다(위 비목표).

## 작업 묶음

### A. 보안과 검증 신뢰성

- A1. `go.mod`의 `go 1.26.3`을 `go 1.26.6`으로 올린다. CI는 `.github/workflows/ci.yml:27-29`의 `go-version-file: go.mod`로 따라온다. 로컬은 `GOTOOLCHAIN=auto`(실측)라 자동으로 받는다. README.md:37, README.en.md:47, TECH_STACK의 Go 버전 표기를 함께 올린다.
- A2. `cmd/issueops/issueopsapp/mcp_concurrency_test.go:113-121` `exerciseMCPStream`: `listErr`를 확인하기 전에 `tools.Tools`를 읽지 않도록 고친다. 실패하면 `ListTools` 에러를 결과로 돌려준다. 근본 원인(부하에서 10초 컨텍스트가 만료되는지)도 재현해 확인한다. 만료가 원인이면 테스트 의도(80개 세션의 카탈로그 동일성)를 유지하며 타임아웃 구조를 정한다. 고정 타임아웃을 늘리는 것만으로 끝내지 않는다.
- A3. `internal/domain/artifactreadability/readability_test.go:282-296` `TestCheckLargeBodyIsFast`: wall-clock 200ms 단언을 결정적 검사로 바꾼다. 테스트 이름이 말하는 성질(O(n) 스캔, I/O 없음)을 입력 크기 배율 비교나 `testing.AllocsPerRun` 같은 부하 독립 관측으로 검증한다. 관측 방식은 구현 중 측정해 정한다. `.issueops/testing/concurrency-and-race.md:20`의 "wall-clock sleep·local machine state에 의존하지 않는다" 규칙을 따른다.
- A4. `quality inspect` 커버리지 실패 진단: `internal/application/quality/inspect.go:78-80`은 `coverage.err`만 warning에 남긴다. `coverage.value`에서 `FAIL\t<pkg>` 줄을 추출하는 순수 함수를 `internal/domain/quality`에 두고 warning에 실패 패키지(상한 있는 목록)를 덧붙인다. 먼저 현재 `go test -cover ./...` 실패 원인을 재현해 기록한다(이슈의 열린 결정). 원인이 A2·A3이면 그 수정으로 함께 해소된다.

### B. 문서와 도움말 정합성

현행처럼 읽히는 문서만 고치고 날짜가 붙은 기록 문서(cautions의 dated 파일, adr record 본문, research, plans, archive)는 고치지 않는다(`.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md` Resolution).

- B1. 누락된 하위 명령과 플래그를 각 줄의 단일 원본에 추가하고 `cmd/issueops/testdata/usage.golden.txt`를 갱신한다. `state maintain`(statecli/state_cli_router.go:25), `contract conformance`(contractcli/contract.go:41), `project append|commit-suggest|lint-diagnose`(projectcli/project_cli_dispatch.go:20-24), self-verify `--base-ref`·`--collect-all-steps`(verifycmd/verify.go:32,40)는 `internal/adapter/inbound/catalog/cli/usage.go`에 넣는다. `benchmark reliability|consensus`(benchmarkcmd/benchmark.go:20)는 `issueops benchmark` 줄의 유일한 원본인 `internal/contract/cli/issueops_catalog.go:94-96` `IssueOpsUsageCatalog`에 넣는다(`usage.go:12-14` 주석이 직접 쓰기를 금지). 그러면 `TestIssueOpsCommandGrammarAgreesAcrossCatalogFlagSetAndSpec`가 그 줄의 flag 목록을 실제 FlagSet과 대조한다. 각 명령의 실제 flag set을 읽고 usage 줄을 쓴다.
- B2. 문서 drift 수정. 대상은 점검의 grep 근거이며 구현에서 줄마다 코드와 다시 대조한 뒤 고친다:
  - `.issueops/OPERATIONS.md:82-87`, `.issueops/architecture/runtime.md:14`: 최상위 명령 목록(`issueops` 제거, `system-status` 추가, 개수는 `root_command_facade.go`의 표 기준).
  - `README.en.md:259`: CLI·MCP 개수를 golden 기준으로 고친다(MCP 50개 실측, CLI는 golden 재계수).
  - `.issueops/conventions/go-and-packages.md:78`: `TestProductionGraphHasNoForbiddenAdapterEdges`(실측: `internal/architecture/dependency_test.go:222`).
  - `.issueops/cautions/issueops-lifecycle.md:103,108,126`, `.issueops/conventions/state-policy-and-hooks.md:14-15,63`, `.issueops/testing/concurrency-and-race.md:42`: 코드에 없는 심볼을 실제 심볼로 바꾸거나 문장을 제거한다(`IssueOpsLifecycleTools`는 Go 코드 0건 실측).
  - `.issueops/CONVENTIONS.md:69`, `AGENTS.md:128`: worker 서술을 `internal/contract/worker/types.go`의 실제 모델과 일치시킨다.
  - `.issueops/ARCHITECTURE.md:90-94`, `.issueops/cautions/issueops-orchestration.md:17,50,56`: 소유 문서 링크와 없는 명령(`issueops handoff start`, `internal/core/issueops`) 안내.
  - `.issueops/TECH_STACK.md:45,81,112-137`: `configs/upstream.json` 내용, 직접 의존성 목록, "예정 사용 예" 표기.
  - `.issueops/OPERATIONS.md:49-56`: `issueops-sync-issue`·`issueops-sync-pr` 스킬 누락.
  - `.issueops/CAUTIONS.md` 색인: 2026-07-07 sqlstore 항목과 2026-08-27 daemon accept-loop 항목에 "경로 이동/daemon 제거" 표기(본문은 dated record라 유지).
- B3. ADR 폐기 표시: `.issueops/ADR.md` 색인과 `adr/README.md`에 superseded 표기를 단다(record 본문은 유지). 대상: 2026-07-02 Z.AI-only, 2026-07-02 External LLM usage observation, 2026-06-16 core facades, 2026-07-01 legacy JSON-RPC, 2026-07-03 Codex PreToolUse fallback, 2026-06-18 worktree tool preparation. `adr/roadmap.md`의 현재형 daemon·`internal/core` 서술은 historical 표기를 단다.
- B4. `.issueops/PROJECT_AUDIT.md` 재대조: 각 항목의 현재 경로와 상태를 HEAD 기준으로 갱신하고, 제거된 subsystem 항목은 "closed by removal"로 정리한다. `quality inspect`의 `audit_p1_p2_items` 파서가 읽는 표 규칙(Open 표의 bare P1/P2 행만 집계, #542에서 판독 실패를 차단)을 유지하고, 재대조 결과로 새 open 항목이 생기면 Open 표에 올린다. 250줄 예산을 넘는 부분은 archive로 옮긴다.
- B5. `skills/stability-audit/SKILL.md:61-62`와 `skills/stability-audit/scripts/e2e_stability_audit.py:407-441`: 등록이 해제된 Stop·SubagentStop·UserPromptSubmit hook 검사를 현재 설정(`configs/codex/hooks.json`, `configs/claude/hooks.settings.json`의 SessionStart만)에 맞춘다. 같은 디렉터리의 중복 테스트(`test_e2e_stability_audit.py`, `e2e_stability_audit_test.py`)는 겹치는 사례를 확인해 하나로 합친다.
- B6. 코드의 `internal/core` 잔재: `internal/architecture/dependency_test.go`의 core 관련 규칙과 synthetic 사례는 그대로 둔다. 현재 그래프에서 발화하지 않는 것은 core가 사라졌기 때문이며, 실제 코드 그래프 전체에서 core 재도입(adapter·cmd의 core import, core의 adapter·infra import)을 막는 유일한 검사가 `forbiddenEdges`의 core 절과 `TestProductionGraphHasNoForbiddenAdapterEdges`(:230,:242)다. `ownership_forbids_core_package`(:1142)는 foundation owner의 import에만 적용되고(:1165-1176), `TestStateSQLNetworkSourcePrefixesAbsent`(:255)는 세 접두사만 막는다(계획 리뷰 1·2라운드 결과). `internal/domain/guard/paths.go:81`의 `internal/core/` 접두사도 `guard/analysis.go:56,71-74`의 `contract-surface-without-golden` finding 조건이라 손대지 않는다. 문자열만 현재 경로로 바꾼다: `internal/adapter/provider/resolve.go:2`, `cmd/issueops/testdata/response_contracts.golden.json:4596,4647`, `cmd/issueops/issueopsapp/quality_contract_test.go:18`의 문자열은 현재 경로로 바꾼다. `internal/contract/state/results.go:3` `HookFailureLogFile`과 `internal/domain/state/doctor.go:92`의 `hook-metrics.jsonl`은 기존 사용자 상태 파일 인식용이므로 참조처를 확인해 유지 여부를 정하고 근거를 커밋 Lore에 남긴다.

### C. 의존성·CI·저장소 위생

- C1. `go get` 직접 의존성 최신화: `golang.org/x/sync`, `x/term`, `x/sys`, `github.com/dop251/goja`, `modernc.org/sqlite`(v1.53→v1.60.x). `golang.org/x/text`는 goja가 v0.3.8을 요구해 MVS로 고정되어 있으므로 명시 require로 최신화한다. `go mod tidy -diff` 빈 결과와 `go mod verify`를 확인한다. sqlite 메이저 동작 변화는 release note와 sqlstore 테스트로 확인한다.
- C2. `.github/workflows/ci.yml` action 메이저 버전: checkout v4→v7.0.1, setup-go v5→v7.0.0, golangci-lint-action v8→v9.3.0, setup-uv v6→v10.2.0(2026-10-07 `gh api releases/latest` 실측). 각 release note의 breaking change(입력 이름, Node 런타임, 캐시 기본값)를 읽고 입력을 맞춘다. golangci-lint 자체 버전 v2.12.2는 유지한다(CAUTION 2026-10-03의 로컬·CI 동일 버전 계약). `scripts/ci_workflow_test.py`가 통과해야 한다.
- C3. CI에 shellcheck 단계를 추가한다(`install.sh`, `scripts/*.sh`). 현재 지적은 `scripts/verify-child-host-smoke.sh`의 SC2181(:778), SC2329(:800,:809)뿐이므로 먼저 고친다. 하네스 install/update/self-verify 경로에는 넣지 않는다(AGENTS.md 철학).
- C4. `.gitignore`의 `evidence` 줄을 `.issueops/evidence/`로 고정한다. 변경 전후 `git status --ignored`로 다른 경로의 ignore 판정이 바뀌지 않는지 확인한다.

### D. 코드 품질과 테스트 격리

- D1. `printJSON` 통합: cmd 하위 15개 패키지(statecli, contractcli, loopcli, statuscli, apidoc, projectcli, channelcli, workercli, installcli, basiccli, policycli, updatecli, gatescli, issueopscli/benchmarkcmd, issueopscli)의 동일 본문(`encoder := json.NewEncoder(os.Stdout); SetIndent("", "  "); Encode`)을 새 leaf 패키지 `cmd/issueops/jsonout`의 `Print(v any) error`·`PrintTo(w io.Writer, v any) error`로 모은다. `issueopsapp/app.go:61`의 `printJSONTo`도 이것을 쓴다. leaf 패키지는 다른 cmd 패키지를 import하지 않아 composition root와 순환하지 않는다.
- D2. 표준 라이브러리 대체: production의 `containsString`/`contains` 사본은 `slices.Contains`, `firstNonEmpty` 사본은 `cmp.Or`로 바꾼다(동작 동일성을 사본마다 확인: `cmp.Or`는 zero value가 아닌 첫 값을 돌려주므로 공백 trim 여부가 다른 사본은 그대로 둔다).
- D3. 테스트 격리:
  - `cmd/issueops/issueopscli/remotecmd/remote_child_recovery_test.go:246-269`: `exec.LookPath("glab")` 결과가 테스트 임시 디렉터리 안인지 단언하고, 아니면 실패시킨다(실제 GitLab 호출 차단).
  - `cmd/issueops/projectcli/bootstrap_service_test_helpers_test.go:19`: `statestore.StateDir()`가 실제 `~/.local/state`를 읽지 않도록 패키지 테스트에서 `ISSUEOPS_STATE_DIR`을 임시 디렉터리로 둔다.
  - `cmd/issueops/contractcli/conformance_test.go:93-166`의 수동 `os.Setenv`/복원을 `t.Setenv`로, `internal/adapter/channel/store_test.go:250,261`의 죽은 줄을 제거한다. `os.Chdir` 2곳(`cmd/issueops/pathutil/path_helpers_test.go:68,74`, `cmd/issueops/updatecli/update_bootstrap_test.go:105-108`)은 `t.Chdir`로 바꾼다.
  - git 전역 설정 격리: `internal/testsupport`에 `IsolateGitConfig(t)`(`GIT_CONFIG_GLOBAL`=빈 임시 파일, `GIT_CONFIG_NOSYSTEM=1`)를 두고 공유 git 헬퍼 정의(`runGit*`·`gitRun`·`initGitRepo*` 등 이름 패턴으로 실측한 11개)에서 호출한다. 이 머신의 전역 설정은 `commit.gpgsign`·`core.hooksPath`가 비어 있어(실측) 현재 실패는 없다. 다른 개발 환경에서의 재현성 보장이 목적이다.
- D5. `internal/application/issueopsremote/artifact_verification.go:50-52` `Validate`가 받은 ctx를 읽기 전용 트랜잭션에 전달하도록 고친다. 취소된 ctx에서 `Validate`가 레코드를 읽지 않고 `context.Canceled`를 돌려주는지 확인하는 focused 테스트로 RED→GREEN을 남긴다. 실제 저장소 경로(sqlstore `runSpan`)는 취소된 ctx에서 콜백 전에 `ctx.Err()`를 돌려주지만 application 테스트 fake(`artifact_verification_test.go:231`)는 `fn(ctx)`를 그대로 호출하므로, 테스트는 ctx 취소를 존중하는 fake를 쓰거나 실제 `RemoteRecordStore`로 검증한다. durable 기록 경로(`issue_create.go:94,118,126`, `Record`의 `WithoutCancel`)는 그대로 둔다.
- D4. 느린 테스트: 전체 race 실행에서 가장 느린 3개(`TestCLIHelpers` 72s, `TestHandleSelfLoopMCPToolCallCoversLocalPayloads` 41s, `TestRunStatusWritesTextAndJSON` 38s)의 시간 소비 원인을 프로파일로 확인한다. 반복 빌드나 중복 실행처럼 검증 범위를 줄이지 않고 제거할 수 있는 비용만 줄인다. 원인이 검증 자체라면 바꾸지 않고 근거를 기록한다. 고정 sleep(`internal/adapter/policy/policy_run_bounds_test.go:47` 2s 등)은 이벤트 기반으로 바꿀 수 있을 때만 바꾼다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장 문제 해결 원칙: 증상만 덮지 않는다. A2는 panic 제거와 함께 `ListTools` 실패의 실제 원인을 확인하고, A3은 기준값 완화가 아니라 관측 방식을 바꾼다.
- `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md`: 제거 작업(B6) 전에 이름으로 git grep을 Go 문자열·usage golden·testdata·skills·현행 문서·configs까지 돌린다. dated 기록 문서는 고치지 않는다.
- `.issueops/cautions/2026-10-03-native-http-validation-and-lint-toolchain.md`: 로컬·CI golangci-lint는 v2.12.2를 유지하고 `GOTOOLCHAIN`은 `go.mod`에서 고른다. A1 뒤 lint는 go1.26.6 표준 라이브러리를 읽으므로 v2.12.2 analyzer가 이를 읽을 수 있는지 확인한다.
- `.issueops/cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md`: 게이트 CHECK는 900초 상한이다. race 전체(이번 점검 실측 676초)는 부하가 낮을 때 실행하고, 시간 초과는 같은 원장으로 재실행한다. EXPECT를 완화하지 않는다.
- `.issueops/conventions/go-and-packages.md:51-79` 계층 규칙과 `internal/architecture/dependency_test.go:1398-1412` `isSameCapabilityAdapter`: adapter capability를 가로지르는 import는 금지다. 원자적 쓰기 헬퍼 통합은 이 규칙 때문에 범위에서 뺐다.
- `.issueops/conventions/state-policy-and-hooks.md:51` reuse-before-new: 새 패키지(`cmd/issueops/jsonout`)와 새 테스트 헬퍼(`testsupport.IsolateGitConfig`)는 아래 절의 근거를 갖는다.
- `.issueops/testing/concurrency-and-race.md:20`: 테스트는 wall-clock sleep과 로컬 머신 상태에 의존하지 않는다(A3, D3).
- `.issueops/TESTING.md` 부분 검증 상태 금지: 완료 증거는 단일 전 단계 통과 run에서 나온다.
- ADR 대조: `2026-07-01-mcp-transport-go-sdk-legacy-jsonrpc`, `2026-07-02-external-llm-zai-only`는 B3에서 superseded 표기 대상이다. 그 밖의 결정과 충돌하는 변경은 없다.

## 재사용하는 기존 구현

- 실패 패키지 추출(A4)은 같은 출력을 이미 파싱하는 `internal/domain/quality`의 `policy.ParseCoveragePackages`(호출: `inspect.go:81`, `coverage.go:266`) 옆에 둔다.
- usage(B1)는 `internal/adapter/inbound/catalog/cli/usage.go`의 기존 렌더링과 `usage.golden.txt` 갱신 경로(`go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update`)를 쓴다.
- 테스트 출력 캡처와 공용 헬퍼는 기존 `internal/testsupport`(현재 41개 파일이 import)에 넣는다.
- `printJSON`(D1)은 재사용할 공용 위치가 없다. `cmd/issueops/pathutil`은 경로 전용이고, `issueopsapp`은 각 cli 패키지를 import하는 composition root라 역방향 import는 순환이 된다. 그래서 최소 leaf 패키지를 새로 만든다.
- 원자적 쓰기 4종(`cmd/issueops/contractcli/conformance.go:369`, `internal/adapter/mcp/conformance_probe.go:306`, `internal/adapter/lifecycle/lifecycle_project_state_store.go:67`, `internal/adapter/outbound/quality/coverage.go:278`)은 서로 다른 계층·capability에 있어 공용 위치를 두려면 계층 규칙 예외가 필요하다. 이번 범위에서 통합하지 않는다.

## 성능 영향

- hot path 변경 없음. D1·D2는 함수 호출 경로만 바꾸며 복잡도가 같다.
- A4의 실패 패키지 추출은 이미 메모리에 있는 커버리지 출력(상한 16MiB, `CoverageCommandOutputLimit`)을 한 번 훑는 O(n)이고 실패 시에만 실행한다.
- A1·C1의 툴체인·의존성 갱신은 빌드 시간 변화가 있을 수 있다. `go build` 시간과 바이너리 크기를 전후로 기록한다.
- D4는 성능 개선 작업이다. 개선 주장은 같은 조건의 전후 `go test -run <name> -count=1` 실측으로만 한다.

## 하위 호환성과 side effect

- CLI JSON·MCP schema·도구 이름: 변경 없음. `mcp_tools.golden.json`과 `response_contracts.golden.json`은 B6의 경로 문자열 외에는 바뀌지 않아야 하며, diff로 확인한다.
- usage golden: B1로 줄이 추가만 된다. 기존 줄 변경이 생기면 그 이유를 커밋 Lore에 남긴다.
- `quality inspect` warning 문자열: 커버리지 실패 시 실패 패키지가 덧붙는다. warning은 사람이 읽는 진단 문자열이며 소비 코드가 파싱하지 않는지 grep으로 확인한다. finding id `quality-collector-error`와 severity는 그대로다.
- record schema·provider body 계약: 변경 없음. DB 마이그레이션·스키마 변경 없음.
- `go.mod` 상향: go1.26.6 미만 툴체인으로 이 저장소를 빌드하던 환경은 `GOTOOLCHAIN=auto`면 자동 다운로드, `local`이면 빌드가 거부된다. README와 TECH_STACK에 반영한다.
- CI action 메이저 상향: 실패하면 해당 action만 이전 메이저로 되돌릴 수 있도록 C2를 단독 커밋으로 둔다.
- `internal/architecture/testdata/ddd_responsibility_inventory.json`은 production 함수 목록을 고정한다(`ParseCoveragePackages`가 :15590에 있다). D1·A4·D5로 함수가 추가·삭제되면 해당 architecture 테스트의 갱신 경로로 inventory를 재생성하고 diff가 의도한 함수만 바꾸는지 확인한다.
- 롤백: 묶음별 커밋이므로 묶음 단위 revert가 가능하다.
- 프롬프트 본문 변경: B5가 stability-audit 스킬 본문을 고치지만 LLM 출력 기준이 아니라 검사 대상 목록 정정이다. 스킬 검증은 `python3 scripts/validate-skill.py skills/stability-audit`와 해당 Python 테스트로 한다.

## 게이트(G1..Gn)

| ID | 종류 | 기준 |
|---|---|---|
| G1 | CHECK | `gofmt -l $(git ls-files '*.go')` 출력 없음 |
| G2 | CHECK | `go vet ./...` 종료 코드 0 |
| G3 | CHECK | `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`(v2.12.2) 0건 |
| G4 | CHECK | `go test -race ./... -count=1` 전체 통과 |
| G5 | CHECK | `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` 종료 코드 0(호출되는 취약점 0건) |
| G6 | CHECK | `go mod tidy -diff` 빈 출력, `go mod verify` 통과 |
| G7 | CHECK | `go test ./internal/architecture -count=1` 통과 |
| G8 | CHECK | `go test -race -run TestConcurrentMCPStreamsShareImmutableConfiguration -count=20 ./cmd/issueops/issueopsapp` 통과 |
| G9 | EXPECT | `issueops quality inspect --repo . --json`의 `collection_status`가 `error`가 아니다 |
| G10 | CHECK | `python3 scripts/ci_workflow_test.py` 통과, `shellcheck install.sh scripts/*.sh` 지적 0건 |
| G11 | EXPECT | `issueops --help`에 B1의 하위 명령·플래그가 모두 보인다 |
| G14 | CHECK | `git diff --quiet "$BASE_SHA" -- internal/architecture/dependency_test.go internal/architecture/ownership_manifest_test.go internal/domain/guard/paths.go` 종료 코드 0(core 회귀 가드와 guard 조건 불변) |
| G15 | CHECK | `grep -n "benchmark reliability" internal/contract/cli/issueops_catalog.go` 결과가 있다 |
| G16 | CHECK | `grep -n "context.Background()" internal/application/issueopsremote/artifact_verification.go` 결과가 없다 |
| G12 | EXPECT | B2·B6 대상 문자열(`IssueOpsLifecycleTools`, `TestProductionGraphHasNoLegacyAdapterEdges`, 현행 문서의 `internal/core/` 경로)이 현행 문서에서 0건이다(dated 기록, `internal/architecture` 회귀 가드, `guard/paths.go` 제외) |
| G13 | CHECK | `issueops self-verify --json` 모든 단계 통과 |

G4·G13은 부하가 낮을 때 실행하고, 900초 상한에 걸리면 같은 원장으로 재실행한다.

## 커밋 구성

A(툴체인·테스트·진단) → C(의존성·CI·위생) → D(헬퍼·격리·성능) → B(문서·도움말·잔재) 순서로 묶음별 커밋을 만든다. 문서 묶음을 마지막에 두는 이유는 A~D의 결과(의존성 목록, 명령 수, 테스트 이름)를 문서에 반영하기 위해서다.
