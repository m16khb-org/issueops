# #572 Effective Go 관례 리팩토링 아키텍처

lifecycle ID: `io-7426b49dc042`, 브랜치 `572-effective-go-refactor`, base `main@7714640b`.

## 요구사항과 규모

기능 요구사항:

- R1. Go 식별자·패키지 이름을 Effective Go "Names"·"Package names" 절에 맞춘다. 대상은 package clause 3개, stutter 이름(패키지 이름 접두어 뒤에 대문자나 `_`가 오는 package-level exported 이름) 전부, revive `confusing-naming`이 보고하는 대소문자만 다른 쌍 전부, 내장 식별자 가림이다. stutter 대상은 주석 유무와 상관없이 같은 규칙으로 뽑는다. revive `exported`는 주석이 없는 타입의 stutter를 보고하지 않으므로(`internal/contract/worker/types.go:9` `WorkerJob`은 주석 누락만 보고된다) revive 결과 29건을 목록으로 쓰지 않는다.
- R2. 에러·인터페이스 관례를 지킨다. 같은 패키지에서 만든 에러는 문자열이 아니라 `errors.Is`로 비교하고, port를 구현하는 concrete 타입은 `var _ port.X = T{}`로 컴파일 시점에 확인한다.
- R3. 호출자의 context가 provider 생성 호출, 네트워크 외부 명령, HTTP probe까지 전달된다. 명령 내부의 `os.Exit`와 취소를 보지 않는 대기를 없앤다.
- R4. production 코드의 exported 식별자와 패키지에 문서 주석이 있다.

비기능 요구사항:

- CLI 명령·MCP tool 이름, JSON 필드, persisted record bytes, response golden이 바뀌지 않는다. 이 이슈의 변경은 Go 소스 수준의 계약만 바꾼다.
- 레이어 의존 규칙(`internal/architecture/dependency_test.go`)을 그대로 통과한다.

규모(2026-11-04 `main@7714640b` 실측):

| 항목 | 값 | 측정 방법 |
|---|---|---|
| production Go 코드 | 128,054줄 | `git ls-files '*.go'`에서 `_test.go` 제외, `wc -l` |
| stutter 이름(타입·함수·상수·변수, package clause 변경 뒤 `issueopsremote` 포함) | 28개 패키지 205개 + `domain/issueopsremote` 8개 | 일회용 도구의 세 GOOS 스캔(`contract/issueops` 105, `contract/selfaugment` 26, `contract/worker` 8, `contract/state` 7, 그 밖 24개 패키지 59) |
| 그중 `contract/issueops` 이름의 참조 | 788개 파일, 7,765회 | 105개 이름의 토큰 검색 |
| 접두어를 떼면 기존 이름과 겹치는 것 | `issueops.MutationAck`·`BranchPrepareAck`(함수), `policy.PolicyTier*` 4개(이미 같은 값의 `Tier*` 상수가 있음), `policy.PolicyTier` 주석만 남은 선언, `issueopsremote.IssueOpsRemoteArtifact` → `Artifact`(같은 패키지의 다른 struct `Artifact`, `artifact.go:14`와 겹침) | 같은 패키지 이름 비교 |
| 대소문자만 다른 쌍 | production 22건 | revive `confusing-naming` |
| 문서 주석이 없는 exported 식별자 | 3,053개 | revive `exported` |
| 패키지 주석이 없는 파일 | 1,029개 | staticcheck ST1000 |

이 규모 때문에 이름 변경은 사람이 고치지 않고 타입 정보로 참조를 찾는 일회용 도구로 한다(구조와 흐름 1번). 문서 주석은 패키지 단위로 나눠 작성한다.

## 데이터와 API 계약

바뀌지 않는 계약:

- CLI·MCP 표면: 명령 목록, tool 목록, 입력 schema, 응답 필드. `cmd/issueops/testdata/*.golden.*`과 `response_contracts.golden.json`을 갱신하지 않는다.
- persisted record: JSON tag가 필드 이름을 정한다. Go 타입 이름은 bytes에 들어가지 않는다. `internal/contract`에서 `reflect`를 쓰는 곳은 테스트뿐이고(`internal/contract/issueopslease/record_preservation_test.go:18`), golden·testdata에는 `IssueOps*` 타입 이름이 없다.
- 에러 문구: provider 시작 실패, lint-diagnose 출력, hostprobe 코드 문자열(`probe_result_too_large` 등)은 문구를 그대로 둔다.

바뀌는 Go 계약:

| 대상 | 변경 전 | 변경 후 |
|---|---|---|
| stutter 이름 | `issueops.IssueOpsRecord`, `selfaugment.SelfAugmentPlan`, `state.StateDir`, `worker.WorkerJob` … | 접두어를 뗀 이름(`issueops.Record`, `selfaugment.Plan`, `state.Dir`, `worker.Job`). 겹치면: 같은 값의 별칭이면 하나로 합친다(`policy.PolicyTierReadOnly` → 기존 `policy.TierReadOnly`). 다른 것이면 기존 이름을 먼저 바꾸거나 새 이름에 뜻을 더한다(함수 `issueops.MutationAck()`·`BranchPrepareAck()` → `NewMutationAck()`·`NewBranchPrepareAck()`, `IssueOpsRemoteArtifact`{Provider, Title, Body} → `ArtifactDraft`) |
| 임베딩 필드 | `IssueOpsBranchPrepareAck{IssueOpsMutationAck; …}` | `BranchPrepareAck{MutationAck; …}`. 임베딩 필드에는 JSON tag가 없어서 필드가 inline되므로 JSON은 같다 |
| package clause | `internal/domain/issueopsremote` → `package remote`, `guardpattern` → `pattern`, `traceclassification` → `classification` | 디렉터리 이름과 같게 바꾼다(`go-and-packages.md:17` "Go 패키지명은 짧은 소문자 단어를 사용한다."와 기존 `<layer>/<capability>` 디렉터리 배치를 유지). 그러면 `issueopsremote.IssueOpsRemote*` 8개가 stutter가 되므로 같은 묶음에서 접두어를 뗀다. importer는 별칭 없이 쓰고, 같은 이름이 충돌하는 파일만 별칭을 둔다 |
| `port.IssueProvider` | `CreateIssue(req)`, `CreatePullRequest(req)`, `CreateChild(req)`, `CloseChild(req)`와 선택 인터페이스 `IssueProviderCreateIssueContexter`, 비공개 `contextPullRequestCreator`·`contextPullRequestReconciler` | 네 메서드가 모두 `ctx context.Context`를 첫 인자로 받는다. 선택 인터페이스 세 개와 fallback을 지운다. `IssueProviderRemoteCreateReconciler.ReconcilePullRequest`도 ctx를 받는다. `optional로 둔 이유` 주석(`internal/port/provider.go:349-353`)을 사실에 맞게 고친다 |
| provider 내부 실행 함수 | GitHub `runGhAPIJSON`(`provider.go:456-474`, ctx 없음), `runGhJSON`(`provider.go:425-427`, `context.Background()` 고정), GitLab `runGlabGraphQL`(`provider_child.go:586-588`, Background 고정)과 `VerifyChildHierarchy(_ context.Context, …)`(github `provider_body_sync.go:88`, gitlab `provider_body_sync.go:112`) | 이미 있는 `runGhJSONContext`·`runGlabGraphQLContext`를 호출자 ctx로 부르고, `context.Background()`를 고정하는 wrapper `runGhJSON`·`runGlabGraphQL`은 지운다. `runGhAPIJSON`은 ctx를 받아 `providerutil`에 새로 두는 상한 없는 실행 함수 `RunContext(ctx, repo, name, args...) ([]byte, error)`를 부른다. 이 함수가 `exec.CommandContext`, `processlease.Attach`(`internal/adapter/outbound/processlease/lease.go:34`), `cmd.Output()`을 쓰고 `*exec.ExitError`를 그대로 돌려준다. provider 패키지는 `processlease`를 import하지 않는다(`.issueops/architecture/hexagonal-core.md:196` "`isProcessLifetimeEdge`는 `outbound/processlease`를 두 명령 실행 패키지(`internal/adapter/issueops`, `internal/adapter/provider/providerutil`)에만 허용한다."). LookPath 사전 확인은 그대로다. 에러 문구(`gh api %s failed: <stderr>` 등), timeout 없음, 출력 상한 없음은 그대로 둔다. bounded command로 바꾸지 않는다. `VerifyChildHierarchy`는 받은 ctx를 이 함수들에 넘긴다 |
| `LockedChildCreator` | `HoldIssueCreate(context.Background(), …)`가 돌려준 lease ctx를 버림(`issue_create_settlement.go:137`) | issue create 경로(`issue_creation_environment.go:33`)와 같이 호출자 ctx로 lock을 잡고 돌려받은 ctx를 `CreateChild`에 넘긴다. adapter 안의 `ChildCreator` 인터페이스, application의 `ChildProvider`(`internal/application/issueopsremote/child_create.go:26-28`)와 그 호출(`:78`), "provider call has no context" 주석을 함께 고친다 |
| 외부 명령 observer | `issue_graph_poster.go`, `issueops_artifact_observation.go`의 `exec.Command`, doctor `runLsof`, `ProbeGatewayHTTP(target)` | 호출자의 ctx를 받아 `exec.CommandContext`·`http.NewRequestWithContext`를 쓴다 |
| stdio MCP 진입점 | `RunMCPWithDependencies`가 `context.Background()`로 서버를 띄우고(`cmd/issueops/mcpcli/mcp_transport.go:13-18`), 입력 EOF 뒤 `waitIdle`(`mcp_stdio_drain.go:76-93`)이 ctx를 보지 않고 최대 10분 기다린다. SIGINT·SIGTERM은 기본 처리로 프로세스를 바로 끝낸다. go-sdk `IOTransport`는 취소를 요청 ctx로 전하지 않으므로(jsonrpc2 `conn.go:232-234`의 `notDone`), 서버 ctx를 취소해도 처리 중인 요청은 끝나지 않는다 | HTTP 경로의 종료 방식(`mcp_http.go:212-241`)을 따른다. (1) 진입점이 `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`(`channelcli/channel.go:94`, `issueopsapp/mcp_http_command.go:47`과 같은 신호 집합)로 serve ctx를 만든다. (2) 요청 ctx의 바탕이 되는 `base`를 `context.WithCancel(context.WithoutCancel(ctx))`로 만들고, `server.AddReceivingMiddleware`로 모든 요청 ctx를 `base`가 취소될 때 함께 취소되게 감싼다. (3) serve ctx가 끝나면 처리 중인 요청을 `httpShutdownDrain`(10초)까지 기다린 뒤 `base`를 취소한다. (4) 그 뒤에도 ctx를 보지 않는 handler 때문에 `server.Run`이 돌아오지 않으면 짧은 상한 뒤 반환한다. (5) 신호로 끝난 종료는 nil을 돌려줘 HTTP 경로처럼 종료 코드 0이고 stderr에 `mcp: context canceled`를 쓰지 않는다. (6) `drainingReader`의 EOF 대기 `waitIdle(ctx, limit)`도 serve ctx에서 끝난다. (7) 첫 신호 뒤 `stop()`으로 기본 처리를 되돌려 두 번째 신호는 바로 종료한다. 신호가 없을 때의 응답 대기 동작과 10분 상한은 그대로다 |
| `projectcli` lint-diagnose | `os.Exit(result.ExitCode)` | `jsonout.Reported(ExitCodeError{Code: result.ExitCode})`를 돌려준다. Reported 표지가 있어서 루트 명령(`cmd/issueops/rootcmd/root_command.go:115-130`)이 `project: <문구>` 줄을 stderr에 더 쓰지 않는다. `rootSubcommandErrorExitCode`(`cmd/issueops/issueopsapp/root_command_facade.go:88-113`)는 `:89`에서 표지를 벗긴 뒤 `case "project"`에서 그 코드를 돌려준다. stdout·stderr 바이트와 종료 코드는 지금과 같다 |

옛 이름의 alias는 남기지 않는다. `.issueops/adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md`의 결정 "re-export는 internal/architecture/dependency_test.go의 레이어 규칙상 정식 심볼을 직접 import할 수 없을 때만 둔다"를 따른다.

## 구조와 흐름

작업은 세 묶음을 순서대로 진행하고 묶음마다 커밋을 나눈다. 뒤 묶음은 앞 묶음이 확정한 이름과 시그니처 위에서 작업한다.

```mermaid
flowchart LR
  A["1. 동작 보존 관례·이름 정리"] --> B["2. context·종료 경로·port"] --> C["3. 문서 주석"] --> D["재측정·전체 검증"]
```

**1. 동작 보존 관례·이름 정리**

- 이름 변경 도구: 저장소 밖(`/tmp`)의 일회용 Go 프로그램이 `golang.org/x/tools/go/packages`(저장소가 이미 쓰는 v0.51.0)로 `GOOS=darwin`, `linux`, `windows` 세 설정을 테스트 파일까지 포함해 불러온다. 바꿀 객체(package-level 이름과 그 이름을 쓰는 임베딩 필드)를 가리키는 식별자 위치를 세 설정에서 모두 모아 한 번에 고친다. `gopls rename`은 현재 GOOS의 파일만 보므로 `*_windows.go`·`*_linux.go`의 참조를 놓친다(`processlease`의 `acquire_unix.go:32`와 `acquire_windows.go:52`가 그 예). 명시적 이름을 가진 struct field는 바꾸지 않는다.
- stutter 목록은 이 도구가 같은 규칙으로 뽑은 결과를 쓴다. 겹치는 이름은 데이터와 API 계약 표의 규칙대로 먼저 처리한다.
- 바꾼 이름을 문자열로 담는 곳을 같은 커밋에서 고친다.
  - `internal/adapter/augmentation/signals.go`의 소스 검색어(`:146-159`의 `service.writeRecord(`, `func (command Command) RunCleanupStuck(`, 상수 이름 `WorkerStatusFailed` 등). 이 검색어는 실제 저장소에서만 의미가 있으므로, base에서 참인 저장소 신호가 head에서도 참인지 저장소 루트로 `CollectSignals`를 부르는 테스트로 고정한다. 이 테스트는 `Repository.ListDocs`를 채운다(`DocsContainTerm`이 바로 부른다, `search.go:9-10`). 검색어를 바꾸면 그 검색어를 픽스처로 쓰는 테스트(`self_augment_repo_signals_test.go:155`의 `const WorkerStatusFailed` 등)도 바꾼다.
  - `internal/architecture/testdata/ddd_contract_function_roles.json`의 함수 키(`:619`, `:624` 등).
  - `internal/domain/issueopsremote/issueops_remote_judge_prompt.go:43`처럼 프롬프트에 Go 타입 이름을 적은 곳은 새 이름으로 바꾼다.
  - flag 도움말 등 Go 문자열의 옛 타입 이름(예: `benchmarkcmd/benchmark_subcommands.go:100`의 `<IssueOpsBenchmarkScore>`).
  - 현행 문서(`.issueops/` 색인·모듈, `skills/`, `AGENTS.md`, README)의 옛 이름. base에서 해당하는 곳은 `.issueops/conventions/state-policy-and-hooks.md` 같은 현행 모듈이다. `.issueops/CAUTIONS.md`·`ADR.md` 색인 표에 옮겨 적은 dated 기록의 제목(예: `CAUTIONS.md:109`)은 그 기록과 같게 두고 바꾸지 않는다. 날짜가 붙은 기록은 고치지 않는다: `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md:13` "날짜가 붙은 기록 문서는 고치지 않고, 현행처럼 읽히는 문서만 고친다." 제외 대상은 `adr/20*`, `cautions/20*`, `archive/`, `issues/`, `verified-execution/`, `evidence/`, `research/`, `plans/`다.
- 대소문자만 다른 쌍: revive `confusing-naming`의 production 22건을 모두 처리한다. 비공개 쪽이 다른 일을 하면 그 이름을 바꾸고(예: `application/state.read` → `readIn`, `authority.check` → `checkBound`, `issueopsstatus.derive` → `deriveEntry`, `audit.append` → `writeRecord`), 한 줄 위임만 하는 쌍은 합친다(`installcli`의 `Apply`·`Rollback`·`Finalize`, `RunInstall`). signals.go가 찾는 이름(`application/state.writeRecord`)을 바꾸면 검색어도 바꾼다.
- 에러 비교: `hostprobe/runner.go:384`의 `fmt.Errorf("evidence_file_too_large")`를 sentinel로 바꾸고 `:318`, `:365`에서 `errors.Is`로 비교한다. `judgement/structured.go:74`는 `errors.Is(err, io.EOF)`를 쓴다.
- 인터페이스 구현 확인: GitHub·GitLab provider가 구현하는 선택 인터페이스 전부, `HostInstaller`를 구현하는 다섯 installer, `CodexRunner`·`ClaudeRunner`, `mcpservice.Service`에 `var _` 확인을 둔다. 테스트 파일에만 있던 확인은 production 파일로 옮긴다.
- 나머지: 내장 식별자 가림 7곳, if-return 3곳, else 체인 2곳, 중복 import, blank import 이유 주석, `readiness_service.go:34`의 버려지는 `s.scoped()` 호출, `wg.Go` 2곳, 오류 타입 이름(`jobNotFound` → `jobNotFoundError`), `nestHttpStatusNames`.

**2. context·종료 경로·port**

- `IssueProvider`: 두 provider는 이미 context 메서드를 구현한다(github `provider.go:34,125,381`, gitlab `provider.go:36,111,290`). 이 구현을 인터페이스 메서드 자체로 올리고, `remote_provider_guard.go:14-75`의 fallback을 지운다. `CreateChild`·`CloseChild`는 ctx를 받아 provider 내부 실행 함수(`runGhAPIJSON`, `runGhJSONContext`, `runGlabGraphQLContext`)에 넘긴다. ctx 전달만 더하고 에러 문구·상한은 그대로 둔다. cleanup의 `closeChild`(`internal/application/issueopscleanup/children.go:185`)와 `observeChildState`(`:169`)가 `Close`의 ctx를 넘기고, 테스트 fake(예: `feedback_cleanup_test.go:542-551`)도 같은 시그니처로 바꾼다.
- `providerutil`에 `RunContext`를 둔다(위 계약 표). `runGhAPIJSON`만 이 함수를 쓰고, 이미 bounded 실행을 쓰는 `runGhJSONContext`·`runGlabGraphQLContext` 경로는 취소 동작만 달라진다.
- `providerutil/bounded_command.go:81`: 시작 실패를 `startError{text, cause}`로 감싼다. `Error()`는 지금과 같은 `command start failed: <boundedDiagnostic>` 문구를 돌려주고 `Unwrap()`은 원래 에러를 돌려준다. provider는 `strings.Contains(err.Error(), "executable file not found")` 대신 `errors.Is(err, exec.ErrNotFound)`를 쓴다(github `provider.go:437`, gitlab `provider_child.go:433`).
- lint-diagnose: 프롬프트를 stderr에 쓴 뒤 `jsonout.Reported(ExitCodeError{Code: result.ExitCode})`를 돌려준다. Reported 표지 때문에 루트 명령이 stderr에 줄을 더 쓰지 않고, 종료 코드 매핑의 `case "project"`가 그 코드를 쓴다. 다른 명령은 이 매핑의 영향을 받지 않는다.
- stdio MCP: 계약 표의 (1)~(7)을 `ServeMCPStreamContextWithDependencies`(`mcp_sdk_server.go:385-400`)와 진입점에 구현한다. drain 상한은 테스트가 짧게 줄 수 있도록 내부 함수의 인자로 받는다. 테스트는 세 가지다. stdin을 열어 둔 채 ctx를 기다리는 도구 호출이 처리 중일 때 serve ctx를 취소하면 drain 상한 안에 nil로 반환한다. 응답하지 않은 요청이 남은 채 EOF가 온 뒤 취소해도 같다. 하위 프로세스로 `issueops mcp`를 띄워 `channel_recv`(wait)가 처리 중일 때 SIGTERM을 보내면 drain 상한 안에 종료 코드 0으로 끝난다(신호 연결 증거). 수정 전에는 첫 둘은 상한까지 기다리고, 셋째는 처리기가 없어 신호로 죽으므로(종료 코드가 0이 아님) RED다.
- 외부 명령·HTTP: ctx가 이미 있는 호출 경로만 바꾼다. ctx가 없는 로컬 git 실행 12곳은 바꾸지 않는다. MCP HTTP 요청은 요청 ctx가 클라이언트 연결 종료로 취소되므로, 이 경로의 provider·외부 명령 호출이 실제 취소 대상이다.
- 쓰기 파일: `install_native_path_unix.go:78-86`은 `Close` 에러를 확인한 뒤에만 `Written = true`로 둔다.

**3. 문서 주석**

- 패키지 단위로 나눠 주석을 쓴다. 한 패키지는 한 작업자만 맡는다.
- 먼저 패키지 밖에서 쓰지 않는 exported 이름을 비공개로 바꾼다. 대상은 revive `exported`가 보는 package-level 타입·함수·상수·변수와 메서드뿐이다. struct field는 비공개로 바꾸지 않는다(encoding/json이 비공개 필드를 버린다). 다른 패키지의 테스트가 쓰는 이름, 인터페이스 구현 메서드, composition root가 조립하는 이름, `augmentation/signals.go`가 찾는 이름(예: `workercli.Command.RunCleanupStuck`), 패키지 밖에서 부르는 exported 함수·메서드가 돌려주는 타입(예: `hostprobe.NewCodexRunner`가 돌려주는 `CodexRunner`, `issueopslease`의 `ClaimResult`. 측정 설정의 revive `unexported-return`이 이것을 지적한다)은 그대로 둔다.
- `internal/architecture/testdata/ddd_contract_function_roles.json`은 여러 패키지가 함께 쓰는 파일이므로 작업자가 고치지 않고, 메인 세션이 작업 결과를 모은 뒤 한 번에 고친다.
- 주석은 식별자 이름으로 시작하는 완전한 문장이고, 코드만 봐서는 알기 어려운 책임·에러·불변식을 적는다. 언어는 패키지 안의 기존 주석 언어를 따른다. 패키지 주석은 패키지마다 한 곳에만 둔다.

재사용하는 기존 구현:

- 종료 코드 매핑 `rootSubcommandErrorExitCode`(`root_command_facade.go:88-113`)와 `gatescli.UsageError` 같은 에러 타입 패턴.
- `processlease.Attach(ctx, cmd)`와 `boundedDiagnostic`(`providerutil/bounded_command.go:75-93`)의 출력 상한·진단 문구.
- `doctor/observations.go`의 `sync.WaitGroup.Go` 팬아웃.

## 병목과 장애

- 런타임 hot path는 바뀌지 않는다. 이름 변경과 주석은 실행 코드를 바꾸지 않는다. context 전달은 취소 경로만 더한다.
- 이름 변경 도구 실패: 도구는 고치기 전에 새 이름이 같은 패키지의 기존 이름이나 같은 scope의 식별자와 겹치는지 검사하고, 겹치면 아무것도 쓰지 않고 멈춘다. 패키지 하나를 바꿀 때마다 세 GOOS에서 `go build ./...`와 `go vet ./...`가 통과해야 다음 패키지로 넘어간다.
- 같은 파일 동시 수정: 문서 주석 작업은 패키지 단위로 나누고 한 패키지는 한 작업자만 맡는다. 이름 변경은 한 프로세스가 순서대로 한다.
- 문자열 의존: 이름을 문자열로 찾는 코드는 두 종류다. `internal/architecture`의 AST 검사는 실제 저장소를 읽으므로 `go test ./internal/architecture`가 변화를 잡는다. `augmentation/signals.go`의 테스트는 모두 `t.TempDir()` 픽스처를 쓰므로(`signals_test.go:10`) 실제 저장소 변화를 잡지 못한다. 그래서 저장소 루트로 `CollectSignals`를 부르고 base에서 참인 신호가 그대로 참인지 확인하는 테스트를 추가한다.
- 플랫폼 분기: `*_windows.go`, `*_linux.go` 파일은 macOS 빌드에서 type check되지 않는다. 이름 변경 뒤 `GOOS=linux go build ./...`와 `GOOS=windows go build ./...`를 실행하고, `./scripts/lint-local.sh`가 GOOS=linux를 함께 검사한다.
- 되돌리기: 묶음마다 커밋이 나뉘므로 `git revert`로 묶음 하나만 되돌릴 수 있다. 2번은 1번의 이름 위에 있으므로 1번을 되돌리려면 2·3번을 먼저 되돌린다.

## 절충점과 기각한 대안

| 대안 | 판단 | 이유 |
|---|---|---|
| `IssueOps*` 중 자주 쓰는 타입만 이름 변경 | 기각 | 패키지 안에 접두어 있는 이름과 없는 이름이 섞이면 규칙이 두 가지가 된다. 사용자가 이름 변경 전체를 범위로 정했다 |
| 옛 이름을 type alias로 남겨 단계적 이전 | 기각 | 2026-10-06 ADR이 re-export shim을 금지한다. 한 번에 모든 참조를 바꾼다 |
| 정규식 일괄 치환 | 기각 | 다른 패키지에 같은 이름이 있으면(`contract/issueopsbenchmark`의 `IssueOps*` 이름 등) 잘못 바꾸고, tag 없는 명시적 struct field를 바꾸면 JSON 키가 바뀐다. 타입 정보로 객체를 구분해야 한다 |
| `gopls rename` | 기각 | 현재 GOOS 빌드 설정의 파일만 보므로 build tag가 붙은 파일의 참조를 놓친다 |
| revive가 보고한 stutter 29건만 처리 | 기각 | 주석이 없는 타입의 stutter는 보고되지 않아, 문서 주석 묶음 뒤 재측정에서 85건이 새로 나온다. 완료 기준 1을 지킬 수 없다 |
| `domain/issueopsremote` 디렉터리를 `domain/remote`로 이동 | 기각 | `application/issueopsremote`가 같은 capability 이름을 쓰고, `internal/architecture/testdata`의 책임 목록이 디렉터리 경로로 이 패키지를 가리킨다. clause를 디렉터리에 맞추고 `IssueOpsRemote` 접두어를 떼는 쪽이 바꾸는 곳이 적다 |
| `PRGitObservations`(11개 메서드) 분리 | 기각 | 유일한 소비자 `internal/application/issueopscycle/readiness_service.go`가 11개 메서드를 모두 쓰고 구현도 하나다. 나눠도 의존이 줄지 않고 인터페이스 이름만 늘어난다 |
| `port.InstallPlan` 축소 | 기각 | #499가 3중 정의를 이 port 하나로 통합했고 `internal/architecture/install_plan_contract_test.go`가 이 구조를 고정한다 |
| 모든 `exec.Command`를 `CommandContext`로 변경 | 기각 | ctx가 없는 로컬 git 실행까지 바꾸려면 호출 경로 전체의 시그니처가 바뀐다. 취소가 의미 있는 네트워크 호출만 바꾼다 |
| provider 자식 경로를 `providerutil` bounded command로 변경 | 기각 | 에러 문구가 `gh api %s failed: <stderr>`에서 `command failed after start: …`로 바뀌어 cleanup 결과의 `Error` 필드가 달라지고, 15초/60초 timeout과 256 KiB 상한(`bounded_command.go:19-22`)이 새로 걸린다. ctx 전달만 더한다 |
| stdio MCP EOF 대기에 `Close()`가 닫는 채널만 추가 | 기각 | production에서 대기 중에 `Close()`를 부르는 쪽이 없다. 취소 신호원(`signal.NotifyContext`)과 함께 ctx를 봐야 실제로 동작한다 |
| stdio 신호 ctx를 `server.Run`에만 넘김 | 기각 | `IOTransport`는 요청 ctx에 취소를 전하지 않고 `Run`은 처리 중인 요청이 끝날 때까지 기다린다(go-sdk `server.go:1367-1372`, jsonrpc2 `conn.go:497-509`). stdin이 열린 채 `channel_recv`(wait 300초)가 처리 중이면 첫 신호 뒤에도 최대 300초 남는다 |
| revive·stylecheck를 상시 lint 게이트로 켜기 | 보류 | `.issueops/testing/unit-and-contract.md:41-42`가 "문체·미세 성능 linter(revive, gocritic, perfsprint, predeclared, stylecheck(ST), quickfix(QF))는 버그를 잡지 않고 코드만 흔들어서 켜지 않는다"고 정한다. 이 이슈는 임시 설정으로 측정만 하고, 게이트 전환은 ADR이 필요한 별도 결정으로 남긴다 |
| 동사 없는 `fmt.Errorf` 1,335건을 `errors.New`로 변경 | 기각 | 같은 문서가 perfsprint를 켜지 않는다고 정했다. 동작 차이가 없다 |
| 하위 이슈 세 개로 분할 | 기각 | 사용자가 단일 이슈를 선택했다. 커밋을 묶음별로 나눠 리뷰·되돌리기 경계를 유지한다 |
