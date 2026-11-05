# #572 Effective Go 관례 리팩토링 계획

- lifecycle ID: `io-7426b49dc042`
- 이슈: https://github.com/m16khb-org/issueops/issues/572 (단일 이슈, `no split`)
- 브랜치: `572-effective-go-refactor`, base `main@7714640b0b40b4b03e64ad1405140d47d26e9c84`
- 사용자 요청 범위: Effective Go 관례 점검 결과의 A군(동작 보존 관례), B군(context·종료 경로·port), 문서 주석과 `IssueOps*` 이름 변경 전부. 진행 방식은 IssueOps 사이클.
- 승인된 종료점: draft PR 발행과 `execution complete`. merge와 cleanup은 별도 승인이다.
- 세션 인계: `execution prepare --confirm` 뒤 환경별 자동 인계 규칙을 따른다. 이 세션은 omp 대화형 세션이므로 Herdr를 확인하고, 쓸 수 없으면 현재 세션이 lease를 유지한 채 구현한다. 인계는 요청 범위를 넓히지 않는다.
- 승인된 아키텍처: staged `architecture` artifact(`/tmp/io572/architecture.md`)

## 적용되는 결정과 주의사항

| 문서 | 항목 | 원문 | 이 계획에 주는 제약 |
|---|---|---|---|
| `.issueops/CONSTITUTION.md:72` | 문제 해결 원칙 | "수정 뒤에는 같은 실패가 재발하지 않음을 보여주는 targeted test, command smoke, grep evidence, 로그/상태 확인 중 작업 성격에 맞는 검증을 남긴다." | 동작 변경(묶음 2)마다 실패-후-통과 테스트를, 이름 변경마다 옛 이름 grep 증거를 남긴다 |
| `.issueops/architecture/hexagonal-core.md:57` | composition root | "`cmd/issueops/issueopsapp`: concrete adapter를 조립하는 유일한 composition root다." | 인터페이스 구현 확인(`var _`)은 adapter 자기 패키지에 두고, 새 adapter import edge를 만들지 않는다 |
| `.issueops/conventions/go-and-packages.md:17` | 네이밍 | "Go 패키지명은 짧은 소문자 단어를 사용한다." | package clause를 디렉터리 이름(`issueopsremote`, `guardpattern`, `traceclassification`)에 맞춘다 |
| `.issueops/conventions/go-and-packages.md:108` | 주입 | "**주입에 default 구현을 두지 않는다.**" | `IssueProvider` fallback을 지울 때 context 없는 기본 경로를 새로 만들지 않는다 |
| `.issueops/conventions/go-and-packages.md:83` | 테스트용 위임 | "production 진입점에서 호출하지 않는 테스트용 위임 함수·타입·상수는 `*_test.go`에 둔다." | 비공개 전환·대소문자 쌍 정리에서 테스트 전용 위임을 production에 남기지 않는다 |
| `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md:13` | 이름 제거 뒤 잔재 | "제거 커밋 전에 지운 이름(명령, 플래그, 경로, 디렉터리, MCP tool)으로 git grep을 Go 문자열·usage golden·testdata·skills·.issueops 현행 문서·configs까지 돌린다." 그리고 "날짜가 붙은 기록 문서는 고치지 않고, 현행처럼 읽히는 문서만 고친다." | 이름 변경 커밋마다 옛 이름 grep을 돌리고, dated 기록은 고치지 않는다(G9) |
| `.issueops/CAUTIONS.md:55` | dated 기록 | "Dated 기록의 IssueOps 명령·필드·상태는 사고 당시 증거일 뿐 실행 지시가 아니다." | `adr/20*`, `cautions/20*`, `plans/` 등 기록 문서는 고치지 않는다 |
| `.issueops/adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md:13` | re-export | "re-export는 internal/architecture/dependency_test.go의 레이어 규칙상 정식 심볼을 직접 import할 수 없을 때만 둔다" | 옛 이름 alias를 남기지 않는다. 이미 있는 근거 있는 alias(`issueopsauthorization.Actor` 등)는 새 이름을 가리키게만 바꾼다 |
| `.issueops/adr/2026-08-08-port-contract-vocabulary.md` | port 어휘 | "port가 contract를 참조하거나 port 사이를 참조하는 것은 위반이 아니다." | `IssueProvider` 시그니처 변경은 port 안에서 contract 타입만 쓴다 |
| `.issueops/testing/unit-and-contract.md:41-42` | linter 정책 | "문체·미세 성능 linter(revive, gocritic, perfsprint, predeclared, stylecheck(ST), quickfix(QF))는 버그를 잡지 않고 코드만 흔들어서 켜지 않는다." | revive·ST는 이슈 폴더의 측정 설정으로만 돌리고 `.golangci.yml`은 바꾸지 않는다 |
| `.issueops/testing/unit-and-contract.md:78` | golden | "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다." | golden을 갱신하지 않는다. golden 변경이 필요하면 계약이 바뀐 것이므로 멈춘다(G8) |
| `.issueops/TESTING.md:46-47` | 부분 검증 금지 | "다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 첫 게이트부터 전체 재실행한다." | 최종 게이트는 한 번의 실행에서 모두 통과해야 한다 |
| `.issueops/ADR.md` | 색인 | 대조했으나 없음(위 두 ADR 외에 이 변경의 범위나 방식을 정하는 결정 없음) | 없음 |
| `.issueops/cautions/2026-08-26-ci-gofmt-gate-local-battery-drift.md:50` | 플랫폼 lint | "CI 재현은 `GOOS=linux golangci-lint run ./...`와 `GOOS=linux go vet ./...`를 함께 돌린다" | `./scripts/lint-local.sh`(GOOS=linux 포함)와 세 GOOS 빌드를 게이트에 넣는다 |
| `.issueops/testing/concurrency-and-race.md` | race | 대조했으나 이 변경에 해당하는 새 규칙 없음(취소 테스트는 race 기본 검증으로 덮는다) | `go test -race ./...`를 최종 게이트에 넣는다 |
| `.issueops/OPEN_API_SPEC.md` | API 문서 게이트 | 열지 않음: endpoint·DTO·OpenAPI를 바꾸지 않는다 | 없음 |

## 재사용하는 기존 구현

- 종료 코드 매핑 `rootSubcommandErrorExitCode`(`cmd/issueops/issueopsapp/root_command_facade.go:88-113`)와 `gatescli.UsageError`·`modelcli.UsageError` 에러 타입 패턴. lint-diagnose는 이 매핑에 `case "project"`만 더한다.
- `jsonout.Reported`/`Unreported`(`cmd/issueops/jsonout`): 이미 출력한 에러를 루트 명령이 다시 출력하지 않게 하는 기존 표지.
- `runGhJSONContext`, `runGlabGraphQLContext`: provider에 이미 있는 ctx 버전. 새 실행 함수는 `runGhAPIJSON`이 쓸 `providerutil.RunContext` 하나뿐이다. provider 패키지가 `processlease`를 직접 import할 수 없어서(`.issueops/architecture/hexagonal-core.md:196`) 기존 bounded 실행과 같은 패키지에 둔다. 상한 없는 실행이 필요한 이유는 지금 `gh api` 경로의 문구·상한을 바꾸지 않기 위해서다.
- `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` 패턴(`cmd/issueops/channelcli/channel.go:94`, `cmd/issueops/issueopsapp/mcp_http_command.go:47`).
- `processlease.Attach(ctx, cmd)`(`internal/adapter/outbound/processlease/lease.go:34`)와 `holdCreateLock`이 돌려주는 lease ctx(`publication_settlement.go:38-48`): issue create 경로(`issue_creation_environment.go:33`)와 같은 방식으로 child create에 쓴다.
- `sync.WaitGroup.Go` 팬아웃(`internal/adapter/doctor/observations.go`).
- 이름 변경 도구는 저장소 밖 일회용 프로그램이다. 저장소에 새 도구를 넣지 않는다. `golang.org/x/tools`는 이미 `go.mod`의 의존(v0.51.0)이다.

## 성능 영향

- hot path 변경 없음. 이름 변경·주석·`var _` 확인은 컴파일 결과의 동작을 바꾸지 않는다. `var _` 확인은 런타임 비용이 없다.
- context 전달은 `exec.CommandContext`의 취소 감시 goroutine 하나를 외부 명령마다 더한다. 외부 명령 실행 비용에 비해 무시할 수 있다.
- `ExitCodeError` 반환은 프로세스 종료 전 defer 실행만 더한다.
- stdio MCP의 signal 처리는 프로세스당 한 번의 등록이다. 신호가 없으면 요청 처리 경로에 비용이 없다.
- 측정 계획: 없음. 측정할 성능 주장을 하지 않는다.

## 하위 호환성과 side effect

- CLI JSON·MCP schema·response golden: 바뀌지 않는다. G8이 golden 갱신 없이 통과하는지 확인하고, `cmd/issueops/testdata`·`internal/adapter/testdata`에 diff가 없어야 한다.
- record schema·persisted bytes: Go 타입 이름은 bytes에 들어가지 않는다. 임베딩 필드는 JSON tag가 없어 inline된다. 명시적 이름의 struct field는 바꾸지 않는다.
- lint-diagnose: stdout·stderr 바이트와 종료 코드가 같다(G7 smoke).
- stdio MCP: SIGINT·SIGTERM의 동작이 바뀐다. 지금은 기본 처리로 바로 종료(신호 종료 상태)하고, 바뀐 뒤에는 처리 중인 요청을 최대 10초 기다린 뒤 취소하고 종료 코드 0으로 끝난다. HTTP 경로와 같은 종료 방식이다. go-sdk는 `server run cancelled` 진단 줄을 diagnostics에 그대로 남긴다. 두 번째 신호는 기본 처리로 바로 종료한다. 신호가 없을 때 EOF 뒤 응답을 기다리는 동작과 10분 상한은 같다. host가 stdin을 닫을 때의 동작은 바뀌지 않는다.
- provider 에러 문구: 바뀌지 않는다. `startError.Error()`는 지금 문구를 그대로 돌려준다.
- Go 소스 계약: 이 저장소 밖에서 이 패키지들을 import하는 소비자는 없다(`internal/` 아래). 롤백 단위는 8단계가 만드는 묶음별 커밋이다(아래 "커밋 방식"). 2·3번 묶음이 1번의 이름 위에 있고 마지막 커밋이 여러 묶음의 정리·문서 변경을 담으므로, 되돌릴 때는 마지막 커밋부터 역순으로 되돌린다.
- `issueops update`의 고아 MCP 정리(`internal/application/update/cleanup.go:36-42`)는 SIGTERM을 보낸 직후 `terminated`로 보고한다. 바뀐 뒤에는 고아 stdio 프로세스가 drain 상한(최대 10초) 뒤에 끝난다. 정리 기능은 종료를 기다리지 않으므로 보고 내용은 같다.
- LLM 프롬프트 변경: `internal/domain/issueopsremote/issueops_remote_judge_prompt.go:43`의 타입 이름 언급만 새 이름으로 바꾼다. 실패 기준: judge 출력 JSON의 필드 목록 문구가 바뀌거나, `remote score --judge file` 파싱 테스트(`cmd/issueops/issueopscli/issueops_remote_score_cli_test.go`)가 실패하면 프롬프트 변경이 실패한 것이다. 확인 산출물은 그 테스트 통과와 프롬프트 diff(타입 이름 한 단어만 다름)다.
- 원격·durable state: 구현 중 원격 쓰기는 없다. PR 발행은 9단계(`pr.create`) 절차를 따른다.

## 실행 단계

구현 단계(IssueOps 4단계)에서는 커밋하지 않는다(`skills/issueops-implement/SKILL.md:195-196`). 실행 단계마다 세 GOOS 빌드, `go vet ./...`, `go test ./... -count=1`을 통과한 뒤 다음 단계로 간다.

**커밋 방식**: 두 묶음이 같은 줄을 고치므로(예: `internal/application/issueopscleanup/children.go:169,185`에서 1단계는 타입 이름을, 3단계는 ctx 인자를 바꾼다) 8단계에서 hunk를 나눠서는 묶음별 커밋을 만들 수 없다. 그래서 실행 단계가 끝날 때마다 실제 index와 브랜치를 건드리지 않는 tree 스냅숏을 남긴다: `GIT_INDEX_FILE=<worktree 밖 임시 파일> git add -A && GIT_INDEX_FILE=<같은 파일> git write-tree`. 스냅숏 기록은 gitignore 대상인 `.issueops/issues/572/review/snapshots.md`에 순서대로 적는다. 줄마다 tree ID, 실행 단계 이름, 실행한 세 검증 명령과 결과를 적어 8단계 Lore의 `Verify` 근거로 쓴다(`.issueops/COMMIT_POLICY.md:66` "`Verify`에는 실제 실행한 명령을 적는다."). 8단계(`atomic-commit-push`)는 스냅숏마다 `git read-tree <tree>`(`-u` 없이, 실제 worktree를 건드리지 않는다) → `git diff --cached --stat` 확인 → `git commit`으로 커밋을 만들고, 마지막으로 현재 worktree 전체를 커밋해 5·6단계의 정리·문서 변경을 담는다. `-u`나 `git checkout <tree> -- .`는 worktree를 바꿔 정리 봉인을 stale로 만들므로 쓰지 않는다(`skills/issueops-slop-clean/SKILL.md:156-159`). 경로 단위 staging 대신 이 방식을 쓰는 이유는 위의 같은 줄 문제다.

1. **package clause와 stutter 이름** — 일회용 이름 변경 도구로 처리한다.
   1. 충돌 선처리: `issueops.MutationAck()`·`BranchPrepareAck()` → `NewMutationAck()`·`NewBranchPrepareAck()`. `policy.PolicyTier*` 4개를 지우고 참조를 기존 `policy.Tier*`로 바꾼다. 남은 `PolicyTier` 주석을 지운다. `issueopsremote.IssueOpsRemoteArtifact` → `ArtifactDraft`.
   2. package clause 3개를 디렉터리 이름으로 바꾸고 importer 별칭을 정리한다.
   3. stutter 전체(도구의 `-scan` 결과, base 기준 28개 패키지 205개와 clause 변경 뒤 `issueopsremote` 8개)를 패키지 단위 spec으로 바꾼다. `contract/issueops`를 마지막에 한다. spec은 이슈 폴더의 `renames.txt`에 누적한다.
   4. 문자열 의존 갱신: `augmentation/signals.go` 검색어와 그 검색어를 픽스처로 쓰는 테스트(`cmd/issueops/selfworkflow/augmentcatalog/self_augment_repo_signals_test.go:155` 등), `ddd_contract_function_roles.json`, judge prompt, flag 도움말(`benchmark_subcommands.go:100`), 현행 문서. 저장소 루트로 `CollectSignals`를 부르는 테스트 `TestCollectSignalsHoldOnThisRepository`를 추가하고(`Repository.ListDocs`를 채운다), base에서 참인 신호 목록을 그 테스트에 고정한다. 이 테스트는 이름 변경 전에 먼저 추가해 base에서 통과시킨다.
2. **대소문자 쌍·에러·인터페이스 확인·나머지 관례** — 측정 설정 기준(세 GOOS) `confusing-naming` 전부(darwin 24건과 windows·linux 전용 파일 4건. production 이름과 테스트 helper가 겹치는 `hookcli.RunHook`·`runHook`, `modelcli.Run`·`run`은 테스트 helper 이름을 바꾼다), early-return(`update/runtime_windows.go:288`), 에러 sentinel·`errors.Is`, `var _` 확인, 내장 식별자 가림, if-return, else 체인, 중복 import, blank import 주석, `wg.Go`, `jobNotFoundError`, `nestHTTPStatusNames`.
   - 비목표 위치의 측정 지적은 근거를 적은 지시문으로 남긴다(완료 기준 1). 저장소 설정의 `nolintlint`가 staticcheck 지시문을 "unused"로 보므로(`.golangci.yml`은 staticcheck `SA*`·`S1*`만 켠다) 형식을 나눈다.
     - ST1005 4건(`internal/adapter/claude/activation.go:21`, `internal/adapter/codex/activation.go:26,37`, `internal/domain/issueops/handoff_delivery.go:603`): `//nolint:staticcheck,nolintlint // <고유 명사로 시작하는 에러 문자열>`.
     - `readiness_service.go`의 copy-on-write 값 리시버: `s = s.scoped()`를 `scoped := s.scoped()`로 바꿔 수신자에 다시 대입하지 않는다. 그러면 `modifies-value-receiver` 지적이 사라지므로 지시문이 필요 없다. `Plan`의 `:34` `s.scoped()` 호출은 그대로 둔다. 공개 연산마다 observation scope를 정확히 하나 만든다는 계약을 `TestReadinessPublicOperationsCreateOneScope`(`readiness_scope_test.go:19,44`)가 고정하고, `Plan`은 이 호출로만 그 계약을 지킨다. 아키텍처의 "버려지는 `s.scoped()` 호출" 항목은 이 근거로 고치지 않는다.
     - `internal/testsupport/fakecmd/main.go:66`의 `deep-exit`: `fail`이 종료 코드를 돌려주고 `main`이 `os.Exit`를 부르게 고친다.
3. **context·종료 경로·port** — RED 테스트부터 쓴다.
   - `TestLintDiagnoseReturnsChildExitCodeWithoutExtraStderr`(`cmd/issueops/issueopsapp`): 자식 종료 코드 7, stderr 마지막 줄이 구분선.
   - `TestBoundedCommandStartErrorUnwrapsCause`(`internal/adapter/provider/providerutil`): 없는 실행 파일로 `errors.Is(err, exec.ErrNotFound)`가 참이고 문구는 그대로.
   - `TestRunContextStopsOnCanceledContext`(`providerutil`): 시작 전에 취소된 ctx면 `cmd.Start`가 돌려주는 `ctx.Err()`를 그대로 돌려주고(`os/exec`의 `Start`), 실행 중에 취소되면 자식을 끝내고 `*exec.ExitError`를 감싸지 않고 돌려준다. provider 패키지는 `processlease`를 import하지 않는다.
   - stdio MCP(`cmd/issueops/mcpcli`): 아키텍처 계약 표의 종료 방식 (1)~(7). `server.Run`에는 `base`를 넘긴다. serve ctx가 끝나면 분리 stdio에 이미 있는 `inflightRequests`(`mcp_stdio_drain.go`)가 0이 될 때까지 최대 10초 기다린 뒤 `base`를 취소한다. 그래서 drain 중에 끝난 요청의 응답이 HTTP 경로처럼 전달된다. EOF 뒤 `waitIdle`도 `base`에 묶어 신호 뒤 drain 규칙이 EOF 전후에 같다. 하나의 연결 stdio(`io.ReadWriteCloser`) 경로도 `drainingReader`와 `observingWriter`를 거치게 해 in-flight 추적 방식을 하나로 둔다(production `issueops mcp`는 분리 stdio 경로를 쓴다, `mcp_facade.go:26`). 아키텍처 (6)은 `waitIdle`을 serve ctx에 묶는다고 적었지만, 계획은 r4 리뷰 메모 2에 따라 `base`에 묶어 EOF 전후의 drain 규칙을 같게 한다. 차이는 EOF 뒤 신호의 drain 시간(즉시 → 최대 10초)뿐이다. go-sdk의 `server run cancelled` 진단 줄은 stderr에 남는다. 테스트 세 개.
     - `TestServeMCPStreamCancelsInFlightRequestOnShutdown`: stdin을 `io.Pipe`로 열어 둔 채 ctx를 기다리는 도구 호출이 처리 중일 때 serve ctx를 취소하면, 짧게 준 drain 상한 안에 nil로 반환한다.
     - `TestServeMCPStreamStopsDrainOnContextCancel`: 응답하지 않은 요청이 남은 채 EOF가 온 뒤 serve ctx를 취소해도 drain 상한 안에 nil로 반환한다.
     - 신호 연결은 하위 프로세스 smoke(G14)로 확인한다. 테스트 프로세스에 직접 신호를 보내지 않는다. smoke는 drain 10초 때문에 10초 넘게 걸리고 POSIX 신호를 쓰므로 Windows에서는 돌리지 않는다. smoke는 stderr 내용을 검사하지 않고 종료 코드와 시간만 본다.
     - 기존 `mcp_stdio_drain_test.go`의 `waitIdle` 테스트는 바뀐 시그니처에 맞춘다. `TestServeMCPStreamContextCancelsIdleSDKSession`(`mcp_stream_test.go:16-31`)은 처리 중인 요청이 없으므로 그대로 1초 안에 통과해야 한다.
   - `TestVerifyChildHierarchyStopsOnCanceledContext`(github·gitlab 각각): fake `gh`·`glab`이 실행되면 표지 파일을 남기게 하고, 취소된 ctx로 `Provider.VerifyChildHierarchy`를 부른 뒤 표지 파일이 없는지 본다. base는 ctx를 버리므로(`provider_body_sync.go:88`, `:112`) 표지가 생겨 RED다. gitlab의 `runGlabGraphQLContext` 자체 취소는 이미 `TestRunGlabHelpersRespectCanceledContext`(`gitlab/provider_test.go:684-699`)가 덮으므로 따로 쓰지 않는다.
   - `TestLockedChildCreatorPassesLeaseContext`(`internal/adapter/issueops`): adapter `ChildCreator`와 application `ChildProvider`(`internal/application/issueopsremote/child_create.go:26-28`)가 ctx를 받고, lock이 돌려준 ctx가 provider까지 간다.
   - doctor `ProbeGatewayHTTP`·`runLsof`, `issue_graph_poster.go`, `issueops_artifact_observation.go`의 취소 테스트.
   - 그 뒤 `IssueProvider` 시그니처 통일과 fallback 제거, fake 갱신.
   - `install_native_path_unix.go:78-86`의 `Close` 에러 처리는 하지 않는다. 로컬 파일을 `O_APPEND`로 연 뒤의 `Close` 실패를 재현할 seam이 없어 실패-후-통과 증거를 만들 수 없고, 실패 경로의 차이도 없다. 이슈 본문 비목표에 근거를 옮긴다.
4. **비공개 전환과 문서 주석** — 패키지 단위 작업자 풀로 나눈다. 한 패키지는 한 작업자만 맡는다. 작업자는 `ddd_contract_function_roles.json`을 고치지 않고, 메인 세션이 결과를 모은 뒤 고친다. 비공개 전환 제외: struct field, 다른 패키지의 테스트가 쓰는 이름, 인터페이스 구현 메서드, composition root가 조립하는 이름, `signals.go`가 찾는 이름, 패키지 밖에서 부르는 exported 함수·메서드가 돌려주는 타입(revive `unexported-return`), `internal/architecture/testdata/ddd_responsibility_inventory.json`의 policy 심볼이 가리키는 타입(`Environment.VerifyArtifact`, `SQLiteRepository.CommitDirect`, `TemplateBodyResolver.ScoreSummary`의 수신 타입). 비공개 이름이 같은 범위의 타입·이름과 겹치면(예: `install.TransactionEffects`, `gatescli.UnmetError`, `sqlstore.SpanObserver`) 비공개로 바꾸지 않고 주석을 단다. windows·linux 전용 파일도 작업자 배정에 넣는다.
5. **재측정과 전체 검증** — 아래 게이트를 한 번의 실행에서 모두 통과시킨다.

## 게이트

측정 설정 `.issueops/issues/572/effective-go-lint.yml`은 IssueOps 구현 단계(4단계) 진입 때 `gates init`과 함께 만든다(revive 규칙과 staticcheck ST 계열, `_test.go` 제외). 이 파일은 이슈 폴더의 추적 산출물이며 `.golangci.yml`을 바꾸지 않는다.

- G1: package clause가 디렉터리 이름과 같다 | CHECK: python3 -B -c "import subprocess;o=subprocess.run(['go','list','-f','{{.Name}}','./internal/domain/issueopsremote','./internal/domain/guardpattern','./internal/domain/traceclassification'],capture_output=True,text=True,check=True).stdout.split();print(' '.join(o))" | EXPECT: issueopsremote guardpattern traceclassification
- G2a: Effective Go 측정 설정의 production 지적이 darwin에서 0건이다 | CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'darwin'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])" | EXPECT: effective-go-clean
- G2b: Effective Go 측정 설정의 production 지적이 linux에서 0건이다 | CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'linux'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])" | EXPECT: effective-go-clean
- G2c: Effective Go 측정 설정의 production 지적이 windows에서 0건이다 | CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'windows'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])" | EXPECT: effective-go-clean
- G3: 저장소 lint 게이트가 darwin과 linux에서 통과한다 | CHECK: ./scripts/lint-local.sh | EXPECT: 0 issues.
- G4: 세 GOOS 빌드와 vet이 통과한다 | CHECK: python3 -B -c "import os,subprocess,sys;[subprocess.run(['go',c,'./...'],env={**os.environ,'GOOS':g},check=True) for g in ('darwin','linux','windows') for c in ('build','vet')];print('builds-ok')" | EXPECT: builds-ok
- G5: gofmt 대상 파일이 없다 | CHECK: python3 -B -c "import subprocess;fs=subprocess.run(['git','ls-files','*.go'],capture_output=True,text=True,check=True).stdout.split();out=subprocess.run(['gofmt','-l',*fs],capture_output=True,text=True).stdout.strip();print(out or 'gofmt-clean')" | EXPECT: gofmt-clean
- G6: 전체 테스트가 race 검사와 함께 통과한다 | CHECK: go test -race ./... -count=1 | EXPECT: /ok\s+issueops/
- G15: 전체 테스트가 race 없이도 통과한다 | CHECK: go test ./... -count=1 | EXPECT: /ok\s+issueops/
- G16: 세 패키지의 importer 별칭이 한 가지로 정리된다 | CHECK: python3 -B .issueops/issues/572/check_import_aliases.py | EXPECT: aliases-clean
- G7: lint-diagnose가 자식 종료 코드를 그대로 돌려주고 stderr에 줄을 더하지 않는다 | CHECK: python3 -B -c "import subprocess;subprocess.run(['go','build','-o','bin/issueops','./cmd/issueops'],check=True);p=subprocess.run(['bin/issueops','project','lint-diagnose','--','sh','-c','exit 7'],capture_output=True,text=True);print(p.returncode,p.stderr.rstrip().splitlines()[-1])" | EXPECT: 7 ------------------------------------
- G8: response-contract golden이 갱신 없이 통과하고 testdata diff가 없다 | CHECK: python3 -B -c "import subprocess;subprocess.run(['go','test','./cmd/issueops/contractgolden','./cmd/issueops/issueopsapp','-run','Golden','-count=1'],check=True);d=subprocess.run(['git','diff','--name-only','7714640b0b40b4b03e64ad1405140d47d26e9c84','--','cmd/issueops/testdata','internal/adapter/testdata'],capture_output=True,text=True,check=True).stdout.strip();print(d or 'golden-unchanged')" | EXPECT: golden-unchanged
- G9: 현행 문서와 코드에 옛 이름이 없고 dated 기록은 바뀌지 않았다 | CHECK: python3 -B .issueops/issues/572/check_renames.py | EXPECT: renames-clean
- G10: 저장소 신호가 이름 변경 뒤에도 참이다 | CHECK: go test ./internal/adapter/augmentation -run TestCollectSignalsHoldOnThisRepository -count=1 -v | EXPECT: --- PASS: TestCollectSignalsHoldOnThisRepository
- G11: provider 시작 실패 에러가 원인을 보존한다 | CHECK: go test ./internal/adapter/provider/providerutil -run TestBoundedCommandStartErrorUnwrapsCause -count=1 -v | EXPECT: --- PASS: TestBoundedCommandStartErrorUnwrapsCause
- G12: stdio MCP가 종료 ctx에서 처리 중인 요청과 EOF 대기를 끝낸다 | CHECK: go test ./cmd/issueops/mcpcli -run TestServeMCPStream -count=1 -v | EXPECT: --- PASS: TestServeMCPStreamCancelsInFlightRequestOnShutdown
- G14: 실제 `issueops mcp` stdio 프로세스가 `channel_recv` 대기 중 SIGTERM에 drain 상한 안에 종료 코드 0으로 끝난다 | CHECK: python3 -B .issueops/issues/572/stdio_signal_smoke.py | EXPECT: stdio-signal-ok
- G13: provider 패키지가 processlease를 직접 import하지 않는다 | CHECK: go test ./internal/architecture -run TestProductionGraphHasNoForbiddenAdapterEdges -count=1 -v | EXPECT: --- PASS: TestProductionGraphHasNoForbiddenAdapterEdges

`check_renames.py`, `check_import_aliases.py`, `stdio_signal_smoke.py`는 IssueOps 구현 단계(4단계) 진입 때 이슈 폴더에 만든다. `check_import_aliases.py`는 세 패키지를 import하는 production·테스트 파일의 별칭 집합을 모아, 별칭이 없거나, 같은 파일에서 이름이 겹칠 때만 둔 별칭이면서 그 별칭 이름이 패키지마다 하나뿐이면(근거를 스크립트 표에 적는다) `aliases-clean`을 출력한다. smoke는 임시 `ISSUEOPS_STATE_DIR`로 빌드한 바이너리의 `mcp`를 띄워 `initialize`와 `channel_recv`(wait) 요청을 보낸 뒤 SIGTERM을 보내고, 20초 안에 종료 코드 0이면 `stdio-signal-ok`를 출력한다. `channel_recv`는 authority 파일이 필요 없는 server tool이다(`mcp_tool_authority.go:102`). `check_renames.py`는 이름 변경 spec 파일(같은 폴더의 `renames.txt`)의 옛 이름을 Go 파일·testdata·skills·configs·`.issueops` 현행 문서에서 검색하고, dated 기록(`adr/20*`, `cautions/20*`, `archive/`, `issues/`, `verified-execution/`, `evidence/`, `research/`, `plans/`)의 base 대비 diff가 없는지 확인한다. 이 이슈의 폴더(`.issueops/issues/572/`)와 색인 표에 옮겨 적은 dated 기록 제목 행은 제외한다. 그대로 두는 같은 이름의 struct field(예: `StateDir`)나 다른 객체와 겹치는 옛 이름은 `renames.txt`에 `# keep: <근거>`로 표시해 검색에서 뺀다. 문서 픽스처(`scripts/autoresearch/doc_injection_fixtures/`)는 현행 문서로 보고 고친다.

## 검증

게이트는 worktree에서 `issueops gates check --file .issueops/issues/572/gates.md --cwd "$WORKTREE" --workspace-root "$WORKTREE" --env HOME,PATH,LANG,GOPATH,GOMODCACHE,GOCACHE --timeout-seconds 900 --json`으로 실행한다(G4·G6·G15가 120초 기본 상한을 넘고, CHECK당 정책 상한은 15분이다: `internal/domain/policy/command_decision.go:77-78`, `.issueops/cautions/2026-10-18-gate-checks-that-run-go-tests-or-self-verify-need-lang-and-t.md`). 게이트 외에 따로 실행하는 명령은 보고용 측정 하나다: base와 head에서 GOOS마다 `golangci-lint run -c .issueops/issues/572/effective-go-lint.yml ./...`의 지적 건수를 센다(base는 측정 설정을 worktree 밖 임시 경로에 복사해 base 체크아웃에서 돌린다). 최종 보고에는 G1~G16(G2는 G2a·G2b·G2c)을 한 번에 통과한 `gates check` 결과와 base·head의 측정 수치(`effective-go-lint.yml` 지적 건수)를 적는다.
