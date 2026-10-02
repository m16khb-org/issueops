# I8 구현 증거: SQL span 단계 지연과 I10 WithRecordGuard

기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e` (commit/push/설치 없음).
이 문서는 owner E 노드의 증거다. 전체 저장소 battery와 호스트 검증은 주장하지 않는다.

## 실행 환경

이 노드 세션의 `tool.bash`로 두 번 확인한 값이다. 환경 변수는 바꾸지 않았다.

```text
PI_MODEL=claude-opus-5-5
PI_REASONING_LEVEL=high
```

부모가 알린 `gpt-6-astra/high`와 모델 값이 다르다. effort는 high로 같다.
모델 구성은 고치지 않았고 불일치만 보고한다.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/adapter/outbound/sqlstore/span_observer.go` | `SpanObservation`에 `Acquired`, `Callback`/`Commit *time.Duration`, `Total`, `CommitCount`, `CommitCoverage`를 추가했다. `SpanCommitCoverageComplete/Unknown` 상수와 context-local 누적기 `spanRun`도 이 파일에 있다. |
| `internal/adapter/outbound/sqlstore/sqlstore.go` | `WithSpan`은 `withSpan(ctx, time.Now, fn)`에 위임하고, lock 구간은 `runSpan`이 맡는다. Apply와 CompareAndApplyFunc의 data commit은 `commitData`를 거친다. Put, Delete, DeleteBucket과 span에 귀속되지 않는 commit은 `DB.unattributedWrite`로 `unattributedEpoch`/`unattributedInFlight`를 갱신한다(리뷰 보정). 테스트 전용 `dbTestHooks`도 이 파일에 있다. |
| `internal/adapter/outbound/sqlstore/record_guard.go` (신규) | `WithRecordGuard(ctx, bind func(context.Context, authorityport.RecordReader) (context.Context, error)) context.Context`와 `bindRecordGuard`. `*DB`가 `authorityport.RecordReader`를 만족한다는 compile-time 단언을 둔다. |
| `internal/adapter/outbound/issueopsrecord/store.go` | context를 잃던 세 호출(Update의 Put, UpdateRelated의 Delete와 Put)을 각각 단일 mutation `database.Apply(spanContext, ...)`로 바꿨다. |
| `internal/adapter/outbound/issueopsrecord/observer.go` | JSON에 `acquired`, `callback_ms`, `commit_ms`(nullable), `total_ms`, `commit_count`, `commit_coverage`를 추가했다. 기존 slow/contended/error 필터는 그대로다. |
| `sqlstore/span_observer_test.go`, `sqlstore/record_guard_test.go`(신규), `sqlstore/span_coverage_test.go`(신규, 리뷰 보정), `issueopsrecord/observer_test.go` | 아래 회귀 테스트. |
| `sqlstore/span_observer_benchmark_test.go`(신규), `issueopsrecord/observer_benchmark_test.go`(신규) | observer overhead benchmark. |

`internal/port/authority/authority.go`는 읽기만 했다. global golden/inventory, 설치 설정,
다른 owner 파일은 수정하지 않았다.

## 단계 의미 (구현 그대로)

| 필드 | 경계 | 비고 |
|---|---|---|
| wait | 진입 → SQLite span lock 획득 | local gate 대기를 포함한다. lock을 얻지 못하면 진입부터 반환까지이며 `acquired=false`다. |
| callback | `fn` 진입 → 반환(panic 포함) | inclusive라서 commit을 포함한다. 미진입(bind 실패, 대기 중 취소)이면 `nil`이다. |
| commit | span context로 실행한 data `tx.Commit()` 호출 시간의 합 | callback의 부분집합이다. 실패한 Commit 호출도 실제 호출이므로 센다. lock rollback은 포함하지 않는다. |
| hold | lock 획득 → lock rollback 완료 | bind와 callback을 포함한다. |
| total | 진입 → gate 반환 완료 | observer는 total을 닫은 뒤 호출되므로 어느 단계에도 들어가지 않는다. |

단계는 서로 겹치므로 더하지 않는다. 내부는 monotonic `time.Time` 차이를 ns로 유지하고,
JSON은 ms floor다. sub-ms 값이 0으로 찍혀도 measured 0이지 unknown이 아니다.

**누적 격리.** `spanRun`은 context 값이고 부모 사슬을 가진다. `commitData`는 사슬에서
**자기 `*DB` 핸들의 활성 run**만 찾는다. 그래서 다른 root의 중첩 span 안에서 바깥 root에
쓴 commit도 바깥 span에 귀속된다. 동시에 도는 다른 root span은 서로 섞이지 않는다.
전역 current-span은 없다.

**coverage.** 귀속 불가 write(Put, Delete, DeleteBucket의 autocommit, span 밖 context의
data commit)는 `DB.unattributedWrite`를 거친다. 이 함수는 write 전체 동안
`unattributedInFlight`를 0이 아니게 두고, 시작과 끝에 `unattributedEpoch`를 하나씩 올린다.
span은 gate를 얻은 뒤, SQLite lock을 잡기 **전에** epoch→in-flight 순서로 표본을 뜬다.
lock rollback이 끝난 **뒤에** epoch를 다시 읽는다. baseline에 in-flight가 있었거나 epoch가
바뀌었으면 `commit_coverage=unknown`, `commit_ms=null`이다. 그렇지 않을 때만 `complete`이며,
commit 합계(관측된 미실행이면 0)를 보고한다. hold 중에 commit된 귀속 불가 write는
두 경우 중 하나로 반드시 잡힌다. baseline 이후에 시작했다면 시작 epoch가 바뀐다.
baseline 이전에 시작했다면 in-flight 표본에 걸린다. 표본을 뜬 시점에 이미 끝난 write는
lock 획득 전에 commit된 것이다. 표본을 lock 획득 전에 뜨므로, SQLite lock을 기다리는 동안
끝난 write도 보수적으로 unknown이 된다.
이 판정은 의도적으로 보수적이다. 같은 핸들에서 span과 무관한 동시 writer가 있어도
unknown이 된다. 0이나 Exec 전체 시간으로 대체하지 않는다. `commit_count`는 관측한
commit 수이므로 unknown과 함께 1 이상일 수 있다(아래 실증 3번째 줄).

**panic.** 기존 코드는 named error가 nil인 채 defer가 돌아 panic을 `success`로 보고했다.
이제 정상 반환 플래그로 `error` outcome을 기록하고, observer를 호출한 뒤 panic은 그대로
전파된다(recover/re-panic 없음). gate와 lock 해제 순서는 바뀌지 않았다.

## WithRecordGuard (I10 hook만)

- lock 획득 후, callback 전에 binder를 정확히 한 번 호출한다. reader는 같은 root의 `*DB`(`Get`)다.
  binder에 넘기는 context는 이미 span chain을 갖고 있으므로 binder가 같은 root span을 열면
  기존 `NestedSpanError`로 거부된다. 테스트로 확인했다.
- 반환 context로 `fn`을 실행한다. 그 context가 이 span의 활성 run에서 파생되지 않았으면
  (nil이나 `context.Background()` 등) chain과 commit 누적이 끊기므로 `fn` 없이 오류로 끝난다.
- bind 오류는 감싸지 않고 그대로 반환한다(errors.Is 보존, state 경로 비노출).
  `fn`과 data write는 일어나지 않고, defer로 lock과 gate를 푼다. outcome은 error,
  `callback=null`, `commit=0/complete`다.
- guard는 binder 호출 직전에 소비한다. callback 안에서 다른 root span을 열어도 다시 bind하지 않는다.
  G가 root별 재-bind를 원하면 바꿔야 할 결정이다.
- grant 검증, ancestry, TTL은 구현하지 않았다(G 소유).

## Apply 전환의 행동 변화

세 호출은 각자 자기 트랜잭션에서 commit하고 span 전체를 하나로 묶지 않으므로, write 직후
reader에게 보이는 기존 경계를 유지한다. 대신 다음 두 가지가 달라졌다.
(1) `spanContext`가 commit 전에 취소되면 write가 실패한다(기존 Put은 Background로 무시했다).
`commitData`는 `tx.Commit` 직전에 `ctx.Err()`를 명시적으로 확인하고, 취소됐으면 Commit을
부르지 않은 채 `context.Canceled`를 돌려준다. commit이 끝난 뒤의 취소는 rollback하지 않는다.
(2) `Apply`의 bucket/id 비어 있음 검증이 적용된다. 현재 호출자는 상수 bucket만 넘긴다.

## RED

```sh
go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1
```

exit 1. 두 패키지 모두 `[build failed]`였다. `undefined: WithRecordGuard`,
`unknown field Acquired/Callback/Commit/Total/CommitCount/CommitCoverage`,
`undefined: SpanCommitCoverageComplete/Unknown`,
`observation.CommitMS/CallbackMS undefined` 등이다.

assertion RED는 `go test -overlay`(임시 디렉터리, 저장소 파일 변경 없음)로 panic 테스트만
현행 production에 붙여 얻었다.

```text
=== RUN   TestWithSpanReportsPanicAsErrorOutcome
    span_observer_test.go:23: panic observations=[{Outcome:success Contended:false Wait:22.917µs Hold:7.833µs}] want outcome "error"
--- FAIL: TestWithSpanReportsPanicAsErrorOutcome (0.01s)
FAIL	issueops/internal/adapter/outbound/sqlstore	0.392s
```

## GREEN

| 명령 | exit | 결과 |
|---|---|---|
| `gofmt -l internal/adapter/outbound/sqlstore internal/adapter/outbound/issueopsrecord` | 0 | 출력 없음 |
| `go vet ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord` | 0 | 출력 없음 |
| `go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1` | 0 | ok 5.8s / 1.3s, 재실행 ok 6.3s / 1.3s |
| `go test -race ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1` | 0 | ok 10.8s / 2.4s |
| `go build ./...` | 0 | 출력 없음 |
| `go test ./internal/adapter/outbound/... -count=1` | 0 | ok가 아닌 줄 없음 |
| `go test ./cmd/issueops/issueopsapp -run IssueOpsRecord -count=1` | 0 | ok |

`-v` 실행에서 새 테스트와 기존 span 테스트가 모두 PASS로 찍혔다.

- 새 테스트: `TestSpanPhaseBoundaries`(success_with_commit, panic_after_commit), `TestWithSpanReportsPanicAsErrorOutcome`,
  `TestSpanCommitCoverage`(no_commit, multiple_commits, failed_commit_call, context_losing_put,
  background_apply, concurrent_unattributed_write, unentered_callback), `TestSpanCancelledContenderReportsUnacquired`,
  `TestSpanCommitAccumulatorIsolation`(nested_different_root, concurrent_roots), `TestSpanObserverReentersAfterRelease`,
  `TestWithRecordGuard*` 3개, `TestStoreWritesCommitInsideObservedSpan`, `TestJSONLineObserverEmitsStageFieldsFromSQLiteSpan`,
  `TestJSONLineObserverKeepsUnknownStagesNull`
- 기존 테스트: `TestWithSpanCancelsLocalWaiter`, `TestWithSpanCancelsSQLiteWaiter`, `TestWithSpanPanicReleasesGate`,
  `TestWithSpanReportsSuccessAndFailureAfterReleasingLock`, `TestJSONLineObserverEmitsOnlyActionableRedactedSpanEvents`,
  `TestStoreScopesSpanObservationByCapability`, cross-process 테스트

결정성 장치는 다음과 같다. 단계값은 scripted clock(`withSpan`의 `now` 주입)으로 정확히 비교한다.
경합과 취소는 holder의 `entered` channel을 받은 뒤 이미 취소된 context로 트리거한다.
commit 실패는 clock의 commit 직전 읽기 hook에서 cancel해서 만든다. lock 보유 여부는 같은 root의
uncached 두 번째 핸들에서 `BEGIN IMMEDIATE`가 BUSY인지로 판정한다. 동시 root는 서로의
`entered`를 기다리는 barrier로 겹치게 한다. 새 테스트에는 sleep이나 시간 비교 assertion이 없다.

## 리뷰 보정 (`storage-observation-verification.md` 미승인 항목)

기존 구현은 그대로 두고 지적된 결함만 고쳤다. 바뀐 파일은 `sqlstore/sqlstore.go`와 신규
`sqlstore/span_coverage_test.go` 두 개다. 실행 환경은 이번에도 세션 `tool.bash`로 확인했고
`PI_MODEL=claude-opus-5-5`, `PI_REASONING_LEVEL=high`이었다.

| 지적 | 보정 | 고정 테스트 |
|---|---|---|
| CAS commit 귀속을 고정하는 테스트 없음(M1 생존) | 테스트 추가. 소스는 이미 맞았다. | `TestSpanCompareAndApplyCommitIsAttributed`(compare_and_apply, compare_and_apply_func): scripted clock으로 Commit 5ms, count 1, complete를 정확히 비교한다. |
| Delete/DeleteBucket unknown 미고정(M2, M3 생존) | 테스트 추가 | `TestSpanAutocommitDeletesAreUnknown`(delete, delete_bucket) |
| baseline 이전에 시작한 귀속 불가 write가 hold 중 commit되면 놓침 | `unattributedWrites` 카운터를 `unattributedEpoch`(시작·끝에 증가)와 `unattributedInFlight`로 바꿨다. baseline은 SQLite lock 획득 전에, 비교는 lock rollback 뒤에 한다(위 coverage 절). | `TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline`(end_recorded_during_hold, end_recorded_after_release) |
| span 도중 다른 핸들 가시성 실행 증거 없음 | 테스트 추가 | `TestSpanApplyIsVisibleToAnotherHandleBeforeSpanExit`: Store가 쓰는 두 모양(단일 upsert, 단일 delete)의 `Apply(spanCtx)`를 매번 channel로 reader goroutine에 요청한다. reader는 같은 root의 uncached 두 번째 핸들에서 row를 읽고, 같은 시점에 span lock이 아직 BUSY인지도 확인한다. |
| 명시적 commit 직전 취소 확인 없음 | `commitData`가 `tx.Commit` 전에 `ctx.Err()`를 확인한다. 취소됐으면 Commit을 부르지 않고 commit 합계에도 넣지 않는다. | `TestSpanCommitChecksCancellationBeforeCommit` |
| commit 뒤 취소 보존 미고정 | 동작 변경 없음 | `TestSpanCancellationAfterCommitKeepsData`: 취소로 span이 canceled를 보고해도 다른 핸들에서 row가 보이고 count 1/complete다. |
| guard 1회 소비 | 유지했다. G는 고정 user-state DB를 쓰고 request-scope 강제는 G가 맡는다. | 기존 `TestWithRecordGuard*` |

결정성 장치는 테스트 전용 `DB.hooks`(production에서는 모두 nil) 세 개다.
`unattributedWriteStarted`는 in-flight 기록 뒤·write 실행 전에, `unattributedWriteFinished`는
write가 끝난 뒤·끝 epoch 기록 전에, `beforeDataCommit`은 취소 확인 직전에 불린다.
race 테스트는 두 번째 핸들이 data DB의 `BEGIN IMMEDIATE`를 잡아 Put이 시작 기록 뒤 commit하지
못하게 막는다. `started`를 받은 뒤 span을 열고 callback 안에서 그 lock을 푼다. 그러면 commit이
반드시 hold 구간에 일어난다. `end_recorded_after_release`는 끝 기록을 span 반환 뒤로 미룬다.
그래서 epoch 비교만으로는 보이지 않고, baseline in-flight 표본만이 이 경우를 잡는다. sleep은 없다.

### RED (수정 전 코드 + 테스트 hook)

수정 전 코드에 hook 호출만 넣고 새 테스트를 돌렸다(`-run` 6개, `-v`). CAS, delete, 가시성,
commit 뒤 취소 4개는 현행 소스에서도 통과했다(회귀 고정용). 결함 2건은 아래처럼 실패했다.

```text
    span_coverage_test.go:115: write in flight at the baseline committed during the hold but was not reported: {Outcome:success Contended:false Acquired:true Wait:29.375µs Hold:90.542µs Callback:83.917µs Commit:0s Total:120µs CommitCount:0 CommitCoverage:complete}
--- FAIL: TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline (0.01s)
    span_coverage_test.go:137: pre-commit cancellation still called Commit: {Outcome:canceled Contended:false Acquired:true Wait:42.792µs Hold:331.583µs Callback:330.792µs Commit:375ns Total:374.5µs CommitCount:1 CommitCoverage:complete}
--- FAIL: TestSpanCommitChecksCancellationBeforeCommit (0.01s)
FAIL	issueops/internal/adapter/outbound/sqlstore	0.406s
```

### 변이 확인 (`go test -overlay`, 저장소 미수정)

첫 수정에서는 M8(in-flight 표본 제거)이 살아남았다. 끝 epoch가 hold 안에 기록되면 epoch 비교만으로도
잡혔기 때문이다. 그래서 `unattributedWriteFinished` hook과 `end_recorded_after_release` 경우를
추가했고, 재실행하자 모든 변이가 잡혔다.

| 변이 | 실패한 테스트 |
|---|---|
| M1 `CompareAndApplyFunc`가 `commitData` 대신 `tx.Commit()` | `TestSpanCompareAndApplyCommitIsAttributed` 두 경우 |
| M2 `Delete`를 추적 없이 실행 | `TestSpanAutocommitDeletesAreUnknown/delete` |
| M3 `DeleteBucket`을 추적 없이 실행 | `TestSpanAutocommitDeletesAreUnknown/delete_bucket` |
| M8 baseline in-flight 표본 제거 | `TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline/end_recorded_after_release` |
| M9 commit 직전 `ctx.Err()` 확인 제거 | `TestSpanCommitChecksCancellationBeforeCommit` |
| M10 baseline을 lock 획득 뒤로 옮기고 표본 제거(수정 전 방식) | `TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline/end_recorded_after_release` |

### GREEN (보정 후)

| 명령 | exit | 결과 |
|---|---|---|
| `gofmt -l` (두 패키지) | 0 | 출력 없음 |
| `go vet ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord` | 0 | 출력 없음 |
| `go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1` | 0 | ok 5.640s / 0.973s |
| `go test -race ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1` | 0 | ok 9.827s / 2.288s |
| `go test -race ./internal/adapter/outbound/sqlstore -run TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline -count=20` | 0 | ok 1.957s |
| `git diff --check -- internal/adapter/outbound/sqlstore internal/adapter/outbound/issueopsrecord` | 0 | 출력 없음 |

`-v` 실행에서 새 테스트 6개와 기존 span/guard 테스트 전부가 PASS였다.

## 실제 임시 SQLite 실증

보정 후 코드로 다시 실행했다. 임시 시나리오 테스트(`/tmp/i8-evidence/zz_i8_evidence_test.go`)를
`go test -overlay`로 붙였고, 저장소에는 파일을 남기지 않았다. `Store.Update`를 실제 임시 state root에서
돌리고 `NewJSONLineObserver(os.Stdout, ...)`로 출력했다. 1-2줄은 기본 100ms 필터를 통과한 actionable
이벤트다. 3-5줄은 1ns threshold로 강제 출력했다. sleep은 실증용 부하를 만드는 데만 썼다.

1. 120ms 걸리는 slow callback
2. 60ms holder 뒤에서 기다린 contended 요청
3. callback 안에서 context 없는 `DB.Put`을 한 요청
4. 평범한 갱신
5. callback 안에서 context를 취소한 요청(`err=context canceled`)

```json
{"event":"issueops_record_span","generated_at":"2026-10-02T10:43:08.005002Z","operation":"evidence.update","outcome":"success","contended":false,"wait_ms":0,"hold_ms":121,"acquired":true,"callback_ms":121,"commit_ms":0,"total_ms":121,"commit_count":1,"commit_coverage":"complete"}
{"event":"issueops_record_span","generated_at":"2026-10-02T10:43:08.068355Z","operation":"evidence.update","outcome":"success","contended":true,"wait_ms":62,"hold_ms":0,"acquired":true,"callback_ms":0,"commit_ms":0,"total_ms":62,"commit_count":1,"commit_coverage":"complete"}
{"event":"issueops_record_span","generated_at":"2026-10-02T10:43:08.069436Z","operation":"evidence.update","outcome":"success","contended":false,"wait_ms":0,"hold_ms":0,"acquired":true,"callback_ms":0,"commit_ms":null,"total_ms":1,"commit_count":1,"commit_coverage":"unknown"}
{"event":"issueops_record_span","generated_at":"2026-10-02T10:43:08.070087Z","operation":"evidence.update","outcome":"success","contended":false,"wait_ms":0,"hold_ms":0,"acquired":true,"callback_ms":0,"commit_ms":0,"total_ms":0,"commit_count":1,"commit_coverage":"complete"}
{"event":"issueops_record_span","generated_at":"2026-10-02T10:43:08.070206Z","operation":"evidence.update","outcome":"canceled","contended":false,"wait_ms":0,"hold_ms":0,"acquired":true,"callback_ms":0,"commit_ms":0,"total_ms":0,"commit_count":0,"commit_coverage":"complete"}
```

3줄은 귀속 불가 autocommit 때문에 null/unknown이고, 4줄은 같은 조건에서 측정된 0/complete다.
5줄은 취소된 요청이 Commit을 부르지 않아 count 0, measured 0이다. ms 단위는 floor이므로
sub-ms 단계는 0으로 찍힌다.

## Benchmark (Apple M1 Pro, darwin/arm64, `-count=5`)

```sh
go test ./internal/adapter/outbound/sqlstore -run '^$' -bench '^BenchmarkSpanObserverOverhead$' -benchmem -count=5
go test ./internal/adapter/outbound/issueopsrecord -run '^$' -bench '^BenchmarkStoreSpanObserverOverhead$' -benchmem -count=5
```

| benchmark | ns/op 5회 (중앙값) | B/op | allocs/op |
|---|---|---|---|
보정 후 코드로 다시 잰 값이다.

| benchmark | ns/op 5회 (중앙값) | B/op | allocs/op |
|---|---|---|---|
| sqlstore empty/nil | 13783, 13908, 13045, 13088, 13286 (13286) | 1616–1619 | 21 |
| sqlstore empty/noop | 13071, 13723, 14886, 13158, 13240 (13240) | 1632–1634 | 23 |
| sqlstore one_commit/nil | 242303, 217526, 232350, 220436, 221498 (221498) | 3283 | 54 |
| sqlstore one_commit/noop | 229583, 230265, 220047, 228320, 219390 (228320) | 3299 | 56 |
| store Update nil | 274489, 274615, 291689, 282059, 281839 (281839) | 11748–11925 | 128 |
| store Update noop | 307455, 288159, 305109, 282421, 277446 (288159) | 12052–12085 | 135 |
| store Update json_actionable | 380171, 354204, 282815, 278411, 270915 (282815) | 12686–12738 | 138 |

보정 전(아래 해석의 첫 측정)과 allocs/op는 같다. epoch/in-flight 추적은 atomic만 쓰고, 귀속 불가
경로의 closure는 이 benchmark에 나오지 않는다. 첫 측정 중앙값은 empty/nil 13382, empty/noop 12568,
one_commit/nil 241052, one_commit/noop 236528, Store nil 451547, noop 360019, json 414623이었다.

해석은 측정값 그대로 둔다. ns/op는 SQLite I/O 분산이 observer 차이보다 크다. nil 중앙값이
noop보다 큰 경우도 있으므로 ns 단위 overhead 결론은 내리지 않는다. allocs/op는 안정적이다.
sqlstore에서 observer가 있으면 +2(Callback/Commit pointer), Store 경로는 noop이 +7,
JSON 직렬화가 다시 +3이다. 합격 기준 수치는 만들지 않았다.
변경 전 기준선은 측정하지 않았다. 작업 트리에 다른 owner의 변경이 있어 stash로 되돌릴 수 없고,
benchmark가 새 test helper에 의존하기 때문이다. nil 경로도 이제 `spanRun` 할당과
`context.WithValue` 하나를 항상 거친다.

## 범위와 부모에게 넘길 항목

- `git status`에서 이 노드의 변경은 위 표의 sqlstore/issueopsrecord 파일뿐이다.
  `git diff --check`는 exit 0이었다.
- `go test ./internal/architecture`의 `TestDDDResponsibilityInventoryMatchesSource`가 실패한다.
  overlay 임시 테스트로 got/want를 경로별로 분해했고, 이 노드 몫은 다음과 같다.
  - `sqlstore/sqlstore.go`: `DB.commitData`, `DB.runSpan`, `DB.withSpan`, `spanOutcome` 추가
  - `sqlstore/span_observer.go`: `activeSpanRun`, `spanRun`, `spanRun.invoke`, `spanRun.observation`, `spanRunKey` 추가
  - 신규 `sqlstore/record_guard.go` (owner adapter, task T20): `DB.bindRecordGuard`, `WithRecordGuard`, `recordGuardFunc`, `recordGuardKey`
  - 리뷰 보정으로 `sqlstore/sqlstore.go`에 `DB.unattributedWrite`, `dbTestHooks`가 더해졌다(inventory는 이번에도 미갱신).
  - `issueopsrecord/observer.go`: `optionalMilliseconds` 추가

  같은 실패에 projectdoc, preflight, hookprompt, next_wiring, authority, mcpservice 계약 등
  다른 owner의 drift도 섞여 있다. 지시대로 inventory는 갱신하지 않았다. 의존성 규칙 테스트는
  통과했으므로 `sqlstore → port/authority` edge는 허용된다.
- M은 이제 `issueopsrecord/observer.go`를 이어받을 수 있다. G는 binder가 반드시 받은
  context에서 파생한 context를 반환해야 한다는 점과 guard 1회 소비 결정을 확인해야 한다.

## 한계

- in-process gate 경합에서 `contended=true`는 결정적으로 고정하지 않았다. 대기 select 진입을
  sleep 없이 관측할 hook이 없기 때문이다. 경합 테스트는 획득 실패, 취소, 단계 null을 고정하고,
  `contended=true`는 위 실증 JSON 2번째 줄로만 보였다.
- coverage 카운터는 핸들 단위다. 다른 프로세스나 같은 root의 다른 uncached 핸들이 쓴 write는
  보지 못한다. 그런 write는 이 span의 commit이 아니므로 commit 합계를 틀리게 만들지는 않는다.
- 같은 핸들에서 span과 무관한 writer가 있으면 unknown이 보수적으로 늘어난다. baseline을 SQLite lock
  획득 전에 뜨므로 lock 대기 중에 끝난 write도 unknown이 된다.
- span context로 귀속된 commit이 lock 해제 경계에 걸쳐 끝나는 경우는 추적하지 않는다. callback이
  반환한 뒤에도 span context로 commit하는 goroutine을 남길 때만 생기는 경우이고, 이번 보정 범위
  (귀속 불가 write의 baseline 경합)에는 들어가지 않는다. 후속 후보로 남긴다.
- Store 수준(issueopsrecord)에서 span 도중 가시성을 직접 관찰하는 테스트는 없다. Store의 세 호출은
  mutate 뒤 마지막 동작이 `Apply`라서 commit과 span 종료 사이에 끼어들 지점이 없고, sqlstore의
  unexported hook은 다른 패키지에서 쓸 수 없다. 대신 같은 두 mutation 모양을 sqlstore에서 고정했다.
