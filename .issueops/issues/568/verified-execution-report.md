# #568 verified-execution report

- lifecycle: `io-7426b49dc042`, branch `568-opencode-first-party-host`, base `e65a62185ba55e54c8e59c8fd0bab7b84d84d358`
- 계획: `.issueops/issues/568/plan.md`(봉인 원본 sha256 `728f12f3`로 시작)
- 게이트 원장: `.issueops/issues/568/gates.md`

## 목표와 수용 기준

opencode v2(2.0.32)를 Codex, Claude Code, Omo native, omp 다음의 다섯 번째 first-party host로 붙인다. IssueOps lease는 `opencode --standalone` 세션만 가질 수 있다.
게이트 결과는 원장의 EVIDENCE가 정본이다.

| 기준 | 게이트 | 결과 |
|---|---|---|
| opencode actor는 `serve --stdio` receipt로만 허용되고 공유 서비스·host 불일치는 core verifier에서 거부된다 | G1 | `ALL_PASS` |
| 생성 plugin이 세션당 한 번 catalog를 읽어 매 호출 system에 넣고, argv가 설치된 플래그만 쓴다 | G2 | `ALL_PASS` |
| opencode 설치 adapter·활성화·inspect·native 검증 테스트가 통과한다 | G3 | `ALL_PASS` |
| 임시 HOME install dry-run이 opencode 항목을 계획하고 아무것도 쓰지 않는다 | G4 | `DRY_RUN_OK` |
| opencode role model이 설정·builtin을 따르고 variant를 바꾸지 않는다 | G5 | `ALL_PASS` |
| cmux launcher와 인계 스킬이 opencode를 standalone·auto로 연다 | G6 | `ALL_PASS` |
| trace `opencode-json`이 `step_finish` 토큰을 합산한다 | G7 | `ALL_PASS` |
| hostprobe opencode runner가 격리 env·warm-up serve·guard로 episode를 구성한다 | G8 | `ALL_PASS` |
| build·vet·gofmt·전체 race 테스트가 통과하고 MCP enum에 opencode가 있다 | G9 | `ALL_PASS` |
| 실제 opencode 2.0.32 standalone 세션이 catalog·MCP 도구·standalone receipt를 받는다 | G10 | `OPENCODE_LIVE_PASS` |
| 격리 clone·임시 HOME·stdio 설치에서 self-verify가 종료 조건을 만족한다 | G11 | abandoned: CHECK가 정책 상한 15분 안에 끝나지 않는다. 같은 단계를 직접 실행한 결과는 `ok=true`, `termination_eligible=true`, `minimum_goal_score=100`(868초) |

## 구현 요약

- receipt 규칙: `NormalizeNativeActor`가 선언 host `opencode`와 opencode 실행 파일 receipt를 서로 묶는다. `verifyNative`는 선택 interface `ProcessArgumentsInspector`로 receipt 프로세스의 실제 인자를 읽어 `opencode serve --stdio`만 받는다. 인자는 record에 남기지 않는다.
- 설치: 새 package `internal/adapter/opencode`가 `opencode.json`의 `mcp.servers.issueops`(0600), `plugins/issueops.js`, 스킬 링크를 쓴다. `opencode.jsonc`에 같은 server가 있으면 설치를 멈춘다. `installutil`의 JSON 병합 helper는 key 경로를 받도록 넓히고 기존 호출자를 옮겼다.
- plugin: `hostprotocol.OpencodeLifecyclePlugin`은 `context` hook 하나만 등록한다. 세션당 한 번 `hook session-start --json`(자식은 `subagent-start --json`)을 부르고, 결과를 최대 512개까지 cache한다.
- argv·모델·trace·probe: interactive `opencode --standalone --auto --prompt`, print `opencode run --standalone --model P/M#E`, role model `Config.Opencode`(effort는 모델 variant, docs-only 하향과 상향 없음), trace `opencode-json`, hostprobe `OpencodeRunner`(private `opencode serve`)를 더했다.
- 계약: MCP host enum과 `channel_send` 설명이 바뀌어 catalog digest가 `be8fdd69…`로 바뀌었다. usage·mcp_tools·response_contracts·install matrix golden, DDD inventory, `configs/{omo,omp,agy,opencode}` 템플릿을 재생성했다.
- 문서: 새 ADR `2026-10-18-opencode-v2-as-the-fifth-first-party-host.md`, 새 가이드 `operations/guides/opencode-host.md`, host 목록을 적은 운영 문서·README·AGENTS.md·스킬을 다섯 host로 고쳤다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/CONSTITUTION.md` | "핵심 정책은 host adapter가 아니라 Go core에 둔다." | receipt 규칙을 `internal/domain/issueops`와 `internal/application/authority`에 두고 plugin은 catalog 전달만 한다. |
| `.issueops/CONSTITUTION.md` | "에이전트, hook, relay, retry, monitor, orchestration의 모든 반복은 유한한 상한과 명시적인 성공·실패·취소·timeout·no-op 종료 경로를 가져야 한다." | plugin hook 호출에 10초 timeout, 세션 cache에 512개 상한을 두었다. probe server 종료는 SIGTERM 뒤 5초 SIGKILL이다. |
| `.issueops/ARCHITECTURE.md` | "새 host 추가 시 공통 domain/application 정책을 복제하지 않고 `port.HostInstaller` 구현체와 composition-root wiring만 추가한다." | 설치는 `internal/adapter/opencode`의 `Installer`와 `issueopsapp` wiring으로만 붙였다. |
| `.issueops/architecture/host-integration.md` | "host adapter 변경: core contract를 복제하지 않았는지 확인하고 `internal/adapter` contract matrix golden으로 Codex/Claude/Omo/omp 설치 표면이 drift되지 않았는지 검증한다." | matrix golden을 재생성하고 다른 host의 경로·kind 변화가 없음을 확인했다(`task-8-golden-diff.txt`). |
| `.issueops/conventions/state-policy-and-hooks.md` | "`issueops hook`의 subcommand는 `session-start`, `subagent-start`, `post-compact` 셋이다." | 새 subcommand 없이 plugin이 기존 두 subcommand를 부른다. |
| `.issueops/CAUTIONS.md` | "Omo·omp MCP catalog는 server config hash로 장기 cache되므로 installer가 advertised catalog SHA를 config env에 포함해 binary-only schema update도 fresh session에서 재조회되게 한다" | opencode 항목에도 같은 digest를 헤더·`environment`로 넣었다. |
| `.issueops/cautions/integrations.md` | "**외부 CLI를 통한 관찰은 쓰기다.** dry-run·preview·readiness 경로는 외부 프로세스를 spawn하지 않는다(`exec.LookPath` 수준의 존재 확인만 허용)." | dry-run·native 검증·inspect는 `opencode`를 실행하지 않는다. |
| `.issueops/cautions/2026-08-28-verify-host-cli-flags-against-the-installed-version.md` | "Do not assume automation flags from another CLI version; inspect the installed host help and lock exact argv with a focused adapter test." | 2.0.32 `--help`로 확인한 플래그만 쓰고 `argv_test.go`·`role_agents_test.go`로 고정했다. |
| `.issueops/adr/2026-10-08-omp-first-party-host.md` | "omp is the fourth first-party host, ordered after Omo everywhere (`codex, claude, omo, omp`; installers end with `agy`)." | 순서를 `codex, claude, omo, omp, opencode, agy`로 했다. |
| `.issueops/adr/2026-10-08-omp-first-party-host.md` | "A session id alone never authorizes a lease; Omo and omp ids together are ambiguous and fail closed." | `OPENCODE_SESSION_ID`를 후보에 더하고 다른 host ID와 함께 보이면 거부한다. |
| `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md` | "세 host의 user 설정에는 issueops entry 하나만 `url`+`Authorization` 헤더로 바꿔 넣고 0600으로 쓴다." | `opencode.json`을 0600으로 쓴다. |
| `.issueops/CONVENTIONS.md` | "생성물은 직접 수정하지 않고 생성 스크립트와 source를 고친 뒤 재생성한다." | golden·inventory·템플릿은 `-update*` 플래그와 installer 테스트로 재생성했다. |
| `.issueops/cautions/audit-and-process.md` | "새 CLI/MCP/native skill 기능은 `issueops self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다." | native integration validator에 opencode 경로·MCP·plugin 바이트 검사를 더했다. |
| `.issueops/TECH_STACK.md` | "`github.com/dop251/goja` … (Omo extension JS를 실행하는 테스트 전용)." | 새 dependency 없이 goja로 plugin을 테스트하고 용도 설명에 opencode plugin을 더했다. |
| `.issueops/documentation/README.md` | "Detailed module in a manifest-listed family: at most 250 lines." | `hosts.md`가 250줄을 넘게 되어 opencode 절을 `operations/guides/opencode-host.md`로 분리하고 `OPERATIONS.md` 표에 링크했다. |
| `.issueops/conventions/go-and-packages.md` | "책임이 섞인 코드에서 변경 이유가 서로 다른 부분을 작게 분리한다." | ai-slop-clean: `normalizeJSONC`의 주석 건너뛰기를 `commentEnd`로, `OpencodeRunner.Run`의 입력 검사와 server 기동을 `episodeInputs`·`startEpisodeServer`로 분리했다. |
| `.issueops/conventions/go-and-packages.md` | "SOLID를 이유로 core 정책을 host adapter에 복제하거나, host별 구현을 중복한다." | ai-slop-clean: `hostActivationReadback.Verify`의 host별 반복 블록 여섯 개를 verifier 목록 순회 하나로 합쳤다. |
| `.issueops/conventions/go-and-packages.md` | "새 abstraction은 최소 두 사용처, 명확한 테스트 경계, 또는 외부 기술 경계 중 하나가 있을 때만 만든다." | ai-slop-clean: 일곱 곳의 host 허용 목록을 공용 함수로 묶는 정리는 이번 변경 범위 밖이라 하지 않고 범위 밖 발견으로만 적었다. |

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/CONSTITUTION.md` | "핵심 정책은 host adapter가 아니라 Go core에 둔다." | 지킴 | receipt 판정은 `internal/domain/issueops/native_actor.go`(`OpencodeStandaloneServer`)와 `internal/application/authority/service.go`(`requireOpencodeStandaloneServer`)에 있고, `configs/opencode/issueops.js`는 hook 호출과 catalog 전달만 한다. |
| `.issueops/CONSTITUTION.md` | "shell/process 실행은 명시적 cwd, timeout, env 정책, audit log 없이 추가하지 않는다." | 지킴 | 새 실행은 모두 관측·probe용이다. `nativeProcessArguments`는 기존 `processField`와 같은 `nativeProcessProbeTimeout`과 `LC_ALL=C`를 쓰고, plugin의 `Bun.spawn`은 `cwd: repo`와 10초 timeout, probe server는 episode `work` cwd·격리 env·종료 상한을 둔다. 사용자 명령을 실행하는 경로는 더하지 않았다. |
| `.issueops/CONSTITUTION.md` | "secret 원문은 로그, 문서, 테스트 assertion, MCP/CLI 응답에 남기지 않는다." | 지킴 | bearer는 `opencode.json`에만 0600으로 쓰고(`internal/adapter/opencode/mcp.go`), probe server 비밀번호는 episode env에만 둔다. 테스트는 가짜 bearer 값(`secret-bearer`)만 쓰고 파일 mode 0600을 단언한다. |
| `.issueops/CONSTITUTION.md` | "에이전트, hook, relay, retry, monitor, orchestration의 모든 반복은 유한한 상한과 명시적인 성공·실패·취소·timeout·no-op 종료 경로를 가져야 한다." | 지킴 | plugin cache 512개 상한과 hook 10초 timeout, probe `waitReady`·`warmProbeMCP`의 `readyLimit`, server 종료의 SIGTERM→5초→SIGKILL. |
| `.issueops/ARCHITECTURE.md` | "새 host 추가 시 공통 domain/application 정책을 복제하지 않고 `port.HostInstaller` 구현체와 composition-root wiring만 추가한다." | 지킴 | `internal/adapter/opencode`와 `cmd/issueops/issueopsapp/host_installers.go`·`install_wiring.go`. domain·application 변경은 host 허용 목록과 opencode receipt 규칙뿐이다. |
| `.issueops/ARCHITECTURE.md` | "`internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete adapter나 `cmd/...`를 import하지 않는다." | 지킴 | `native_actor.go`는 contract만 import한다. `go test ./internal/architecture` 통과. |
| `.issueops/CONVENTIONS.md` | "MCP tool 이름은 snake_case `<capability>_<verb>`(예: `harness_inspect`, `issueops_execution`). CLI와 MCP는 같은 core request/response DTO를 공유한다." | 지킴 | 새 tool 없이 `issueops_execution.host` enum에 값만 더했고, CLI usage와 MCP schema가 같은 host 목록을 쓴다(usage·mcp_tools golden). |
| `.issueops/CONVENTIONS.md` | "hook은 `SessionStart`와 `SubagentStart` 둘을 context-only로 등록하고(`post-compact`는 Omo·omp·진단용, opencode plugin은 `context` hook에서 `session-start`·`subagent-start`를 부른다), 작업을 대신 실행하거나 shared docs를 직접 수정하지 않는다." | 지킴 | plugin은 catalog를 system prompt에 더하기만 하고 `process.env`·파일을 쓰지 않는다(`TestGeneratedOpencodeLifecyclePluginProofRejectsRuntimeMutations`). 이 문장의 괄호 부분은 이번 변경이 더한 것이며 근거는 계획 `## 적용되는 결정과 주의사항`의 state-policy-and-hooks 항목이다. |
| `.issueops/CAUTIONS.md` | "core behavior는 Go core에 두고 host adapter는 CLI/MCP wrapper로 제한한다." | 지킴 | 위 CONSTITUTION 첫 행과 같은 근거. |
| `.issueops/CAUTIONS.md` | "Omo·omp MCP catalog는 server config hash로 장기 cache되고 opencode는 server config가 바뀔 때 재연결하므로 installer가 advertised catalog SHA를 config env에 포함해 binary-only schema update도 fresh session에서 재조회되게 한다" | 지킴 | `internal/adapter/opencode/mcp.go`의 `mcpCatalogHeader`와 `environment.ISSUEOPS_MCP_CATALOG_SHA256`. 문장의 opencode 부분은 계획 `## 적용되는 결정과 주의사항`의 CAUTIONS 항목이 정했다. |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | 지킴 | 문서·스킬 수정 뒤 `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update`로 재생성했다. |

## 성능 영향

- plugin `context` hook은 모델 호출마다 실행되지만 `issueops hook` 프로세스는 세션당 한 번만 띄운다. 이후 호출은 `Map` 조회와 배열 추가 한 번이다. goja 테스트(`TestGeneratedOpencodeLifecyclePluginExecutesMockContextHook`)가 세션당 spawn 1회를, live 테스트(G10)가 두 번째 모델 호출에 spawn이 없음을 고정한다.
- Go 쪽은 host 목록이 4개에서 5개로 늘어 설치·검증·inspect가 상수 배 늘어날 뿐이다. receipt 규칙은 ancestry를 한 번 순회하고, opencode receipt일 때만 `ps` 한 번을 더 부른다.

## ai-slop-clean

측정은 `.issueops/evidence/slop-clean-metrics.json`에 남겼다. 범위는 변경되거나 새로 생긴 Go·JS·shell 파일이다.

| 지표 | 정리 전 | 정리 후 |
|---|---|---|
| SNR(추가된 비어 있지 않은 줄 중 주석이 아닌 비율) | 0.950 (3404/3584) | 0.948 (3444/3633) |
| 순환 복잡도 12 초과인 새 함수·바뀐 함수 | 4개: `normalizeJSONC` 21, `OpencodeRunner.Run` 17, `ListLeaseHolderIndexes` 16(base 15), `hostActivationReadback.Verify` 13(base 12) | 1개: `ListLeaseHolderIndexes` 16(base 15) |
| 중복 블록(golangci-lint v2.12.2 `dupl`, 기본 150 token, `--new-from-rev=e65a621`) | 0 | 0 |
| golangci-lint(저장소 설정, go1.26.6 toolchain) | 0 issues | 0 issues |

- 바꾼 것: `internal/adapter/opencode/mcp.go`(`commentEnd` 추출), `internal/adapter/hostprobe/opencode.go`(`episodeInputs`, `startEpisodeServer` 추출), `cmd/issueops/issueopsapp/install_wiring.go`(readback 순회), DDD inventory 재생성. SNR은 새 helper의 doc comment 때문에 0.002 내려갔다.
- 동작 보존 확인: `normalizeJSONC`는 정리 전 구현과 무작위 입력 200,000개의 출력이 모두 같았다(일회성 테스트, 실행 뒤 삭제). runner는 `TestOpencodeRunnerWarmsMCPBeforeRun`의 준비 실패·MCP 연결 실패 행이 server를 한 번 멈추는지 단언하고 통과한다. readback 순회는 `cmd/issueops/issueopsapp`·`installcli` 테스트(여섯 host 설치와 readback 11건)가 통과한다. 정리 뒤 hostprobe·opencode는 `-race`, 저장소 golangci-lint, `go vet ./...`, architecture 테스트를 다시 통과했다.
- 일부러 둔 것: `ListLeaseHolderIndexes`는 base에서 이미 15였고, 이번 변경은 host 조건 하나를 더했을 뿐이다.
- 범위 밖 발견: host 허용 목록이 `execution_state.go`, `prepare.go`, `record_validation.go`, `classifier_identity.go`, `authority.go`, `handoff_delivery.go`, `native_actor.go`에 따로 있다. host를 더할 때마다 일곱 곳을 고쳐야 한다.
- 주장 점검: ADR·새 가이드·caution·이 보고서에서 근거 없는 "보장·완전·검증됨·always·guarantee" 표현을 찾지 못했다.

## 구현 리뷰

- 1라운드(`omp -p --model anthropic/claude-opus-5-5 --thinking high`, 빈 컨텍스트, tier `contract`, 렌즈 compat·reuse·perf·side-effect): `revise`. 필수 결함 1건: 주석이나 끝 쉼표가 있는 `opencode.json`은 opencode가 정상으로 읽지만, 설치기는 엄격한 JSON으로 읽다가 경로 없는 오류로 실패했다. 그 결과 다른 host까지 포함한 install·update 전체가 `ok=false`가 됐다. project `.opencode/opencode.json`의 HTTP 제거 경로도 같았다.
- 수정: `internal/adapter/opencode/mcp.go`의 `requirePlainJSON`이 병합 전에 원본과 `normalizeJSONC` 결과를 비교한다. 다르면 파일을 고치지 않고, 경로와 복구 방법(`opencode.jsonc`로 옮기거나 주석 삭제)을 담은 오류로 opencode 설치를 멈춘다. 병합 오류에도 경로를 붙였다. 주석을 지키려고 사용자 파일을 다시 쓰지 않는다는 기존 결정과 같은 방향이다. ADR·`operations/install.md`·`opencode-host.md`에 복구 절차를 적었고, 회귀 테스트 `TestInstallerRefusesCommentedConfigWithoutRewritingIt`는 user 파일과 project HTTP 제거 경로를 함께 고정한다. 리뷰어의 CHECK는 수정 뒤 `JSONC_OK`를 냈다.
- 비차단 메모 반영: `.gitignore`에 `.opencode/`를 더했고, cmux 인계에서 opencode의 `owner_claimed`가 기록되지 않는 이유를 ADR에 적었다. `CAUTIONS.md`의 Omo 캐시 사실("server config hash로 장기 cache")은 원래 문장으로 되돌렸다. G1 `need` 목록은 봉인된 원장이라 고치지 않았다. `TestExecutableMatchesHostOpencode`는 같은 `-run Opencode` 실행 안에서 종료 코드로 확인된다.
- 2라운드(같은 모델·effort, 빈 컨텍스트 delta 리뷰): `pass`. 결함 1은 해결됨으로 판정했고 새 필수 결함은 없었다. 리뷰어는 라운드 1 CHECK(`JSONC_OK`)와 `go test ./internal/adapter/opencode`를 다시 실행했고, (a)안이 intent 성공 기준 1("기존 opencode 설정은 보존된다")에 맞는다고 판단했다. 비차단 메모 셋: 감싼 오류에서 경로가 두 번 나올 수 있다. `--project-local` stdio 경로는 기존 방식대로 `.opencode/opencode.json`을 통째로 다시 쓴다(Claude `.mcp.json`, extensionhost와 같다). 봉인된 plan 본문에는 주석 있는 `opencode.json` 거부가 없다(ADR과 운영 문서에는 있다).

## side effect

- 파일: 위 구현 파일, golden·inventory·`configs/` 템플릿, ADR·caution 각 1건, 운영 문서·README·AGENTS.md·스킬, `.gitignore`(`.opencode/`), `.issueops/issues/568/` 추적 사본·원장·이 보고서.
- 사용자 설치 결과(다음 `io update`부터): `~/.config/opencode/opencode.json`의 `mcp.servers.issueops`(0600), `~/.config/opencode/plugins/issueops.js`, `~/.config/opencode/skills/<skill>` 링크. 다른 키·plugin·`opencode.jsonc`는 건드리지 않는다.
- 호환성: `opencode` host 값이 든 record와 `agent-models`의 `opencode` 절은 이전 바이너리가 거부한다(ADR에 적었다).
- 원격: 8~9단계의 브랜치 push와 draft PR 하나(승인 범위). 이슈 본문 계약 변경은 인계 전에 반영했다(`feedback` `issue_updated_at` 기록됨).
- durable state: 이 사이클 record와 원장.
- 검증 산출물은 워크트리 밖 `/tmp/io-568-*`(base clone, CI와 같은 Python venv, 격리 self-verify clone)와 ignore되는 `.issueops/evidence/`에 두었다. 사용자 `~/.local/share/opencode`는 probe 전후 mtime이 같다.
- source checkout(`$SOURCE_ROOT`)은 구현 중 한 번 잘못 쓰였다가 되돌렸고, `git status` 0줄을 다시 확인했다.

## 의도 대조

| intent 성공 기준 | 덮는 게이트·증거 |
|---|---|
| 임시 HOME 설치와 dry-run이 opencode MCP(`mcp.servers.issueops`, `codemode:false`)·plugin을 기록·검증하고 기존 opencode 설정은 보존된다 | G3(`TestInstallerPreservesUserConfigAndPlugins`, `TestInstallerRefusesCommentedConfigWithoutRewritingIt` 포함), G4(dry-run이 임시 HOME에 아무것도 쓰지 않음), G11 직접 실행과 검증 단계 self-verify의 `native integration` step |
| opencode 세션에서 issueops 스킬이 이름마다 한 번, MCP 도구가 `issueops_*` 이름으로 노출·호출된다 | G10(실제 opencode 2.0.32, `<name>atomic-commit-push</name>` 1회, `issueops_*` 도구 호출) |
| 새 세션과 압축 뒤 모델 요청에 catalog가 있다 | G10(모든 모델 요청에 catalog), G2(goja가 세션의 매 호출마다 catalog를 넣음). 압축 뒤 첫 호출에도 `context` hook이 돈다는 사실은 계획 조사 때 2.0.32 바이너리에서 확인했고 ADR에 적었다(이슈 계약 변경으로 반영됨) |
| `--standalone` 셸의 whoami가 host=opencode와 `OPENCODE_SESSION_ID`를 반환하고 claim이 성공하며 공유 서비스 셸의 claim은 구조화된 오류로 거부된다 | G10(whoami와, claim과 같은 verifier를 쓰는 `mcp authorize` 성공), G1(`serve --service`·TUI·`run` receipt 거부 단위 테스트), `task-1-receipt-error.txt` |
| model show가 다섯 host를 보이고 model set --host opencode가 prepare·리뷰 모델에 반영된다 | G5, 검증 단계 smoke `verify-model-smoke.txt`(hosts 5개, set 뒤 `model resolve`가 `opencode run --standalone --model mock/reviewer#xhigh`를 냄) |
| 실행 profile argv가 2.0.32 `--help`의 플래그만 쓴다 | G2, G6 |
| trace `--input-format opencode-json`이 `step_finish` 토큰을 합산한다 | G7 |
| golden과 omo·omp·agy 템플릿 digest가 재생성된다 | G9(전체 테스트가 golden·템플릿 일치를 확인), `task-8-golden-diff.txt` |
| ADR과 현행 문서·스킬이 다섯 host를 말한다 | 계획의 host 나열 `git grep` 검사 0줄(`task-10-docs.txt`), 새 ADR |
| gofmt·vet·test·race·build·self-verify가 통과한다 | G9, 아래 검증 표의 self-verify |

덮지 못한 성공 기준은 없다. 비목표는 intent에 없다.

## 검증

| 요구한 문서 경로 | 그 문서의 해당 문장 인용 | 명령 | 결과 |
|---|---|---|---|
| `.issueops/TESTING.md` | "단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." | 임시 clone에 워크트리 변경을 복사하고 임시 HOME·XDG·state, LANG, CI와 같은 Python venv, 사용자 Go cache로 `./scripts/install-native.sh --path-mode=skip --mcp-transport=stdio` 뒤 `./bin/issueops self-verify --base-ref e65a62185ba55e54c8e59c8fd0bab7b84d84d358 --seed=100 --target-score=95 --llm-eval=false --json` | 리뷰 수정 뒤 최종 tree에서 install exit 0, self-verify exit 0, `ok=true`, `termination_eligible=true`, `minimum_goal_score=100`, 27개 step 모두 통과(808초, `verify-isolated-self-verify.txt`). risk QA tier는 `go test -race ./... -count=1 && go vet ./...`를 실행해 통과했다(522초). 그 결과의 `scope.base_sha`·`scope.head_sha` 필드는 저장하지 않아 대조하지 못했다. 다만 tier가 전체 race와 vet를 실행했으므로 범위가 좁아지지는 않았다. |
| `.issueops/TESTING.md` | "Go 코드 변경의 기본 검증(`gofmt -l`, `go test -race ./...`, `go vet ./...`, architecture ratchet, operational-health 위임, golden 갱신 조건)은 [unit-and-contract.md](testing/unit-and-contract.md)가 소유하고" | 위 self-verify의 `gofmt`·`risk QA tier`·`contract golden tests` step과 G9 | 통과 |
| `.issueops/TESTING.md` | "다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 첫 게이트부터 전체 재실행한다." | 리뷰어와 동시에 돈 첫 검증 단계 self-verify는 race step이 10분 상한에 걸려 실패했다. 그 결과를 쓰지 않고 수정 뒤 최종 tree에서 전체를 다시 실행했다 | 위 첫 행 |
| 게이트 원장 | G1–G10 CHECK | `gates check --write --env HOME,PATH,LANG`(CI venv를 앞에 둔 `PATH`) | 10/11 met. G11은 CHECK가 정책 상한 15분을 넘어 abandoned로 남기고, 같은 단계를 직접 실행했다(`task-11-isolated-self-verify.txt`, 위 첫 행). 기본 env(`HOME,PATH`)에서 G3·G8·G9가 실패한 원인은 base `e65a621`에서도 재현되는 locale 의존 테스트였다. caution `2026-10-18-gate-checks-that-run-go-tests-or-self-verify-need-lang-and-t.md`로 남겼다. |
| `skills/issueops-verify/SKILL.md` | "`issueops execution sync-base --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json`으로 충돌 유무만 확인해 report에 적고 **apply하지 않는다.**" | `execution sync-base --preview` | `merge_needed:false` |
| 스킬 | 구현 리뷰 CHECK | 라운드 1 리뷰어 CHECK(주석 있는 `opencode.json` dry-run) | 수정 뒤 `JSONC_OK` |
| `.issueops/OPEN_API_SPEC.md` | 해당 없음 | endpoint·DTO·OpenAPI 변경 없음 | Not Run: 대상 아님 |

- 증거 파일: `.issueops/evidence/task-{1..10}-*.txt`, `task-7-probe-live.json`(무료 Zen 모델의 `provider.auth` 403으로 `harness_environment:provider_auth_failed`, 계획이 예상한 결과), `task-11-isolated-self-verify.txt`, `verify-isolated-self-verify.txt`, `verify-model-smoke.txt`, `slop-clean-metrics.json`.
- 문서 checker: 위반은 `.issueops/issues/561/plan.md`의 깨진 링크 1건뿐이고 main과 같은 파일이다. 계획의 host 나열 검사는 0줄이다.
- 병렬 실행: 검증 단계에서 self-verify와 delta 리뷰를 같은 fingerprint에 동시에 돌렸다(`SUB_AGENT_PATTERNS.md`의 `parallel_speed`). 두 작업은 각각 898초와 130초였고, 순서대로 돌렸을 때보다 약 130초를 줄였다. 첫 라운드 때는 리뷰어가 전체 테스트를 돌려 self-verify의 race step이 시간 상한에 걸렸으므로, 2라운드 프롬프트에서는 리뷰어의 전체 테스트 실행을 막았다.

## 발행

- draft PR: https://github.com/m16khb-org/issueops/pull/569 (`main` ← `568-opencode-first-party-host`, label `enhancement`, assignee `m16khb`, `Closes #568`)
- 게시 뒤 `remote verify-artifact`와 `gh pr view`로 draft 상태, head·base 브랜치, label, assignee, 연결 이슈를 다시 읽어 확인했다.
- 커밋은 구현(`e85c698b`), 문서(`2015bc54`), 사이클 자료(`21432b3a`) 셋과 보고서 갱신 커밋들이다. 구현 커밋만 있는 tree를 임시 clone에서 `go test ./...`로 돌렸을 때 실패는 `/tmp` 경로 symlink 때문에 base에서도 실패하는 `installcli` 테스트와, 부하에서 100ms timeout이 흔들리는 기존 `TestExecRunnerTimeoutKillsAndReapsUnixDescendant`(단독 5회 반복 통과)뿐이었다.
- 머지와 워크트리·브랜치 정리는 별도 승인 단계다.
- PR CI(verify)가 실패했다. `scripts/meeting_notes_skill_contract_test.py`의 `test_synthetic_fixture_family_has_no_identified_meeting_data`는 `git ls-files`로 추적 파일을 훑으며 금지된 식별 데이터의 지문을 찾는다. 이 테스트가 이 보고서의 side effect 절에 적은 source checkout 절대 경로 안의 계정 이름을 잡았다. 경로가 아니어도 계정 이름이 어디에 나오면 실패한다. 로컬 격리 self-verify는 커밋 전 변경을 clone에 복사만 했으므로 보고서가 추적 파일이 아니어서 이를 놓쳤다(caution `2026-10-08-tracked-intent-copies-quote-the-raw-request-and-local-self-v.md`와 같은 원인). 메인 세션이 사이클을 다시 열었고, 경로를 `$SOURCE_ROOT`로 바꿔 같은 PR에 반영한다.
