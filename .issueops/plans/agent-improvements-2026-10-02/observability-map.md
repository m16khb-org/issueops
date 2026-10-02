# I8/I9 observability 구현 지도

기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`.
Effective `PI_MODEL=gpt-6.1-sol`; `PI_THINKING_LEVEL`은 export되지 않았다.
이 문서는 설계 근거이며 production 구현이나 host 실행 검증 결과가 아니다.

## Repo grounding

- `internal/adapter/outbound/sqlstore/sqlstore.go:1-11,340-413`:
  `WithSpan`은 local gate와 별도 lock DB를 잡고, 종료 시 rollback한다.
  현재 wait는 획득 전 시간이고 hold는 callback과 lock rollback을 포함한다.
  lock transaction의 rollback을 data commit으로 이름 붙이면 틀린다.
- 같은 파일 `:700-722,738-787`: 실제 data `tx.Commit()`은
  `Apply`와 `CompareAndApplyFunc` 안에 있다.
  `Put(:675)`와 `Delete(:823)`의 autocommit `Exec`에서는 statement와
  commit을 분리할 수 없다.
  `internal/adapter/outbound/issueopsrecord/store.go:135-153,174-198`은
  callback 안에서 context 없는 Put/Delete를 호출한다.
- `internal/adapter/outbound/sqlstore/span_observer.go:15-37`,
  `internal/adapter/outbound/issueopsrecord/observer.go:18-95`에 기존
  observer와 wait/hold, slow/contended/error 출력 필터가 있다.
- `internal/adapter/trace/decode.go:10-38`, `input.go:13-25`,
  `internal/domain/trace/observation.go:24-45`,
  `internal/application/trace/service.go:22-74`는 입력 파일/stdin/state를
  분석하는 기존 표면이다. 현재 usage나 W3C context를 해석하지 않는다.
- 역사적 `.issueops/adr/decisions/2026-07-02-external-llm-usage-observation.md`
  의 writer 경로는 현재 없다. 실행 소스 검색에서 해당 usage prefix는
  `internal/adapter/outbound/state/state_test.go:340-358`의 retention fixture에만
  남는다. ADR을 현행 writer나 수집 권한으로 취급하지 않는다.

## Decision-complete plan: I8

기존 observer를 확장한다. 새 profiler나 exporter는 만들지 않는다.
monotonic clock으로 다음 경계를 측정하고, observer 호출 시간은 제외한다.

| 필드 | 정확한 의미 |
|---|---|
| wait | span 진입부터 SQLite lock 획득까지; local gate 대기도 포함한다. |
| callback | `fn` 진입부터 반환까지의 wall time; data commit을 포함한다. |
| commit | data `tx.Commit()` 호출 구간; callback의 부분집합이며 별도 단계다. |
| hold | lock 획득부터 lock rollback 완료까지다. |
| total | span 진입부터 lock/gate 해제 완료까지다. |

callback/commit/hold를 더해 total을 만들지 않는다. callback은 inclusive라고
schema에 명시한다. 여러 commit은 count와 duration을 함께 보고하며, 병렬
commit의 합을 wall time으로 표시하지 않는다. 정확한 exclusive callback이
필요하면 commit 구간의 합이 아니라 구간 합집합을 빼야 한다.

`Apply`/CAS에는 context로 전달되는 span-local accumulator를 둔다.
다른 root의 nested span, 동시 요청, 실패한 commit을 구별하고 전역 current-span은
금지한다. commit 미실행을 확인한 경우만 count=0/duration=0으로 표현한다.
관측하지 못한 autocommit은 `commit_ms:null`과 coverage=unknown으로 표현한다.
이를 0이나 Exec 전체 시간으로 대체하지 않는다.
callback 미진입도 callback_ms=null이다. lock 미획득은 기존 hold_ms=0을
유지하되 acquired=false로 구별한다. 관측된 commit_count=0만으로 context 없는
callback의 data write 부재를 확정하지 않는다.

기본 선택은 autocommit 의미를 보존하는 것이다. 전체 commit coverage가 수용
조건이면 bounded alternative로 Store의 세 Put/Delete 호출을 단일 mutation
`Apply(spanContext, ...)`로 바꾼다. 각 write가 끝난 뒤 reader에게 보이는 기존
경계는 유지하고, span 전체를 하나의 data transaction으로 묶지 않는다.
이 선택은 아래 미결정 항목이다.

기존 wait_ms/hold_ms와 필터를 유지하고 callback_ms/commit_ms/total_ms,
commit_count/coverage를 추가한다. 내부 duration은 ns 정밀도를 유지한다.
출력에서 sub-ms가 0으로 반올림된 경우도 measured이지 unknown이 아니다.
rollback과 gate 해제 뒤에 observer를 호출한다. panic은 재전파하며 success로
보고하지 않는다. slow/error-only 이벤트는 표본 전체가 아니므로 이 로그만으로
전체 p95나 평균을 계산하지 않는다.

## Decision-complete plan: I9

기존 `trace analyze --input`에 명시적
`--input-format=issueops|claude-stream|codex-exec|omo-json`을 추가한다.
기본값은 issueops이며, host 실행을 새로 시작하거나 home transcript를 찾지 않는다.
입력은 사용자가 제공한 지원 이벤트 export 파일/stdin뿐이다. host 형식에서는
state-key fallback을 금지한다. OTLP receiver, collector, hook telemetry,
SDK dependency, 가격표는 추가하지 않는다.
host 입력은 16 MiB/file, 1 MiB/line으로 제한하고 초과 시 incomplete를 표시한다.
I1의 parser-loss 계약과 충돌하면 reducer가 cap을 통일하며 조용히 잘라내지 않는다.

정규 DTO는 host/version, source scope, session/turn/message identity,
provider/model, finality, temporality, nullable token/cache/cost 필드와
coverage/dedup 경고만 보존한다. 누락/null/지원하지 않는 값은 unknown이고,
유효한 최종 usage가 명시한 0만 measured zero다.

- Claude: `result`의 `modelUsage`를 cumulative authority로 사용한다.
  설치된 SDK 0.3.286의
  `$HOME/node_modules/@anthropic-ai/claude-agent-sdk/sdk.d.ts:5735-5746`
  에서 `usage`는 main-loop per-turn이고, `modelUsage`/`total_cost_usd`는
  streaming-input 누적값이다. 둘을 합산하지 않는다. resume/fork는 이전
  totals를 포함할 수 있고 `/clear`는 reset한다. crash의 zeroed result는
  미관측을 0으로 확정하지 않는다.
- Codex: `codex exec --json`의 `thread.started`/`turn.completed.usage`를
  turn delta로 정규화한다. 연구 근거는
  `.issueops/research/agent-updates-2026-10-02/codex-03.md:8-12`이다.
  cached input은 input의 부분집합이다. raw schema/설치 버전 fixture 확인이
  선행되어야 한다. OTel response/turn 집계를 같은 usage에 더하지 않는다.
- Omo: 지원 JSON stream의 assistant `message_end.message.usage`가 authority다.
  `$HOME/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d/docs/json.md:95-119`
  에서 `message_update.usage`는 cumulative이며 완료 전 0일 수 있다.
  message_end/turn_end/agent_end가 반복한 message를 한 번만 센다.

집계는 한 입력 안에서만 수행한다. delta는 안정적인 event identity로 dedup한다.
누적값은 scope/model/epoch별 마지막 authoritative snapshot을 보존한다.
두 snapshot 사이의 차이만 delta이며 첫 snapshot은 prior baseline을 모른다.
reset/감소/역순/충돌은 경고하고 음수나 추정 delta를 만들지 않는다.
identity가 없으면 중복 제거 가능성을 unknown으로 표시한다. parent totals에
포함된 child totals를 다시 합산하지 않는다. reasoning/cache subset도 중복 합산하지
않는다. host-reported cost는 estimate와 basis를 표시하고 billing으로 부르지 않는다.
Codex 가격을 만들지 않으며, unknown pricing basis는 cost를 unknown으로 남긴다.

## TRACEPARENT와 payload

CLI boundary는 환경의 TRACEPARENT를 한 번 읽어 요청 context에 넣는다.
v00의 길이/hex/nonzero trace-id/parent-id/flags를 검증하고 malformed/미지원
version은 값 없이 경고한다. parent-id는 incoming parent이며 새 span-id가 아니다.
trace context는 correlation이지 actor 권한이나 전체 cycle attribution이 아니다.
I10 HTTP boundary는 요청별 header를 처리하며 서버 환경이나 전역 context를
다른 요청에 재사용하지 않는다.

`.issueops/research/agent-updates-2026-10-02/issueops-05.md:13-17`의
Claude 조건은 telemetry와 beta tracing/exporter 활성화다.
custom `ANTHROPIC_BASE_URL`이면 `CLAUDE_CODE_PROPAGATE_TRACEPARENT=1`도
필요하다. subprocess가 OTEL_* exporter 설정을 상속한다고 가정하지 않는다.
Codex/Omo에는 동일한 자동 전파를 주장하지 않는다.

allowlist projection으로 prompt/thinking/text/tool args/results, endpoint URL,
headers/auth/env 원문을 버린다. ID는 길이를 제한하고 opaque digest로 반환한다.
오류에 raw line을 넣지 않는다. trace-id/parent-id만 검증된 값으로 보존한다.

## File ownership과 의존성

- I8 단독: `internal/adapter/outbound/sqlstore/{sqlstore.go,span_observer.go,
  span_observer_test.go,span_context_test.go}`, 신규 `span_observer_benchmark_test.go`;
  `internal/adapter/outbound/issueopsrecord/{observer.go,observer_test.go}`.
  대안 승인 시 같은 owner가 `store.go`도 수정한다.
- I9 신규 파일: `internal/adapter/trace/{host_usage.go,host_usage_test.go}`,
  `internal/domain/trace/{host_usage.go,host_usage_test.go,trace_context.go,
  trace_context_test.go}`, `internal/contract/trace/host_usage.go`.
- **I1 완료 후 직렬 수정**:
  `internal/adapter/trace/{decode.go,input.go,analyze_test.go}`,
  `internal/domain/trace/{observation.go,analysis.go,analysis_test.go}`,
  `internal/application/trace/{service.go,service_test.go}`,
  `internal/contract/trace/types.go`.
  I1의 incomplete/loss 진단을 I9 usage coverage로 전달한다.
- I9 CLI 연결은 `cmd/issueops/basiccli/{trace.go,trace_guard_cli_test.go}`;
  correlation observer 연결은 I8 이후 `issueopsrecord/observer.go`에서 한다.
  I10이 요청 context 전달을 소유한다. catalog/schema/golden/architecture inventory
  갱신은 reducer가 I4/I5 통합 owner에게 배정하며 병렬 수정하지 않는다.
  CLI/observer context wiring은 `cmd/issueops/issueopsapp/issueops_record_store_wiring.go`
  에서 통합 owner가 직렬 처리한다.

## Acceptance criteria와 실행 검증

기존 `TestWithSpanReportsSuccessAndFailureAfterReleasingLock`,
`TestWithSpanCancelsLocalWaiter`, `TestWithSpanCancelsSQLiteWaiter`,
`TestJSONLineObserverEmitsOnlyActionableRedactedSpanEvents`,
`TestTraceAnalyzeInvalidJSONAndJSONLFallback`,
`TestTraceAnalyzeRedactsFailureCauseEvidence`를 유지한다.

신규 테스트는 `TestSpanPhaseBoundaries`, `TestSpanCommitCoverage`,
`TestSpanObserverReentersAfterRelease`, `TestHostUsageAuthorityAndDedup`,
`TestHostUsageUnknownZeroAndReset`, `TestTraceparentValidationAndIsolation`이다.
앞의 세 테스트는 `sqlstore/span_observer_test.go`, usage 둘은
`trace/host_usage_test.go`, context는 `domain/trace/trace_context_test.go`에 둔다.
phase 수치는 fake clock으로 검증하고 contention은 channel signal을 사용한다.
고정 sleep이나 observer duration을 total에 넣는 assertion은 금지한다.
신규 fixtures는 `internal/adapter/trace/testdata/host-usage/` 아래
`claude-cumulative-reset.jsonl`, `codex-turn-duplicate.jsonl`,
`omo-final-authority.jsonl`, `unknown-zero-redaction.jsonl`,
`truncated-mixed.jsonl`이며 synthetic metadata만 담는다.

```sh
go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -run 'Test.*Span|TestStoreScopes' -count=1
go test ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace ./cmd/issueops/basiccli -count=1
go test -race ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord ./internal/domain/trace -count=1
go test ./internal/adapter/outbound/sqlstore -run '^$' -bench '^BenchmarkSpanObserverOverhead$' -benchmem -count=5
./bin/issueops trace analyze --input internal/adapter/trace/testdata/host-usage/omo-final-authority.jsonl --input-format omo-json --json
```

benchmark은 observer nil/no-op/JSON actionable 경로를 비교하며 ns/op와 allocs/op를
기록한다. 개선율이나 허용 overhead 수치는 측정 전에 만들지 않는다.
위 명령은 후속 구현 검증용이며 이 설계 작업에서는 실행하지 않았다.

## Assumptions/defaults와 Unresolved questions

I8의 autocommit commit=unknown이 충분한지, 명시적 단일-row transaction 대안을
채택할지는 reducer가 수용 기준과 overhead 증거로 결정한다. Claude 설치 schema와
연구의 최신 0.3.287 주장을 혼동하지 않는다. Codex의 정확한 export schema,
resume turn identity, Omo stable message identity가 확인되지 않은 경우 해당
fixture 검증 전에는 dedup 완전성을 주장하지 않는다. 구현은 이 불확실성을
coverage/warnings로 보존해야 한다.
