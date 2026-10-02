# 첫 구현 lane 독립 검증 (I1 / I2 / I5 / I6 / I7)

기준: 같은 폴더의 `contract.md`와 다섯 구현 보고서
(`implementation-{jsonl-completeness,reuse-statistics,sdk-compatibility,request-io,catalog-reads}.md`).
작업 트리 HEAD는 `92eaa001`이며 commit은 없다. 작업 트리에는 parent가 진행 중인
authority DTO/port, `contract/mcpservice`, `port/mcpservice`, sqlstore `record_guard`/span
observer(I8) 변경이 함께 있다. 이 다섯 판정에서는 그 변경을 제외했다.

검증 세션 환경: `PI_MODEL=claude-fable-5-1`, `PI_REASONING_LEVEL=high`, `go1.26.4 darwin/arm64`.
모든 Go 명령은 `PATH=/tmp/issueops-ten-improvements-01a0fa31/venv/bin:$PATH`로 실행했다.
production/test 파일, config, 사용자 HOME은 수정하지 않았다. 유일한 임시 쓰기는
아래 §7의 DDD inventory 재생성이며 원본을 byte 동일하게 복원했고 `git status`로 확인했다.

## 1. 판정 요약

| Lane | 판정 | 근거 요약 |
|---|---|---|
| I1 JSONL completeness | **PASS** | focused/package 테스트 exit 0, 실제 CLI 6 fixture 재현, 원문 미노출 확인 |
| I2 재사용 표시·표본 수 | **PASS (lane 범위)** | 다섯 패키지 exit 0, fixture 테스트 exit 0, duration 회귀 유지. 트리 수준에서 §6의 미해결 3건이 남는다 |
| I5 SDK v1.8.0 | **PASS** | `go mod verify`/`tidy -diff` 깨끗, 패키지·race 테스트 exit 0, go.sum checksum이 보고서와 일치 |
| I6 요청 내 Git 관측 | **PASS** | 테스트 exit 0, 실제 바이너리에서 git 호출 7회·`log` 1회 확인, memo 수명이 계약 경계 안 |
| I7 bounded 문서 catalog | **PASS** | 테스트·race exit 0, 실제 hook에서 141개 중 64개 선택·`over_cap=77`·symlink 제외 확인 |

트리 수준 실패 2건은 lane 코드 결함이 아니라 공유 golden 미갱신이며 Z-integration 소유다
(§6). 실패를 통과로 보지 않았고, lane 판정에도 포함하지 않았다.

## 2. 실행한 명령과 결과

보고서에 적힌 명령을 그대로 다시 실행했다. 로그는 `/tmp/iv-scratch/*.log`에 있다.

| 명령 | exit | 관측 |
|---|---:|---|
| `go test ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace -run '^(TestTraceAnalyzeJSONLCompleteness\|TestAnalyzePreservesIncompleteWarningsWithOrWithoutFindings\|TestAnalyzeReportsCompletenessIndependentlyOfExecution\|TestAnalyzeReportsReadFailureWithoutDecodingPartialBody)$' -count=1 -v` | 0 | 세 패키지 PASS |
| `go test ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace -count=1` | 0 | ok 0.192s / 0.097s / 0.101s |
| `go test ./internal/adapter/verification/probe/contractauditworker ./internal/application/selfverify ./internal/domain/selfaugment ./internal/domain/selfverify ./cmd/issueops/selfworkflow/steps -count=1` | 0 | 다섯 패키지 ok (selfverify 6.472s) |
| `go test ./cmd/issueops/selfworkflow/steps -run '^TestSelfVerifyReuseFixtureEntryPoint$' -count=1 -v` | 0 | PASS |
| I2 보고서의 8개 테스트 focused `-run` | 0 | 네 패키지 PASS |
| `go test ./internal/adapter/verification/probe/contractauditworker -run TestSuccessfulProbesPreserveCommandDurations -count=1 -v` | 0 | 137/149/163ms, worker 60ms 회귀 유지 |
| `go test ./cmd/issueops/selfworkflow/summary -run '^TestSelfVerificationContractIncludesSummaryExtensions$' -count=1 -v` | **1** | B가 보고한 범위 밖 v6 assertion 실패를 재현 (§6-1) |
| `go mod verify` | 0 | `all modules verified` |
| `go mod tidy -diff` | 0 | 출력 없음 |
| `go test ./cmd/issueops/mcpcli -count=1` | 0 | ok 5.072s |
| `go test -race -shuffle=on ./cmd/issueops/mcpcli -run TestMCPRevision -count=1 -v` | 0 | 4개 그룹 PASS 2.366s |
| `go test ./cmd/issueops/issueopsapp -run 'TestNextLocalReadinessObservation\|TestNextGitObservation' -count=1 -v` | 0 | 7개 PASS |
| `go test ./internal/adapter/preflight -count=1 -v` | 0 | PASS 1.113s |
| `go test ./internal/adapter/issueops -count=1` | 0 | ok 167.227s |
| `go test ./internal/adapter/projectdoc ./internal/domain/projectdoc ./internal/application/hookprompt ./internal/adapter/hookprompt ./cmd/issueops/hookcli -count=1` | 0 | 다섯 패키지 ok |
| `go test ./internal/adapter/projectdoc -run 'TestDiscoverReportsUnreadableDirectory\|TestDiscoverReadsBoundedHeader\|TestDiscoverSelectionIndependentOfCreationOrder\|TestScanProjectDocNamesMarksReadErrorAsTruncated' -count=1 -v` | 0 | parent가 추가한 unreadable-directory 테스트 포함 PASS |
| `go test -race ./internal/adapter/projectdoc ./internal/application/hookprompt ./internal/adapter/hookprompt -count=1` | 0 | ok |
| `go test ./internal/architecture -count=1` | **1** | `TestDDDResponsibilityInventoryMatchesSource` (§6-3) |
| `go test ./cmd/issueops/contractgolden -run Golden -count=1` | 0 | ok |
| `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` | **1** | `$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count unexpected` (§6-2) |
| `go build -o /tmp/iv-scratch/issueops ./cmd/issueops` | 0 | 이후 실제 표면 검증에 사용 |
| `go vet` (다섯 lane의 전 패키지 20개) | 0 | 출력 없음 |
| `gofmt -l $(git ls-files -m -o --exclude-standard '*.go')` | 0 | 출력 없음 |
| `git diff --check` | 0 | 출력 없음 |

## 3. I1 — JSONL completeness: PASS

읽은 파일: `internal/adapter/trace/{decode.go,analyze_test.go}`,
`internal/domain/trace/{observation.go,analysis.go,analysis_test.go}`,
`internal/application/trace/{service.go,service_test.go}`, `internal/contract/trace/types.go`.

- `decode.go:21-25` 단일 JSON 손상은 `Incomplete=true`와
  `invalid_json:invalid_jsonl_line`을, `:41-48`은 손상 행과 `scanner.Err()`를
  각각 `invalid_jsonl_line`/`jsonl_scan_error`로 기록한다. 원문 오류 문자열은
  어디에도 담지 않는다. 기존 Scanner cap은 그대로다.
- `analysis.go:12`는 입력 경고를 복사해 finding이 있어도 지우지 않고, `:29`가
  `Incomplete`를 전달한다. `service.go:33-36`은 Load 실패를 `ok=false`,
  `trace_read_error`, wrapped error로 돌려주고 decode를 호출하지 않는다. `:66`은
  `complete=!analysis.Incomplete`다. `types.go:15`의 `complete`는 omitempty가 없어
  항상 직렬화된다.
- 테스트 약화 여부: `analysis_test.go`의 기존 `JSONError` 검사는 새 입력 모델의
  동등한 검사(`Incomplete`+두 경고)로 바뀌었고 기대값이 느슨해지지 않았다.
  12개 fixture 테스트는 경고 수를 정확히 비교하고 `private-input-marker` 유출을 검사한다.
  sleep·시간 단언은 없다.

실제 CLI(`/tmp/iv-scratch/issueops trace analyze --input … --json`, 임시 state):

| fixture | ok | complete | finding_count | warnings | 유출 |
|---|---|---|---:|---|---|
| 두 sentinel JSONL | true | true | 2 | 없음 | 없음 |
| sentinel 사이 `private-marker` 행 | true | false | 2 | `invalid_jsonl_line` | 없음 |
| sentinel 사이 70,000자 padding 행 | true | false | 1 | `jsonl_scan_error` | 없음 |
| `{"private-marker":` 단일 JSON | true | false | 0 | `invalid_json:invalid_jsonl_line`, `no_supported_trace_findings` | 없음 |
| 존재하지 않는 파일 | false | false | 0 | `trace_read_error` | 없음 |

보고서 표와 일치한다. 큰 행 뒤 `last sentinel`은 유실되며 `complete=false`로 명시된다.

보고서의 남은 확인 4번(`input.go` 두 source fallback)은 사실이다.
`internal/adapter/trace/input.go:17-25`는 파일 읽기 실패 뒤 state key 읽기를
시도하고 그 오류 메시지를 wrap한다. I1 범위 밖이며 동작 결함은 아니다.

## 4. I2 — 재사용 표시와 duration 표본 수: PASS (lane 범위)

읽은 파일: 보고서가 나열한 14개와 parent가 인계한 probe 4개 파일, 새 테스트 2개.

- `internal/domain/selfaugment/summary.go:47-59`는 reused step을 pass/fail·label에
  포함한 뒤 `SlowestSteps`와 `durationsByLabel` 추가를 건너뛴다. `:74-76`이
  `ReusedCount`를 채운다. `step_stats.go:36-42`는 표본이 비어 있는 label에 count=0
  통계를 만든다. reused-only label이 `count=0, reused_count>0`이 되는 경로가 코드로 확인된다.
- `internal/application/selfverify/steps.go:84,147`의 두 재사용 경로만 `Reused: true`를
  설정한다. `self_verify_summary.go:16`이 Reused를 summary로 전달한다.
- `internal/domain/selfverify/contract.go:13` v7. `contract_test.go`는 v6와 hash가
  다름을 검사한다. `internal/application/selfaugment/planner.go:50-54`는 baseline의
  name/version/hash를 현재 contract와 대조하므로 v6 증거를 v7 통과로 재사용하지 않는다.
- parent가 인계한 duration 수정 4개 파일은 `step.DurationMS` 또는 네 command의
  DurationMS 합을 전달하며 `time.Since(time.Now())`는 사라졌다. 새 clock·sleep은 없다.
- 테스트 약화 여부: `python_history_test.go:38`의 `Version != 6`을 7로 바꾼 것은
  계약에 따른 기계 소비 값 갱신이다. `steps_test.go`는 실패 child의 duration/오류
  보존을 추가했고, 기존 단언을 제거한 곳은 없다.
- `TestSelfVerifyReuseFixtureEntryPoint`는 실제 `ExecuteLoop`→`SummarizeSelfVerification`
  진입점을 사용하고 covered command 재실행 시 `t.Fatalf`로 실패하도록 되어 있다.

## 5. I5 — SDK v1.8.0과 stdio 리비전 호환성: PASS

- `go.mod:34` `github.com/modelcontextprotocol/go-sdk v1.8.0`, `:26` `golang.org/x/time v0.15.0 // indirect`.
  `go.sum`의 두 checksum `h1:KIvahhYqwtbeniWVPs3TcXEA7b8jEtwfBpOTAI+Urx4=`,
  `h1:dL7u98E/zjJTGzEq+j30jQ8K2k1mb6LeAH4inEcSGts=`는 보고서 값과 같다.
  `cmd/issueops/mcpcli/mcp_sdk_server.go`는 `git status`에 없다.
- `mcp_revision_compatibility_test.go`: 5초 context와 `read()`의 `ctx.Err()` 검사로
  timeout을 성공으로 오인하지 않는다(`:84-87`, `:225-231`). cancellation 테스트는
  `entered`/`cancelled` channel을 요청 전에 만들고 정확한 requestId를 취소한다.
  sleep·polling은 없다. 2026-07-28 initialize가 2025-11-25로 fallback한다는 주장은
  `TestMCPRevisionStdioLegacyNegotiation`이 단언하며 이번 실행에서 통과했다.
- 보고서의 실제 바이너리 stdio 실행(tools 51개, docs 557개)은 재현하지 않았다.
  같은 matrix를 production `ServeMCPStreamContextWithDependencies`에 대해 패키지
  테스트로 재현한 것으로 충분하다고 판단했다. HTTP는 H 소유로 이 lane이 주장하지 않는다.

## 6. I6 — 요청 내 Git 관측: PASS

읽은 파일: `internal/adapter/preflight/{preflight.go,helpers.go,preflight_history_test.go}`,
`cmd/issueops/issueopsapp/{issueops_next_wiring.go,issueops_next_git_observation_test.go}`,
`internal/adapter/issueops/readiness_git_observation_test.go`,
`internal/application/issueopsnext/service.go`.

- `preflight.go:16-18`의 `Run` seam은 계약 시그니처 `func(string,...string)(int,string,string)`이다.
  `:55` 한 번의 `log -10 --format=%h%x00%s%x00%B%x00`에서 `helpers.go:22-66`의
  `parseHistory`가 NUL 3필드를 읽고 `last/commits(limit)/bodies`로 projection한다.
  잘린 마지막 tuple은 버린다(`:34`). 빈 history의 `bodies()`가 `[""]`를 돌려주어
  기존 `strings.Split("", "\x1e")` 의미를 보존한다. `out` helper는 기존 `GitOut`
  (`git.go:32-38`)과 같이 비정상 종료에 빈 문자열을 돌려준다.
- `issueops_next_wiring.go:39`는 `Next` 호출마다 `nextGitObservation`을 만든다.
  memo는 `branches[root]` 하나이고 `currentBranch`만 재사용한다(`:162-169`).
  toplevel/HEAD는 매번 새로 읽는다(`:175,179`). 전역 상태는 없다.
- memo 수명 검토: `service.go:202` `CurrentBranch(cwd)` → `:172` `WorktreeState(record root)`
  → `:335` `WorktreeState(cwd)` 사이에 fetch나 외부 작업이 없다. `LocalReadiness`는
  주석대로 fetch 없이 읽으며 `WorktreeState` 뒤에 실행된다. 계약의 "관측 단계 안에서만
  공유" 경계 안이다. 보고서가 적은 대로 같은 호출 안의 외부 checkout은 이전 branch 값을
  재사용하며, 계약은 atomic snapshot을 요구하지 않는다.
- `readiness_git.go`는 수정되지 않았고 새 테스트는 ref별·HEAD probe가 매번 실행됨을
  고정하는 특성화 테스트다. RED 단계에서도 통과했다는 보고는 맞으며, 보고서도
  두 RED가 컴파일 실패였다고 밝힌다. 동작 수준 RED는 없다.

실제 바이너리: 12 commit 임시 repo(짝수 commit에 Lore body)에서 PATH shim으로 git
호출을 기록하며 `issueops preflight --json`을 실행했다(exit 0).

```text
git 호출 7회, log 1회:
rev-parse --show-toplevel / branch --show-current / rev-parse --short HEAD /
rev-parse --abbrev-ref --symbolic-full-name @{u} / status --porcelain=v1 --branch /
log -10 --format=%h%x00%s%x00%B%x00 / remote -v
ok=true last="d584ef4 fix: change 12" recent=5 recent_count=10 conventional_subjects=10 lore_bodies=5
```

보고서의 "history 4→1, 전체 10→7" 중 변경 후 값 7과 1을 확인했다. 변경 전 10은
소스 기준 계산이며 이번에 측정하지 않았다.

## 7. I7 — bounded 문서 catalog: PASS

읽은 파일: `internal/adapter/projectdoc/{catalog.go,catalog_test.go}`,
`internal/domain/projectdoc/catalog_types.go`, `internal/contract/hookprompt/types.go`,
`internal/application/hookprompt/{catalog.go,catalog_test.go}`,
`internal/adapter/hookprompt/{catalog.go,catalog_test.go}`, `cmd/issueops/issueopsapp/hook_facade.go`.

- 디렉터리 scan(`catalog.go:112-131`): `ReadDir(128)`을 EOF까지 반복하고 EOF가 아닌
  오류면 `truncated=true`로 부분 결과를 반환한다. `.issueops` open 실패는
  `ScanTruncated=true`(`:76-78`). parent가 추가한 `TestDiscoverReportsUnreadableDirectory`가
  이 경로를 검사하며 통과한다.
- selector(`:136-186`): 필수→선택→기타 사전순으로 상위 64개를 정렬 슬라이스에 유지하고
  탈락분을 `overCap`에 센다. 가득 찬 상태에서 끝에 들어갈 이름은 즉시 탈락, 중간에
  들어갈 이름은 마지막을 밀어내며 둘 다 `overCap++`이므로 `over_cap = 전체 − 64`다.
  메모리는 batch+64개로 bounded다.
- header read(`:196-226`): `Lstat`→`Open`→`fstat` 뒤 `Size() > 256KiB`면 본문을 읽지
  않고 oversize로 센다. `io.ReadFull`로 8KiB만 읽고, 잘린 경우 마지막 개행까지만
  해석한다(개행이 없으면 빈 header). `os.OpenRoot`와 symlink 거부는 유지된다.
  읽기 byte 상한 64×8KiB는 `TestDiscoverReadsBoundedHeader`가 `BytesRead ≤ 9×8192`로 단언한다.
- 교체한 두 기존 테스트(합계 2MiB 상한, 단일 `ReadDir(128)` 고정)는 계약 §4 I7이
  폐기한 불변식이며, 같은 입력을 새 불변식으로 다시 단언했다. 약화가 아니다.
- `hookprompt/catalog.go:22-49`: 생략이 없으면 compact/user view를 바꾸지 않는다
  (`TestBuildWithoutOmissionsIsByteIdenticalToDiscoverPath`가 JSON 동일성을 검사).
  `Omitted`는 `omitempty` pointer이며 `contractgolden`은 통과했다.
- 사소한 관찰 2건, 결함 아님: scan 시점과 `Lstat` 사이에 regular file이 symlink로 바뀌면
  `unreadable`로 센다(TOCTOU, 안전 방향). frontmatter가 닫히지 않은 header에서
  `firstMarkdownHeading`은 YAML 줄까지 훑는데, 이는 기존 `firstMarkdownTitle`과 같은 동작이다.

실제 바이너리: `.issueops`에 `0-00..0-69`(70), `pad-00..59`(60), 240KiB `big-0..8`(9),
257KiB `zz-oversize.md`, `ARCHITECTURE.md`, `/etc/hosts`를 가리키는 `link.md`를 만들고
`issueops hook session-start --repo <fixture> --json </dev/null`을 실행했다(exit 0).
stdin을 닫지 않으면 hook이 stdin JSON을 기다린다.

```text
docs=64 omitted={"over_cap": 77}        # 적격 141개(symlink 제외) − 64
first5=0-00..0-04  last3=0-61, 0-62, ARCHITECTURE.md
compact_tail=…; omitted: over_cap=77
user_view_tail=⚠ 일부 문서가 목록에서 생략됨: over_cap=77
link.md 포함 여부=false  zz-oversize 포함 여부=false
```

`--host claude` 출력에도 같은 생략 줄이 있다. 보고서 값과 일치한다.

## 8. 트리 수준 미해결 사항 (lane 결함 아님, 소유자 명시)

1. **`cmd/issueops/selfworkflow/summary/self_augment_summary_test.go:118`이 version 6을 고정한다.**
   현재 트리에서 이 패키지는 exit 1이다. 계약 §5 표의 B 범위에 이 패키지는 없고
   "나열하지 않은 공유 파일은 Z만 수정한다"에 해당한다. 기계 소비 값(version) 갱신이므로
   Z-integration이 7로 바꿔야 한다. B의 보고서가 이미 지적했다.
2. **`cmd/issueops/testdata/response_contracts.golden.json`에 새 필드가 없다.**
   `"complete"`(I1), `"reused"`(I2 StepResult), `"reused_count"`(I2 stat) 모두 0건이다.
   `TestResponseContractsGolden`은 첫 불일치 `reused_count`에서 멈추므로 그 뒤
   `reused`·`complete`·v7 projection도 차례로 드러난다. 계약 §5가 golden 갱신을 Z의
   마지막 단계로 정했으므로 Z-integration 소유다. 이 실패를 통과로 보지 않았다.
3. **`internal/architecture/testdata/ddd_responsibility_inventory.json`이 오래됐다.**
   `-update-ddd-inventory`로 임시 재생성해 원본과 diff한 뒤 원본을 복원했다
   (`cmp` 동일, `git status` 깨끗). 누락 심볼의 출처는 세 갈래다.
   - I6: `nextGitObservation`(type, `.out/.currentBranch/.worktreeState`), `newNextGitObservation`,
     `commitHistory`(type, `.last/.commits/.bodies`), `historyRecord`, `parseHistory`;
     제거 `observeIssueOpsWorktreeState`, `recentCommits`.
   - I7: `DiscoverProjectDocsReport`, `FormatProjectDocCatalogOmissions`, `scanProjectDocNames`,
     `projectDocNameSelector`(type, `.add/.before/.names/.rank`), `newProjectDocNameSelector`,
     `projectDocDirReader`, `readProjectDocCatalogHeader`, `buildProjectDocCatalogEntry`,
     `firstMarkdownHeading`, `CatalogOmissions`(type, `.Any/.Summary`), `CatalogStats`,
     `RenderProjectDocCatalogOmissions`; 제거 `firstMarkdownTitle`, `readProjectDocCatalogEntries`,
     `readProjectDocCatalogFile`.
   - parent(이번 판정 제외): `contract/authority`, `contract/issueops/verified_actor.go`,
     `contract/mcpservice`, `port/authority`, `port/mcpservice`, sqlstore `record_guard.go`·
     `spanRun`·`DB.commitData/runSpan/withSpan`, issueopsrecord `optionalMilliseconds`.
   I1과 I2는 새 심볼을 만들지 않았다. 계약 §5대로 Z가 마지막에 한 번 갱신한다.
4. **cross-version duration 비교 경계.** `internal/domain/selfaugment/history_compare.go:10-19`의
   `CompareSnapshots`는 baseline/candidate의 contract version·hash를 대조하지 않고
   `StepDurationStatsForCompare`와 slowest를 비교한다. 호출자 `application/selfaugment/history.go:134`도
   검사하지 않는다. summary 생성(`SummarizeSteps`)은 v6/v7 series를 합치지 않지만, 사용자가
   v6 snapshot과 v7 snapshot을 명시적으로 compare하는 경로는 막혀 있지 않다. 계약 문장
   "v6와 v7 duration series를 합치지 않는다"를 compare까지 적용할지는 parent/Z의 결정이 필요하다.
   B의 보고서가 지적한 내용과 같다.
5. `.issueops/TECH_STACK.md:99,104`의 SDK pin 설명은 v1.6.1이다(I5 보고서 지적). 문서 owner 소유다.

## 9. 보고서 주장 중 이번에 입증하지 않은 것

- 각 보고서의 `mon_*` monitor ID와 RED 출력 전문은 다른 세션의 전사이며 직접 보지 않았다.
  대신 같은 명령을 다시 실행해 GREEN을 재현했다. RED는 재현하지 않았다.
- 각 보고서의 `PI_MODEL`/`PI_REASONING_LEVEL` 기록(I1/I2/I5 `gpt-6.1-sol`/high,
  I6/I7 `claude-sonnet-5-5`/high)은 해당 세션의 export 주장이며 이 세션에서 확인할 수단이 없다.
  `execution-ledger.md`는 설계 DAG의 모델만 적고 구현 DAG의 모델은 적지 않았다.
- I1 보고서의 LSP 진단 0건, I2 보고서의 변경 파일 14개 LSP 0건은 LSP 대신 `go vet`
  (exit 0)과 `gofmt -l`(출력 없음)로 대체 확인했다.
- I5 보고서의 실제 바이너리 stdio 4회 실행(tools 51개, docs 557개, `resultType=complete`)은
  재현하지 않았다. 패키지 테스트가 같은 production 진입점의 matrix를 검사한다.
- I6 보고서의 "변경 전 전체 10회"는 소스 기준 계산이며 측정하지 않았다. 변경 후 7회·log 1회만 측정했다.
- I7 보고서가 밝힌 대로 옛 코드의 byte 수와 벤치마크는 측정되지 않았고, 이번에도 측정하지 않았다.
  읽기 상한은 `TestDiscoverReadsBoundedHeader`의 `BytesRead` 단언으로만 확인했다.
- 전체 저장소 `go test ./...`/`-race`, API 문서 게이트, 세 호스트 설치·HTTP 검증은 이 범위 밖이며
  실행하지 않았다. parent의 authority/sqlstore 변경이 섞인 트리에서 다섯 lane 패키지와
  `cmd/issueops/issueopsapp`(focused), `cmd/issueops/mcpcli`, `internal/adapter/issueops`는 통과했다.
