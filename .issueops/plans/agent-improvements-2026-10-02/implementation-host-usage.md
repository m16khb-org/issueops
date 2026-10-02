# I9 host usage 입력 구현 증거

## 결과

I9 중 host usage 입력 부분을 구현했다. `issueops trace analyze`가
`--input-format=issueops|claude-stream|codex-exec|omo-json`을 받는다.
기본값은 `issueops`이며 I1의 완전성 동작(Scanner 64KiB cap, `complete`,
경고 코드)은 바꾸지 않았다. HTTP/TRACEPARENT 연결은 이 작업에 포함하지 않았다.

host 형식은 명시한 파일 또는 `-`(stdin)만 읽는다. state-key fallback은 없다.
파일은 16MiB, 한 줄은 1MiB가 상한이다. 한도를 넘으면 `complete=false`로 표시하고
나머지 입력은 계속 처리한다. 모든 host 응답은 새 `usage` 필드(`UsageReport`)를 담는다.
기본 모드 응답에는 `usage` 키가 생기지 않는다.

## 변경 파일

| 파일 | 내용 |
|---|---|
| `internal/contract/trace/host_usage.go` (신규) | `UsageReport`, `UsageSample`(nullable 지표), 입력 형식·cap·어휘 상수 |
| `internal/contract/trace/types.go` | `TraceAnalyzeRequest.InputFormat`, `TraceAnalyzeResult.Usage` (`omitempty`) |
| `internal/domain/trace/host_usage.go` (신규) | `UsageObservation`, `ReduceUsage`(dedup·cumulative·epoch·coverage), `NoteUsageLoss`, `UsageDigest` |
| `internal/domain/trace/observation.go`, `analysis.go` | `Input.Usage` 추가, usage 입력에는 `no_supported_trace_findings` 경고를 붙이지 않음 |
| `internal/adapter/trace/host_usage.go` (신규) | 세 host의 JSONL을 allowlist 필드만 뽑아 관측으로 변환 |
| `internal/adapter/trace/input.go`, `decode.go` | `Load(input, format)`가 `truncated`를 돌려주고 `Decode(body, format)`가 형식별로 분기, `Source.Stdin` 주입 지점 |
| `internal/application/trace/service.go` | 형식 검증, 잘림 처리, usage 전달 |
| `cmd/issueops/basiccli/trace.go` | `--input-format` 플래그와 text 출력의 usage 요약 |
| 테스트 | `internal/adapter/trace/host_usage_test.go`, `testdata/host-usage/*.jsonl` 4개, `internal/domain/trace/host_usage_test.go`, `internal/application/trace/service_test.go` 갱신, `cmd/issueops/basiccli/trace_host_usage_cli_test.go` |

## 적용한 의미 규칙

| host | authority | temporality | 처리 |
|---|---|---|---|
| Claude `result.modelUsage` | cumulative snapshot | `cumulative` | (session, model, provider)별 마지막 snapshot만 보존. 지표가 하나라도 줄어들거나 cost가 줄면 reset으로 보고 epoch를 올리며 `usage_cumulative_reset` 경고. assistant/stream의 `usage`는 합산하지 않는다 |
| Codex `turn.completed.usage` | delta | `delta` | `turn_id`가 있을 때만 dedup. 같은 id·같은 값은 한 번, 같은 id·다른 값은 첫 값 유지와 `usage_conflicting_duplicate` |
| Omo `message_end.message.usage` | final | `delta` | `message_update`, `turn_end`, `agent_end`가 반복한 값은 세지 않는다. `responseId`(없으면 `id`)로 dedup |

- 지표는 10진 정수 토큰만 받는다. 음수, 소수, 지수 표기(`1e3`), 문자열, int64 초과는
  `null`과 `usage_invalid_metric` 경고다. `9007199254740993`처럼 float64로는 틀리는
  정수도 정확히 보존한다(`json.Number` + `ParseInt`).
- 필드 부재와 `null`은 `null`(unknown)이다. 0은 성공한 final에서만 measured zero다.
  실패·중단·finality 불명인 항목의 0은 `null`로 낮춘다(Claude 비성공 result,
  Omo `error`/`aborted`/알 수 없는 stopReason).
- 원본 ID는 256바이트까지만 받고 `sha256(kind NUL id)` hex로 바꾼다. kind는
  session/turn/message/scope다. 초과하면 빈 값과 `usage_identity_too_long`이다.
- identity가 없는 sample은 합치지 않고 그대로 보존한다. 이 경우 report `coverage`는
  `unknown`이다. 같은 숫자의 서로 다른 turn이 중복으로 오인되지 않는다.
- `coverage`: sample이 없거나 identity 없는 sample이 있으면 `unknown`, 비-final
  sample·경고(parse 손실, reset, 충돌, 알 수 없는 usage 이벤트, 잘림 등)가 있으면
  `partial`, 그 외 `complete`. 무해한 `usage_duplicate_event`만 coverage를 낮추지 않는다.
- cost는 Claude `costUSD`와 Omo `cost.total`(양수일 때만)을 `host_reported_estimate`로
  싣는다. Codex는 cost가 없으므로 항상 `null`/`unknown`이다. 가격표·합계·청구액은
  만들지 않는다.
- prompt/text/thinking/tool payload, endpoint, 원문 parser 오류는 어디에도 싣지 않는다.
  경고는 고정 코드뿐이다.

## 계약과 관측 스키마가 다른 지점 (보수적 unknown으로 처리)

1. **Codex turn identity**: 설치된 Codex 0.160.0 벤더 바이너리의 문자열에서 `exec`
   이벤트는 `thread.started`, `turn.started`, `turn.completed`, `turn.failed`, `item.*`이고
   usage 필드는 `input_tokens`, `cached_input_tokens`, `output_tokens`,
   `reasoning_output_tokens`다. `turn.completed`에 turn id 필드는 확인되지 않았다.
   그래서 실제 `codex exec --json` export는 dedup 불가이며 `coverage=unknown`이다
   (`turn_id`가 있는 export만 dedup). `reasoning_output_tokens`는 output의 부분집합이라
   DTO 필드가 없어 매핑하지 않았다.
2. **Omo 버전**: 세션 헤더의 `version`은 세션 포맷 버전(3)이지 host 버전이 아니다.
   Omo `Version`은 빈 문자열이다. Codex도 스트림에 버전이 없어 빈 문자열이다.
   Claude는 `system/init`의 `claude_code_version`이 하나뿐일 때만 채운다.
3. **Epoch/ScopeID**: `Epoch`는 DTO 대로 string(`"0"`, `"1"`...)이다. `ScopeID`는
   export 안의 누적 계열 단위인 `sha256("scope" NUL host NUL session)`이며 session이
   없으면 빈 값이다. delta/final 계열의 epoch는 항상 `"0"`이다.
4. **Omo 비용 0**: 설정 가격이 0인 모델과 가격 불명을 구분할 수 없어 `cost.total=0`은
   `null`로 둔다.
5. **Claude 설치 SDK**: `modelUsage`의 누적·crash zero 설명은 설치된 SDK 타입 정의
   (`sdk.d.ts`)의 `SDKResultMessage` 주석으로 확인했다. 최신 버전과의 차이는 조사하지 않았다.

## RED / GREEN

- 구현 전 RED: adapter와 application 테스트는 `Decode(body, format)`/`Load(..., format)`
  시그니처, `Source.Stdin`, `Input.Usage`가 없어 빌드에서 실패했다
  (`too many arguments in call to Source{}.Decode`, `unknown field Stdin`,
  `*fakeEffects does not implement Effects`). 솔직한 기록: 도메인 reducer는 RED 실행
  전에 작성했으므로 도메인 패키지의 RED는 직접 관측하지 못했다.
- GREEN: 아래 명령이 모두 exit 0이다.

```sh
PATH=/tmp/issueops-ten-improvements-01a0fa31/venv/bin:$PATH \
  go test ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace ./cmd/issueops/basiccli -count=1
```

```text
ok  issueops/internal/adapter/trace       0.410s
ok  issueops/internal/domain/trace        0.256s
ok  issueops/internal/application/trace   0.096s
ok  issueops/cmd/issueops/basiccli       14.646s
```

- 같은 세 trace 패키지의 `go test -race`: 통과 (adapter 1.954s, domain 1.293s, application 1.287s).
- `gofmt -l`(변경 Go 파일): 출력 없음. `go vet`(5개 패키지): 출력 없음.
  `git diff --check`: 출력 없음. `go build ./...`: exit 0.
  LSP: domain/trace 진단 0건. adapter/trace에는 `new(expr)` 안내 힌트 몇 건만 남았다
  (테스트 helper `i64`/`f64`; 동작과 무관, 기존 `analyze_test.go`의 `slices.Contains` 힌트는 그대로).
- 테스트에는 sleep/polling이 없다. 16MiB cap 테스트만 16MiB 임시 파일을 쓴다(0.04s).

### 테스트 범위

세 형식의 정상 경로, 중복·충돌·cumulative reset·감소·cost 감소, 정체 없는 입력의 비병합,
부재/0/`null`/음수/소수/지수/문자열/오버플로/정확한 int64 보존, 256바이트 ID 경계,
원문·ID 비노출(여러 fixture를 직렬화해 문자열 검사), 손상 줄 뒤 계속 처리,
1MiB 줄 초과/정확히 1MiB, 16MiB 파일 초과/정확히 16MiB, stdin·파일 로드,
state-key fallback 금지(ReadState 호출 0회) 및 기본 모드 fallback 유지,
기본 모드의 Scanner cap 유지와 `usage` 부재, 미지원 형식(adapter·service·CLI).

## 실제 CLI 실행

`go build -o <tmp>/issueops ./cmd/issueops`로 임시 바이너리를 만들고
`ISSUEOPS_STATE_DIR=<tmp>/state`에서 12개 케이스를 실행했다. 모두 stdout JSON을 파싱해
단언했고 모두 통과했다. 임시 폴더는 삭제했다.

| 케이스 | 입력 | 결과 |
|---|---|---|
| claude 파일 | `--input <file> --input-format claude-stream` | exit 0, sample 2개(입력 250/20, epoch 0/1), coverage `partial`, `usage_cumulative_reset` |
| codex stdin | `--input - --input-format codex-exec` (파일을 stdin으로) | `input_source=stdin`, 동일 숫자 두 turn이 2 sample로 보존, coverage `unknown`, cache_write `null`, cache_read 40 |
| omo stdin | `--input - --input-format omo-json` | sample 1개(120/30, cache_read 0 measured, cache_write 7, cost 0.3 estimate), coverage `complete` |
| unknown vs zero | omo fixture | cache 지표 `null`, 중단 message는 전부 `null`·finality `partial`, 출력에 `SECRET`/`sk-`/원본 ID 없음 |
| 미지원 형식 | `--input-format nope` | exit 1, `ok=false`, 경고 `unsupported_input_format` |
| state-key 금지 | host 형식 + `some-state-key` | exit 1, `ok=false`, 경고 `trace_read_error` |
| 기본 모드 | summary JSON | exit 0, `usage` 키 없음, finding 1 |
| 1MiB 초과 줄 | 1MiB+줄을 낀 omo 입력 | exit 0, `complete=false`, `jsonl_line_too_long`, 앞뒤 sample 2개 유지 |
| 손상 줄 | 가운데 손상 줄 | `complete=false`, `invalid_jsonl_line`, sample 2개, 원문 비노출 |
| 16MiB 초과 stdin | 유효 줄 + 16MiB 쓰레기 | `complete=false`, `input_truncated`, sample 1개, coverage `partial` |
| 기본 Scanner cap | 70,000자 줄이 낀 issueops JSONL | `complete=false`, `jsonl_scan_error`, `usage` 없음 (I1 유지) |
| text 출력 | `--input-format codex-exec` | `usage: 2 sample(s), coverage=unknown` |

## 남은 통합 경계 (이 작업 범위 밖)

1. `internal/architecture` 인벤토리: `TestDDDResponsibilityInventoryMatchesSource`가
   현재 실패한다. 원인은 이 작업의 신규 production 파일 세 개
   (`adapter/trace/host_usage.go`, `domain/trace/host_usage.go`,
   `contract/trace/host_usage.go`)뿐 아니라 다른 작업자의 미등록 신규 파일
   (`contract/authority`, `contract/mcpservice`, `sqlstore/record_guard.go` 등)도 있다.
   인벤토리 갱신은 Z-integration이 한 번에 처리해야 한다.
2. 응답 계약 golden(`cmd/issueops/testdata/response_contracts.golden.json` 등):
   `trace_analyze` 응답의 `complete`(I1)와 이번 `usage`(`omitempty`), 요청의
   `input_format`이 반영돼야 한다. 이 작업에서는 수정하지 않았다.
3. MCP `trace_analyze`의 tool schema/catalog: `input_format` 입력과 `usage` 출력 노출이
   필요하다. 서비스는 이미 `TraceAnalyzeRequest.InputFormat`을 받으므로 schema만 추가하면 된다.
   MCP는 stdin을 쓰지 않으므로 파일 경로 입력만 의미가 있다.
4. API 문서 게이트: DTO가 바뀌었으므로 `api-doc static-check`/`api_doc_review`를
   통합 단계에서 staged diff 기준으로 실행해야 한다.
5. TRACEPARENT/HTTP 연결(`ParseTraceparent`, `TraceContext`)과 `issueopsrecord` observer
   상관관계는 이 작업에 포함하지 않았다. 이번 DTO는 trace context 필드를 요구하지 않는다.
6. 기존 I1 보고서의 4번 항목(파일 읽기 실패 뒤 state-key 시도로 오류 원인이 바뀌는
   현상)은 기본 모드 동작이라 그대로 두었다. host 형식에서는 이 fallback이 없다.
