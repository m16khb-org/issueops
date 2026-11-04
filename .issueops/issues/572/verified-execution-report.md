# #572 verified-execution report

- lifecycle: `io-7426b49dc042`, branch `572-effective-go-refactor`, base `7714640b0b40b4b03e64ad1405140d47d26e9c84`
- 계획: `.issueops/issues/572/plan.md`(봉인 원본 sha256 `ef0a07ec8b1824369e50312fd235987590ea04631e7284fc5a7471ccb80a680b`로 시작)
- 게이트 원장: `.issueops/issues/572/gates.md`

## 목표와 수용 기준

Effective Go 관례에 맞춰 Go 코드를 고친다. 이름(package clause·stutter·대소문자 쌍·내장 식별자 가림), 에러·인터페이스 확인, context·종료 경로·port, 문서 주석이 대상이다. CLI/MCP 이름, JSON 필드, record schema, golden은 바꾸지 않는다.
게이트 결과는 원장의 EVIDENCE가 정본이다.

| 기준(intent 성공 기준) | 게이트 | 결과 |
|---|---|---|
| 측정 설정으로 다시 쟀을 때 production의 비주석·주석 지적이 0건이다 | G2a·G2b·G2c | 통과. base darwin 4724·linux 4722·windows 4721건, head 0·0·0건 |
| build·vet·gofmt·`lint-local.sh`·`go test`·`go test -race`가 모두 통과한다 | G3·G4·G5·G6·G15 | 통과 |
| response-contract golden이 갱신 없이 통과한다 | G8 | `golden-unchanged` |
| B군 동작 변경마다 실패-후-통과 테스트나 실행 증거가 있다 | G7·G11·G12·G13·G14와 아래 구현 요약의 테스트 | 통과 |
| package clause가 디렉터리 이름과 같고 importer 별칭이 하나로 정리된다 | G1·G16 | 통과 |
| 이름 변경 뒤 옛 이름이 현행 문서·코드에 남지 않는다 | G9 | `renames-clean` |
| 저장소 신호가 이름 변경 뒤에도 참이다 | G10 | 통과 |

## 구현 요약

변경 규모: base 대비 2,181개 파일(Go 2,163개), +21,496/-12,993줄. 이름 변경과 문서 주석이 대부분이다.

1. package clause·stutter 이름: 저장소 밖 일회용 도구(`golang.org/x/tools/go/packages`로 세 GOOS를 함께 읽어 객체 단위로 치환)로 처리했다. package clause 3개를 디렉터리 이름에 맞췄고 importer 별칭을 정리했다. stutter 이름 208개(`contract/issueops` 105개 포함, `issueopsremote` 7개 포함)의 접두어를 뗐다. 충돌한 `MutationAck()`·`BranchPrepareAck()`는 `NewMutationAck()`·`NewBranchPrepareAck()`로, `policy.PolicyTier*` 4개는 기존 `Tier*`로 합쳤고 `IssueOpsRemoteArtifact`는 `ArtifactDraft`가 됐다. 이름을 문자열로 찾는 `augmentation/signals.go` 검색어, DDD 역할 목록, judge prompt 한 단어, flag 도움말, 현행 문서를 같이 고쳤다. 저장소 신호 테스트 `TestCollectSignalsHoldOnThisRepository`를 이름 변경 전에 먼저 추가해 통과시켰다.
2. 대소문자 쌍·에러·인터페이스 확인·나머지: `confusing-naming` 전부(24건과 플랫폼 전용 파일), 내장 식별자 가림 7곳(`copy`·`any`·`max`), if-return 3곳, else 체인 2곳, 중복 import, blank import 주석, `wg.Go` 2곳, `jobNotFoundError`, `nestHTTPStatusNames`, `hostprobe` 에러 sentinel과 `errors.Is`, `judgement/structured.go`의 `io.EOF` 비교를 고쳤다. `var _` 확인을 두 provider의 모든 선택 인터페이스, 설치기 다섯 개, `CodexRunner`·`ClaudeRunner`, `mcpservice.Service`에 더했다. `readiness_service.go`는 `scoped := s.scoped()`로 바꿔 수신자 재대입을 없앴고, `fakecmd`는 `fail`이 종료 코드를 돌려주게 했다. 고유 명사로 시작하는 에러 문자열 4건은 근거를 적은 `//nolint:staticcheck,nolintlint`로 남겼다.
3. context·종료 경로·port: 아래 테스트로 실패-후-통과를 확인했다.
   - `TestLintDiagnoseReturnsChildExitCodeWithoutExtraStderr`(`issueopsapp`): 수정 전에는 `os.Exit(7)`이 테스트 프로세스를 끝냈다. 수정 뒤 `ExitCodeError`가 종료 코드 7을 돌려주고 stderr 마지막 줄은 구분선이다(G7).
   - `TestBoundedCommandStartErrorUnwrapsCause`·`TestRunContextStopsOnCanceledContext`(`providerutil`): `RunContext`와 `startError`를 넣기 전에 실패했다(G11).
   - `TestVerifyChildHierarchyStopsOnCanceledContext`(github·gitlab): `VerifyChildHierarchy`가 ctx를 버리게 되돌리면 표지 파일이 생겨 실패한다.
   - `TestServeMCPStreamCancelsInFlightRequestOnShutdown`·`TestServeMCPStreamStopsDrainOnContextCancel`(`mcpcli`): 이전 동작을 본뜬 스텁에서 각각 5초와 10초 뒤 실패했다. 새 `serveMCPStream`에서 통과한다(G12).
   - 하위 프로세스 smoke `stdio_signal_smoke.py`: base에서는 `mcp exit status -15 after SIGTERM`으로 실패했고 지금은 종료 코드 0이다(G14, 약 15초).
   - `TestLockedChildCreatorPassesLeaseContext`, `TestIssueGraphPosterStopsOnCanceledContext`, `TestObserveRemoteArtifactStopsOnCanceledContext`, doctor `ProbeGatewayHTTP`·`runLsof` 취소 테스트.
   `port.IssueProvider`의 `CreateIssue`·`CreatePullRequest`·`CreateChild`·`CloseChild`와 `IssueProviderRemoteCreateReconciler.ReconcilePullRequest`가 ctx를 첫 인자로 받는다. `IssueProviderCreateIssueContexter`와 두 비공개 fallback 인터페이스를 지웠다. provider 내부 `runGhAPIJSON`은 `providerutil.RunContext`로 ctx를 받고, `runGhJSON`·`runGlabGraphQL` wrapper를 지웠다. 시작 실패는 `errors.Is(err, exec.ErrNotFound)`로 판정한다. stdio `issueops mcp`는 SIGINT·SIGTERM에서 처리 중인 요청을 최대 10초 기다린 뒤 취소하고 종료 코드 0으로 끝난다.
4. 비공개 전환과 문서 주석: 도구가 패키지 밖에서 쓰이지 않는 exported 이름 401개를 비공개로 바꿨다(인터페이스 메서드 이름, 다른 패키지 사용, 임베딩, `signals.go`·DDD 목록이 가리키는 이름, 다른 exported 호출이 돌려주는 타입은 제외). 남은 exported 식별자와 패키지 주석은 패키지 단위 작업자 25배치와 재작업 2배치가 작성했다. 작성 뒤 토큰 단위 비교(주석과 공백 제외)로 코드가 바뀌지 않았음을 확인했다.
5. 재측정과 전체 검증: 위 기준표와 아래 "계획과의 차이"를 따른다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/CONSTITUTION.md:72` | "수정 뒤에는 같은 실패가 재발하지 않음을 보여주는 targeted test, command smoke, grep evidence, 로그/상태 확인 중 작업 성격에 맞는 검증을 남긴다." | 묶음 3의 동작 변경마다 실패 테스트를 먼저 쓰고 통과시켰고, 이름 변경마다 옛 이름 검색(G9)을 돌렸다. |
| `.issueops/architecture/hexagonal-core.md:57` | "`cmd/issueops/issueopsapp`: concrete adapter를 조립하는 유일한 composition root다." | `var _` 확인은 adapter 자기 패키지(`provider/github`, `provider/gitlab`, `codex`, `claude`, `agy`, `opencode`, `extensionhost`, `hostprobe`, `mcpservice`)에 두었고 새 adapter import edge는 없다(G13). |
| `.issueops/conventions/go-and-packages.md:17` | "Go 패키지명은 짧은 소문자 단어를 사용한다." | package clause 3개를 디렉터리 이름(`issueopsremote`, `guardpattern`, `traceclassification`)에 맞췄다(G1). |
| `.issueops/conventions/go-and-packages.md:108` | "**주입에 default 구현을 두지 않는다.**" | `IssueProvider` 네 메서드가 ctx를 직접 받는다. context 없는 기본 경로와 fallback 인터페이스를 지웠다. |
| `.issueops/conventions/go-and-packages.md:83` | "production 진입점에서 호출하지 않는 테스트용 위임 함수·타입·상수는 `*_test.go`에 둔다." | 테스트 전용 위임을 production에 남기지 않았다. 한 줄 위임 쌍(`installcli`의 `Apply`·`Rollback`·`Finalize`, `RunInstall`)은 하나로 합쳤다. |
| `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md:13` | "제거 커밋 전에 지운 이름(명령, 플래그, 경로, 디렉터리, MCP tool)으로 git grep을 Go 문자열·usage golden·testdata·skills·.issueops 현행 문서·configs까지 돌린다." 그리고 "날짜가 붙은 기록 문서는 고치지 않고, 현행처럼 읽히는 문서만 고친다." | 이름 변경 뒤 Go 문자열, testdata, flag 도움말, 현행 문서를 `check_renames.py`(G9)로 검색해 고쳤고 dated 기록은 바꾸지 않았다. |
| `.issueops/CAUTIONS.md:55` | "Dated 기록의 IssueOps 명령·필드·상태는 사고 당시 증거일 뿐 실행 지시가 아니다." | `adr/20*`, `cautions/20*`, `plans/` 등 기록 문서는 바꾸지 않았다. G9가 base 대비 diff 없음을 확인한다. |
| `.issueops/adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md:13` | "re-export는 internal/architecture/dependency_test.go의 레이어 규칙상 정식 심볼을 직접 import할 수 없을 때만 둔다" | 옛 이름 alias를 만들지 않았다. 기존 근거 있는 alias는 새 이름을 가리키게만 바꿨다. |
| `.issueops/adr/2026-08-08-port-contract-vocabulary.md` | "port가 contract를 참조하거나 port 사이를 참조하는 것은 위반이 아니다." | `IssueProvider` 시그니처 변경은 `port` 안에서 contract 타입과 `context`만 쓴다. |
| `.issueops/testing/unit-and-contract.md:41-42` | "문체·미세 성능 linter(revive, gocritic, perfsprint, predeclared, stylecheck(ST), quickfix(QF))는 버그를 잡지 않고 코드만 흔들어서 켜지 않는다." | `.golangci.yml`을 바꾸지 않았다. revive·ST는 이슈 폴더의 측정 설정으로만 돌렸다. |
| `.issueops/testing/unit-and-contract.md:78` | "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다." | golden을 갱신하지 않았다. G8이 갱신 없이 통과하고 testdata diff가 없다. |
| `.issueops/TESTING.md:46-47` | "다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 첫 게이트부터 전체 재실행한다." | 최종 게이트 18개를 한 번의 `gates check`에서 모두 통과시켰다. |
| `.issueops/cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md:50` | "CI 재현은 `GOOS=linux golangci-lint run ./...`와 `GOOS=linux go vet ./...`를 함께 돌린다" | `./scripts/lint-local.sh`(GOOS=linux 포함, G3)와 세 GOOS 빌드·vet(G4)을 게이트에 넣었다. |

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/conventions/go-and-packages.md` | "Go 패키지명은 짧은 소문자 단어를 사용한다." | 지킴 | `internal/domain/{issueopsremote,guardpattern,traceclassification}`의 package clause. G1 |
| `.issueops/testing/unit-and-contract.md` | "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다." | 지킴 | `cmd/issueops/testdata`·`internal/adapter/testdata` diff 없음. G8 |
| `.issueops/adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md` | "re-export는 internal/architecture/dependency_test.go의 레이어 규칙상 정식 심볼을 직접 import할 수 없을 때만 둔다" | 지킴 | 옛 이름 alias 없음. 기존 `issueopsauthorization.Actor` 등은 새 이름을 가리킨다 |
| `.issueops/conventions/go-and-packages.md` | "**주입에 default 구현을 두지 않는다.**" | 지킴 | `internal/adapter/issueops/remote_provider_guard.go`가 fallback 없이 `prov.CreateIssue(ctx, req)`를 부른다 |
| `.issueops/testing/unit-and-contract.md` | "문체·미세 성능 linter(revive, gocritic, perfsprint, predeclared, stylecheck(ST), quickfix(QF))는 버그를 잡지 않고 코드만 흔들어서 켜지 않는다." | 지킴 | `.golangci.yml` 변경 없음. 측정은 `.issueops/issues/572/effective-go-lint.yml`로만 했다 |
| `.issueops/CONSTITUTION.md` | "에이전트, hook, relay, retry, monitor, orchestration의 모든 반복은 유한한 상한과 명시적인 성공·실패·취소·timeout·no-op 종료 경로를 가져야 한다." | 지킴 | stdio MCP 종료는 drain 10초(`httpShutdownDrain`)와 닫힘 대기 2초(`stdioStuckLimit`)로 끝나고, 두 번째 신호는 기본 처리로 즉시 종료한다(`cmd/issueops/mcpcli/mcp_sdk_server.go`, `mcp_transport.go`). EOF 대기도 같은 ctx에 묶었다 |
| `.issueops/ARCHITECTURE.md` | "`internal/port`는 contract 외 `internal/...` concrete 구현에 의존하지 않는다." | 지킴 | `internal/port/provider.go`는 `context`와 contract 타입만 쓴다. `go test ./internal/architecture`(G13) 통과 |
| `.issueops/ARCHITECTURE.md` | "`internal/adapter/*`는 composition root(`cmd/issueops/issueopsapp`)에서만 조립되고 금지 adapter edge는 0이다." | 지킴 | 새 `var _ port.X` 확인과 `mcpservice`의 `port` import는 허용된 edge다. `provider` 패키지는 `processlease`를 직접 import하지 않고 `providerutil.RunContext`를 쓴다(G13) |
| `.issueops/cautions/2026-07-28-update-mcp-lifetime-host-owned.md` | "update는 host가 소유한 stdio MCP 프로세스나 외부 MCP를 열거·종료·접속하지 않는다." | 지킴 | `update`는 stdio 프로세스를 다루지 않는다. 바뀐 것은 `issueops mcp` 프로세스가 신호를 받았을 때의 종료 방식뿐이다. `mcp cleanup --apply`가 보내는 SIGTERM은 같은 규칙으로 최대 10초 뒤 끝난다(ADR에 적었다) |

## 성능 영향

- hot path 변경 없음. 이름 변경, 문서 주석, `var _` 확인은 컴파일 결과의 동작을 바꾸지 않는다.
- context 전달은 외부 명령마다 취소 감시 goroutine 하나를 더한다. 외부 명령 실행 비용에 비해 무시할 수 있다.
- stdio MCP의 신호 처리는 프로세스당 등록 한 번이다. 신호가 없으면 요청 경로에 비용이 없다.
- 측정한 성능 주장은 없다.

## 계획과의 차이

- G6·G15의 CHECK를 `python3 -B -c` 한 줄 래퍼로 감쌌다. policy runner가 긴 출력의 가운데 줄을 버려 `/ok\s+issueops/`가 불일치했다. 명령(`go test -race ./... -count=1`, `go test ./... -count=1`)과 통과 조건(종료 코드 0과 `ok` 줄 존재)은 같다. `decision add`로 기록했다.
- G9·G16 스크립트는 작업 트리에서 지워졌지만 index에 남은 파일을 건너뛰고 새 파일을 포함하도록 고쳤다.
- 계획은 비공개 전환을 작업자 풀에 맡겼지만, 외부 사용 여부는 타입 정보로 정확히 가려지므로 도구가 먼저 전환했고 작업자는 주석만 맡았다. 같은 제외 규칙을 적용했다.
- `domain/operationalhealth/contract_alias.go`와 `domain/projectdoc/contract_alias.go`의 그룹 주석은 첫 항목의 이름으로 시작하지 않아 항목별 주석으로 나눴다(작업자 검사기가 놓친 2건을 재측정에서 찾았다).
- 문서: `.issueops/conventions/go-and-packages.md`에 이름·주석·context 규칙을, `.issueops/architecture/runtime.md`에 stdio 종료 방식을 적었다.

## ai-slop-clean

범위는 base 대비 변경 파일 전체다. 점검 결과 제거한 것은 없다.

| 분류 | 점검 방법 | 결과 |
|---|---|---|
| dead-code | `./scripts/lint-local.sh`의 `unused`·`unparam`(darwin·linux), 비공개 전환 뒤 빌드·vet | 0건. 비공개로 바꾼 이름 중 production에서 쓰이지 않게 된 것이 없다 |
| weak-artifact | 추가된 줄에서 `TODO`·`FIXME`·`XXX`·`HACK`·`STUB` 검색, `git diff --check` | 표지 없음. 검색에 걸린 `temporary`는 임시 디렉터리를 뜻하는 정상 문구다 |
| unsupported-claim | 추가된 문서·보고서 줄에서 `보장`·`완전`·`검증됨`·`always`·`guarantee` 검색 | 근거 없는 단정 없음. `완전한 문장`은 주석 형식 규칙의 표현이다 |

- 일부러 둔 것: 작업자가 쓴 주석은 개별로 읽지 않고 샘플로 점검했다. 코드가 주석 외에 바뀌지 않았다는 사실은 토큰 비교로 확인했다. 주석 내용의 정확도는 구현 리뷰가 다룬다.
- 범위 밖 발견: `RunInstall` 같은 한 줄 위임은 이번에 합쳤지만, `internal/adapter/hostprobe`의 episode 러너 다섯 개는 구조가 비슷하다. 통합은 이 이슈의 범위가 아니다.
- 이 단계는 측정값을 내세우지 않는다. 정리 전후 SNR·중복 수치를 주장하지 않는다.

## 검증

| 요구한 문서 | 그 문서의 해당 문장 인용 | 명령 | 결과 |
|---|---|---|---|
| `.issueops/TESTING.md` | "다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 첫 게이트부터 전체 재실행한다." | `issueops gates check --file .issueops/issues/572/gates.md ... --env HOME,PATH,LANG,GOPATH,GOMODCACHE,GOCACHE --timeout-seconds 900`(원장을 초기화하고 18개를 한 번에) | 18/18 met. 세 번째 시도에서 통과했다. 앞선 두 시도는 `internal/adapter/cmux`의 `TestExecRunnerReportsFailureKindsWithInvocationState`(시간 의존)가 G15·G6에서 실패했다. 같은 테스트 바이너리를 12개 병렬로 돌린 부하 시험에서 base도 72회 중 2~6회, head는 4~15회 실패했다. cmux는 이 변경에서 이름 비공개 전환과 주석만 바뀌었다 |
| `.issueops/issues/572/artifact/plan.md`의 게이트 | G2a·G2b·G2c | `golangci-lint run -c .issueops/issues/572/effective-go-lint.yml ./...`(GOOS=darwin·linux·windows) | base(`git archive` 임시 경로) 4724·4722·4721건, head 0·0·0건 |
| 스킬(`issueops-verify`의 self-verify 배터리) | 해당 없음(스킬이 명령을 정한다) | `./bin/issueops self-verify --base-ref 7714640b... --seed=100 --target-score=95 --llm-eval=false --json` | `ok=true`, 27/27 step, `minimum_goal_score` 100, `termination_eligible` true(약 187초) |
| 스킬(`issueops-verify`) | 해당 없음(환경 메모) | 같은 명령의 첫 두 시도 | 실패. 첫 시도는 이 머신의 기본 python3에 `pydantic`이 없어 `slack-delegate` Python 스위트가 실패했다. 둘째 시도는 `python3`를 감싼 셸 스크립트를 PATH에 둔 탓에 fake 명령을 복사하는 테스트가 `exec format error`로 실패했다. CI venv(`/tmp/issueops-ci-venv`, pydantic 2.11.7)의 bin을 PATH 앞에 두고 통과했다 |
| 스킬 | 해당 없음 | `python3 -B .issueops/issues/572/stdio_signal_smoke.py` | `stdio-signal-ok`(base에서는 `mcp exit status -15`로 실패) |
| 스킬 | 해당 없음 | `deadcode ./cmd/...`, `deadcode -test ./...`(v0.52.0) | 0줄, base와 같은 1줄(`LinkIssueOpsPlanWithActor`) |

실행하지 않은 명령(`Not Run`): 없음.

## 구현 리뷰

- 1라운드(`omp -p --model anthropic/claude-opus-5-5 --thinking high`, 빈 컨텍스트, tier `contract`, 렌즈 compat·reuse·perf·side-effect): `revise`. 필수 결함 2건. (1) 테스트 전용 위임 함수 `serveMCPStreamWithDependencies`가 production 파일에 남았다(비공개 전환이 만든 잔재). (2) 비공개로 바꾼 이름의 옛 대문자 spelling이 현행 문서와 Go doc 주석 25곳에 남았다. JSON 직렬화(명시적 field 이름·태그 불변, 임베딩 3곳은 태그 없는 inline), reflection·문자열 의존, ctx 전달 뒤 부분 상태, stdio drain의 경쟁·교착, readiness, lint-diagnose, provider는 계약을 지켰다고 판정했다.
- 수정: 위임 함수를 `mcp_test_seams_test.go`로 옮겼다(deadcode `./cmd/...` 0줄). doc 주석 25곳과 Go 주석·테스트 메시지 32곳, 현행 문서 10곳을 새 이름으로 고쳤고, 이름 변경 기록 4절에 비공개 전환 이름 304개를 더해 G9가 같은 잔재를 보게 했다.
- 2라운드(같은 모델, high, delta): `revise`. 1라운드 결함 둘은 해결됐다고 판정했다. 새 필수 결함 1건: 문서 한 줄이 아직 export된 `CommandRunner`를 소문자로 잘못 바꿨다. 한 줄을 되돌렸다.
- 3라운드(같은 모델, `xhigh`, 3라운드부터 상향 규칙): `pass`. 2라운드 결함이 해결됐고, 이전 delta의 Go 주석 57쌍과 문서 10쌍을 선언된 이름과 하나씩 대조해 반대 방향 잘못된 치환이 더 없다고 판정했다.
- 비차단 메모: 단일 연결(rwc) stdio 경로에서 클라이언트가 응답 전에 연결을 닫으면 EOF 대기가 길어질 수 있지만 production은 분리 stdio만 쓴다. 신호 종료 때 go-sdk가 `server run cancelled`를 stderr에 남긴다. `RunContext`는 `gh api` 자식이 create 잠금을 상속한다(상한이 없는 기존 동작). G9는 비공개 전환 이름 중 CamelCase 8자 이상이고 어디에도 선언되지 않은 304개만 검사한다. `domain-responsibilities.md`의 `ResumeHandler`는 base에도 선언이 없다(기존).

## side effect

- 파일: Go 소스 2,163개와 `internal/architecture/testdata` JSON 2개, 현행 문서 5개, `scripts/autoresearch` 픽스처 1개, `.issueops/issues/572/` 추적 사본·원장·측정 설정·검증 스크립트·이 보고서.
- 사용자 영향: `issueops mcp` stdio 프로세스가 SIGINT·SIGTERM을 받으면 최대 10초 뒤 종료 코드 0으로 끝난다(이전에는 신호 종료 상태). `issueops update`의 고아 MCP 정리는 종료를 기다리지 않으므로 보고 내용이 같다.
- 호환성: CLI/MCP 이름, JSON 필드, record schema, golden은 그대로다. `internal/` 아래 Go 소스 이름은 바뀐다(저장소 밖 소비자 없음).
- 원격: 이 단계에서 원격 쓰기는 없다. 8~9단계의 브랜치 push와 draft PR이 승인 범위다.
- 검증 산출물은 워크트리 밖 `/tmp/io572/`(측정 출력, 이름 변경·비공개 전환·주석 검사 도구)와 ignore되는 `.issueops/issues/572/review/`에 두었다.

## 의도 대조

| intent 성공 기준 | 덮는 게이트·증거 |
|---|---|
| 측정 설정 재측정에서 비주석·주석 지적이 0건이다 | G2a·G2b·G2c |
| build·vet·gofmt·lint-local·test·race가 통과한다 | G3·G4·G5·G6·G15 |
| golden이 갱신 없이 통과한다 | G8 |
| B군 동작 변경마다 실패-후-통과 증거가 있다 | G7·G11·G12·G14와 구현 요약 3의 테스트 |
| package clause·별칭이 정리된다 | G1·G16 |
