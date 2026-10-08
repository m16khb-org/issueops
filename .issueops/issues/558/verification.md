# #558 구현 검증

## 의도 대조

계획 A~G를 구현했다. main CI의 errcheck 1건을 고치고, 로컬 최종 battery에 CI와 같은
golangci-lint 단계를 넣었다. read-only 조회는 bucket을 연결 1회로 읽고, 채널 recv 대기는
context 취소에 반응한다. 채널 메시지는 7일 보존 뒤 `send`가 지운다. issueops 패키지
테스트를 병렬화했고, MCP HTTP 서비스 로그에는 크기 상한을 두었다. 은퇴한 state 경로는
doctor가 보고하고 install/update가 지운다. Omo·omp host adapter는 하나로 합쳤다.
복잡도 리팩터링, hook·바이너리 최적화, 새 기능은 범위에 넣지 않았다.

| intent 성공 기준 | 결과 | 게이트 |
|---|---|---|
| go.mod toolchain golangci-lint가 darwin·linux 0건, PR CI 통과 | 로컬 0건. PR CI는 push 뒤 확인 | G3, G17(ABANDON, 사유는 원장) |
| 최종 battery 문서에 toolchain 고정 lint | `.issueops/testing/self-verification.md` 최종 battery 항목에 추가 | G4 |
| 다른 채널 메시지 500개에서 recv 50ms 이하 | 중앙값 10.9ms (5회: 11.3, 11.3, 10.9, 10.7, 10.2) | G7 |
| recv --wait가 context 취소에 즉시 반환 | application 테스트와 2026-07-28 프로토콜 HTTP 테스트 통과 | G6 |
| send가 7일 지난 메시지를 지움 | adapter·application 테스트 통과 | G5 |
| issueops 패키지 단독 44.5초 이하 | 38.0초(게이트 실행), 31.1·35.0초(위임 실행 측정) | G14 |
| server.log 상한 rotation | 단위 테스트와 실제 서버 smoke 통과 | G9 |
| 은퇴 경로를 doctor가 보고하고 install/update가 commit 뒤 제거 | domain·application·adapter 테스트, doctor·install dry-run smoke | G10 |
| omo/omp 단일 구현, 문구·설정 바이트 동일 | extensionhost 테스트, 변경 전후 출력 비교 일치 | G11 |
| gofmt, vet, race, self-verify | 모두 통과 | G1, G2, G15, G16 |

## 변경과 계약

- A: `internal/adapter/hostprobe/omp.go`의 `defer tx.Rollback()`을 반환값을 버리는 closure로 바꿨다.
- B: `sqlstore.WalkExistingAfter`는 read-only 연결 하나에서 cursor row 존재를 확인하고
  `id > start` 범위를 읽는다. 범위 시작은 호출자가 정하며, 채널은 도메인
  `channeldomain.RangeStart`가 정한다. `WalkExisting`은 이 함수에 위임한다. 채널
  `Effects`는 `ListIDs`/`Get` 대신 `MessagesAfter`를 쓴다. loop는 `Store.ReadAllExisting`,
  health는 `adapter/issueops.VisitIssueOpsExisting`으로 bucket을 한 번에 읽는다.
  `ListIssueOpsIDs`, `IDsAfter`, 채널·loop Store의 `ListExisting` 계열 필드는 소비자가
  없어져 지웠다.
- C: `channelapp.Service.Recv(ctx, req)`는 `Effects.Wait(ctx, d)`가 취소되면 `ctx.Err()`를
  반환한다. CLI는 SIGINT/SIGTERM signal context를, MCP 디스패치는 요청 context를 넘긴다.
  `Send`는 쓰기 뒤 `PruneBefore(now-7d)`를 호출하고, adapter는 `msg-<cutoff ns hex>`
  미만을 `sqlstore.DB.DeleteBefore`로 한 번에 지운다. prune 실패는 send 결과를 바꾸지 않는다.
- D: `internal/adapter/issueops`의 514개 top-level 테스트 중 349개에 `t.Parallel()`을 넣었다.
  제외 기준과 계획 대비 확장은 아래 "결정과 편차"에 있다.
- E: `mcpservice.ServiceLogWriter`는 stderr가 `<state>/mcp-http/server.log`와 같은 파일일
  때만 감싼다. 같은 파일을 가리키는 fd 1·2에 `O_APPEND`를 보장하고, 쓰기 뒤 8MiB를 넘으면
  `server.log.1`로 복사한 뒤 원본을 truncate한다. unit 파일 형식은 그대로다.
- F: `statedomain.RetiredStateEntries`가 은퇴 경로 allowlist를 소유한다. doctor는 이 항목을
  `retired_path`(warning)로 보고하고, `channel`·`upstream` 디렉터리를 정상으로 본다.
  `installapp.RunTransaction`은 dry-run이면 지울 경로를 나열하고, 실제 실행이면 activation
  seal과 `Finalize` 뒤에 `RemoveRetiredState`를 호출한다. adapter는 정확한 이름만
  `Lstat`로 확인하고 symlink와 종류가 다른 항목은 건너뛴다. `daemon/`은 legacy socket이
  응답하거나 `issueops.pid`(JSON `pid`)가 살아 있으면 남긴다. `statecontract.HookFailureLogFile`은 지웠다.
- G: `internal/adapter/extensionhost`의 `Spec{Host, DisplayName, ConfigRoot, SkillsRoot, StdioType}`
  값 `Omo`·`Omp`가 두 host를 표현하고 `adapter/omo`·`adapter/omp`는 지웠다. shell quoting은
  `shelltoken.QuoteWord`, bucket 이름은 `internal/port/record_buckets.go`에 한 번만 정의했다.

CLI/MCP 입력·출력 필드와 MCP 도구 설명은 바뀌지 않았다. response-contract golden과
contract check는 self-verify 안에서 통과했다. `state doctor`에는 issue code 값 `retired_path`가
새로 생겼다. record schema, DB 스키마·인덱스, provider body 계약은 바뀌지 않았다. API
endpoint·DTO·OpenAPI 변경은 없다.

## Side effect

- `channel send`가 `channel_v1` bucket에서 7일이 지난 row를 지운다. 지운 메시지는 복구하지 않는다.
- `issueops install`/`update`가 commit 뒤 state root 바로 아래의 `daemon/`, `hook-failures.jsonl`,
  `hook-metrics.jsonl`, `.last-store-maintain`, `issueops-migration-receipt.json`을 지운다.
  dry-run은 `would remove retired state path ...`로 나열만 한다.
- MCP HTTP 서비스가 `server.log`가 8MiB를 넘을 때 `server.log.1`을 만들고 원본을 비운다.
- 원격 side effect는 없다.

## 성능 측정

| 경로 | 변경 전 | 변경 뒤 | 근거 |
|---|---|---|---|
| `channel recv` (다른 채널 500건 + 대상 1건) | 520ms (인계 자료 실측) | 중앙값 10.9ms | G7 |
| `go test ./internal/adapter/issueops -count=1` 단독 | 74.1s(인계), 66.9s(같은 HEAD 재측정, load 3~5) | 38.0s(G14), 31.1s·35.0s(load 8~9) | G14, 위임 측정 |
| 채널·loop·health 조회 연결 수 | N+1회 | 1회 | 코드 경로, G5·G8 테스트 |

## AI slop 정리와 품질 측정

측정 범위는 변경·추가된 production Go 파일 42개다(`_test.go`와 재생성한 DDD 원장 제외).
SNR은 추가된 줄에서 주석·출력 줄을 noise로 센 shell 근사치다. 복잡도와 중복은
golangci-lint v2.12.2(go1.26.6)의 gocyclo(>6)·dupl(threshold 100)을 `--new-from-rev=HEAD`로
실행했다. 정리 전후에 같은 스크립트를 썼다.

| 지표 | 정리 전 | 정리 뒤 |
|---|---|---|
| SNR | 0.894 (signal 647, noise 76) | 0.895 (signal 651, noise 76) |
| production 함수 복잡도 >6 / >12 | 3 / 0 | 3 / 0 |
| 중복 블록(dupl) | 1쌍 (`host_installers.go` Omo·omp 의존성 조립) | 0 |
| boilerplate 50% 초과 새 파일 | 0 / 7 | 0 / 7 |

- 제거한 것(duplication): `newOmoInstaller`와 `newOmpInstaller`가 같은 `extensionhost.Dependencies`
  조립을 반복하던 것을 `newExtensionHostInstaller(spec, lifecycleExtension)` 하나로 합쳤다.
- 제거한 것(duplication, 구현 리뷰의 비차단 지적): 채널 adapter에 새로 만든 `SleepContext`가
  기존 `adapter/issueops.SleepWithContext`와 같은 함수였다. 이를 지우고 composition root가 기존
  함수를 주입한다. 이 수정 뒤 DDD 원장을 재생성하고 영향 패키지와 lint를 다시 실행했다.
- 남긴 것: `RemoveRetiredState`(12), `WalkExistingAfter`(11), `Service.Recv`(10)는 >6이다.
  각 분기가 계획의 안전 조건(종류 확인, symlink 제외, daemon 생존, dry-run)이나 커서·취소 처리에
  대응하고 >12는 없어서 쪼개지 않았다.
- 정리 뒤 확인: `go vet ./cmd/issueops/issueopsapp`, `go test ./cmd/issueops/issueopsapp
  ./internal/adapter ./internal/adapter/extensionhost`(install 관련 테스트) 통과, `git diff --check` 통과.

## 검증 결과

- RED: `RangeStart`·`RetentionCutoff` 미정의로 domain 테스트 build 실패, retired state 단계
  누락으로 transaction 테스트 2건 실패, `RetiredStateEntries` 미정의로 doctor 테스트 build 실패를
  확인한 뒤 구현했다. MCP HTTP 취소 테스트는 디스패치에 `context.Background()`를 넘기도록
  임시로 되돌렸을 때 "recv kept waiting after the HTTP request was cancelled"로 실패했다.
- 게이트: G1~G16 met, G17은 ABANDON. 원장 `.issueops/issues/558/gates.md`에 증거가 있다.
- G13: `go test -race -shuffle=on -count=3 ./internal/adapter/issueops` 통과. 위임 실행에서도
  새 shuffle seed로 두 번 연속 통과했고 data race 보고는 없었다.
- G15: `go test -race ./... -count=1` 전체 통과.
- G16: self-verify 27단계 통과. 로컬 `python3`에 `pydantic`이 없어 첫 실행의 Python script
  tests가 `skills/slack-delegate/scripts/test_capability_routing.py` import에서 실패했다. 이 파일은
  이번 변경과 무관하다. CI처럼 `uv venv --python 3.13`에 `scripts/python_test_requirements.txt`를
  설치하고 그 bin을 PATH 앞에 둔 뒤 통과했다.
- SURFACE:
  - E: 8MiB로 채운 `server.log`를 stderr로 서버를 띄웠다. ready 줄을 쓴 뒤
    `server.log.1`(8,388,674B)이 생기고 원본은 103B였다. `O_APPEND` 없이 연 fd로 띄워도
    원본은 103B였다. truncate 뒤 구멍(sparse)이 생기지 않았다.
  - F: 임시 state에 은퇴 경로 5개와 `channel/`을 만들었다. `state doctor`는 은퇴 경로 5개만
    `retired_path`로 보고했고, `install --dry-run --json`은 5개를 `would remove ...`로 나열했다.
    실제 install은 실행하지 않았다. `ResolveStableNativeRoot`가 worktree를 source checkout으로
    해석해 source의 `bin/issueops`를 대상으로 잡기 때문이다. 실제 삭제는 adapter 테스트
    (allowlist, symlink 보존, live PID·socket 보존, 종료된 PID)와 transaction 순서 테스트로 확인했다.
  - G: 위임 실행이 변경 전후 Omo·omp install 결과(stdio, stdio project-local, HTTP, HTTP
    project-local, dry-run)와 생성 파일 바이트를 캡처해 비교했다. temp 경로에 따라 달라지는
    inode·sha 필드를 가린 `diff -r`은 비어 있었다. `install --dry-run --json` 두 종류는
    `cmp` 결과 같았다.

## 결정과 편차

status.decisions에 기록했다.

- `WalkExistingAfter`는 계획의 `afterID` 인자 대신 `cursor`와 `start func(cursorExists bool) string`을
  받는다. 같은 연결에서 존재를 확인하면서 커서 규칙을 도메인에 두기 위해서다.
  `Service.read`의 observed 집합은 결과가 같아 지웠다.
- shell quoting은 `installutil`이 아니라 `domain/shelltoken`, bucket 이름은 `internal/port`에 있다.
  `TestProductionGraphHasNoForbiddenAdapterEdges`가 adapter 간 import를 막기 때문이다.
- 테스트 병렬화는 계획 기준에 더해 `processlease.Acquire/Drain`에 닿는 135개
  (fork/exec의 fd 상속으로 LOCK_NB flock이 잡힌 것처럼 보임)와 MemStats를 재는 1개를 제외했다.
- channel 문서는 실제 channel 절이 있는 `.issueops/operations/guides/cli-and-mcp.md`에 썼다.

## 남은 위험

- 병렬화한 테스트 일부는 `ps`·`lsof` 시스템 프로브(상한 3초)에 닿는다. 로컬 실측은 0.1~0.25초이고
  G13·G15 부하 실행에서 실패는 없었다. `.issueops/testing/concurrency-and-race.md`가 말하는
  부하 의존 위험은 남는다. CI에서 실패하면 프로브 상한을 늘리지 않고 저장소 규칙대로
  관측자를 주입해 고친다.
- 2026-07-28 이전 프로토콜의 HTTP client가 끊어지면 서버 쪽 recv 대기는 `timeout_seconds`
  (기본 300초)까지 이어진다. go-sdk v1.8.0의 한계이며 `cli-and-mcp.md`에 적었다.

## 재실행 명령

```bash
gofmt -l $(git ls-files '*.go')
go vet ./...
GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...
GOOS=linux GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...
go test -race ./... -count=1
go test -race -shuffle=on -count=3 ./internal/adapter/issueops
go build -o bin/issueops ./cmd/issueops
PATH=<CI와 같은 python venv>/bin:$PATH ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
issueops gates check --file .issueops/issues/558/gates.md --cwd "$PWD" --workspace-root "$PWD" --network --timeout-seconds 900 --json
```
