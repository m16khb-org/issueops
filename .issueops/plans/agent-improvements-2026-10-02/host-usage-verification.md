# I9 host usage 입력 독립 검증

검증 대상은 `implementation-host-usage.md`가 설명하는 host usage 입력 구현이다.
`contract.md` §4 "I1 / I9: trace와 usage"의 I9 항목을 기준으로 삼았고, HTTP/TRACEPARENT와
`issueopsrecord` observer 상관관계는 producer가 범위 밖이라고 밝혔으므로 이 보고서도
그 부분은 판정하지 않는다. production 파일, 테스트, 설정은 수정하지 않았다.

## 결론

host usage 입력 자체는 계약을 만족한다. 중복 합산, cumulative reset, 안정 ID 없는 turn,
unknown과 measured zero 구분, 손상·초과 입력, 원본 ID와 payload 비노출, state-key fallback
금지, 기본 `issueops` 모드 유지를 모두 CLI로 직접 재현했고 producer 주장과 어긋나는 결과는
없었다. 계약 문구를 벗어나지는 않지만 reviewer가 알아야 할 틈이 세 가지 있고,
통합 단계(Z)가 처리해야 할 미결이 두 가지 있다.

| 구분 | 수 | 항목 |
|---|---|---|
| pass | 17 | 아래 "계약별 판정" 표 참조 |
| 틈(계약 문구 안, 보수적 처리 아님) | 3 | 라벨 원문 통과, `is_error` 문자열, 빈 host 입력의 warning 코드 부재 |
| 통합 미결(이 작업 범위 밖) | 2 | 응답 골든에 `complete`/`usage` 없음, architecture inventory 실패 |
| fail | 0 | |

## 재실행한 명령

모두 `/Users/habin/workspace/issueops`에서 실행했다. 임시 바이너리와 fixture는
`/tmp/i9verify`에 두고 검증 뒤 삭제했다(`rm -rf` 후 `ls`가 "No such file or directory").

```text
$ go test ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace ./cmd/issueops/basiccli -count=1
ok  	issueops/internal/adapter/trace	0.355s
ok  	issueops/internal/domain/trace	0.199s
ok  	issueops/internal/application/trace	0.359s
ok  	issueops/cmd/issueops/basiccli	15.703s
EXIT=0

$ go test -race ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace -count=1
ok (adapter 1.966s, domain 1.381s, application 1.366s)

$ go vet (위 4개 패키지)            -> 출력 없음
$ gofmt -l (trace 4개 패키지 + basiccli/trace*.go) -> 출력 없음
$ go build -o /tmp/i9verify/issueops ./cmd/issueops -> exit 0
```

CLI는 `ISSUEOPS_STATE_DIR=/tmp/i9verify/state /tmp/i9verify/issueops trace analyze ...`
형태로 33회 실행했다. 아래 표의 "근거" 열은 그 stdout JSON에서 확인한 값이다.

## 계약별 판정

| # | 계약 문구 (contract.md §4 I9) | 판정 | 근거 |
|---|---|---|---|
| 1 | `--input-format=issueops\|claude-stream\|codex-exec\|omo-json`, 기본 issueops | pass | `basiccli/trace.go:97` 기본값 `InputFormatIssueOps`. `--input-format nope` → exit 1, `ok=false`, `warnings=["unsupported_input_format"]`, stderr `unsupported trace input format "nope" (want issueops, claude-stream, codex-exec or omo-json)` |
| 2 | host 입력은 file/stdin만 | pass | `adapter/trace/input.go:17-19,34-62` `loadHost`는 `-`와 파일만 연다. `--input some-state-key --input-format omo-json` → exit 1, `warnings=["trace_read_error"]`, stderr `open some-state-key: no such file or directory`. ReadState 경로로 가지 않는다 |
| 3 | 16MiB/file cap, 초과 시 complete=false | pass | 17.0MiB 파일(유효 줄 2개 사이에 filler): file과 stdin 모두 `complete=false`, `warnings=["input_truncated"]`, `usage.warnings=["input_truncated"]`, 앞쪽 sample 1개 보존·뒤쪽 sample 소실, `coverage=partial`. 개행 없는 17MiB 한 줄: `samples=[]`, `coverage=unknown`, exit 0. `input.go:54-61`은 cap+1까지만 읽고 마지막 개행에서 자른다 |
| 4 | 1MiB/line cap | pass | 1MiB+1 바이트 줄 뒤에 유효 줄: `complete=false`, `warnings=["jsonl_line_too_long"]`, 뒤 sample(2/2) 보존. `host_usage.go:64-67`은 초과 줄만 건너뛴다 |
| 5 | 원문 parser 오류를 출력하지 않는다 | pass | `null`, `[1,2]`, 한 줄 두 JSON, 잘린 객체, CRLF, 빈 줄을 섞은 입력: `warnings=["invalid_jsonl_line"]`만 나오고 stderr 비어 있음. CRLF 줄의 `session`은 정상 인식(scope_id 채워짐). `decodeHostEvent`(`host_usage.go:86-96`)는 오류 문자열을 버린다 |
| 6 | `UsageReport`/`UsageSample` 필드, snake_case, nil은 null | pass | `contract/trace/host_usage.go:33-58` 태그. 모든 응답에서 `cache_write_tokens: null`, `cost_usd: null` 등 null 직렬화 확인 |
| 7 | ID는 SHA-256 digest로만 | pass | 모든 응답의 `scope_id`/`session_id`/`turn_id`/`message_id`가 64자 hex 또는 빈 문자열. 원본 `sess-claude-A`, `thr-codex-1`, `resp-omo-1`, `th-1`, `ses-1`은 어떤 출력에도 없다. `domain/trace/host_usage.go:32-38` `sha256(kind NUL id)` |
| 8 | 숫자는 finite/nonnegative, 정수만 | pass | omo usage `input:-1, output:1.5, cacheRead:"7", cacheWrite:1e3, cost.total:"x"` → 전부 null + `usage_invalid_metric`. `9007199254740993`은 그대로 보존, `9223372036854775808`(int64 초과)은 null, `cost.total:-0.5`는 null. `host_usage.go:137-171` |
| 9 | Claude `result.modelUsage`는 마지막 cumulative snapshot만, main-loop usage 합산 금지 | pass | fixture `claude-cumulative-reset.jsonl`: result 3개(100→250→20) → sample 2개 `epoch 0: 250/120/30/0 cost 0.03`, `epoch 1: 20/5/0/0 cost 0.002`. `assistant.message.usage`(5/7)와 `result.usage`(3/4)는 어디에도 더해지지 않음 |
| 10 | reset은 경고하고 추정 delta를 만들지 않는다, reset마다 epoch 증가 | pass | 같은 fixture: `usage.warnings=["usage_cumulative_reset"]`, `coverage=partial`, 250−20 같은 차이 값은 없음. `domain/trace/host_usage.go:60-66` |
| 11 | 동일 snapshot 반복은 한 번만 | pass | 같은 session·같은 숫자 result 2개 → sample 1개, `coverage=complete` |
| 12 | 안정 ID 없으면 합치지 않고 coverage=unknown, 합계 없음 | pass | Codex `turn_id` 없는 동일 숫자 turn 2개 → sample 2개 그대로, `coverage=unknown`, `warnings=[]`(fixture) / `["usage_turn_failed"]`(turn.failed 추가 시). Claude `session_id` 없는 result 2개 → sample 2개, `scope_id=""`, `coverage=unknown`. 응답 어디에도 total 필드 없음(`UsageReport`에 합계 필드 자체가 없다) |
| 13 | Codex `turn_id` 있을 때만 dedup, 충돌 경고 | pass | `t1` 동일 2회 → 1 sample + `usage_duplicate_event`; `t2` 10→99 → 첫 값(7/1) 유지 + `usage_conflicting_duplicate`, `coverage=partial` |
| 14 | Omo `message_end.message.usage`가 final authority, update/turn_end/agent_end 재합산 금지 | pass | `message_update`(120/30), `turn_end`, `agent_end`에 같은 usage가 있어도 sample은 `message_end` 1개(120/30/0/7, cost 0.3). 동일 `responseId` `message_end` 2회 → 1 sample + `usage_duplicate_event`. `role:"user"` message_end(999/999)는 무시 |
| 15 | 누락과 crash zero는 unknown, 성공 final의 0만 measured | pass | Omo `stopReason:"aborted"` 0들 → 전부 null, `finality=partial`. Omo 성공 `cacheRead:0` → `0`(measured). Claude `is_error:true` result의 0들 → 전부 null, `finality=partial`. Codex `turn.completed` `input_tokens:0` → `0`. Codex `turn.completed`에 usage 없음 → `usage_missing`. `host_usage.go:173-194` `clearZeros`는 non-final에만 적용 |
| 16 | 모르는 event/field는 계량하지 않고 unknown coverage | pass | `{"type":"mystery","usage":{...}}` → `usage_unknown_event`, sample에 반영 안 됨, `coverage=partial`(다른 sample이 있을 때). Claude 파일을 `omo-json`으로 읽으면 `samples=[]`, `coverage=unknown`, `usage_unknown_event` — 형식 오지정이 잘못된 숫자를 만들지 않는다 |
| 17 | 원본 ID는 256 bytes까지, 초과는 unknown | pass | 257바이트 `id` → `session_id=""`, `message_id=""`, `scope_id=""`, `coverage=unknown`, `usage_identity_too_long`. 256바이트 → 정상 digest, `coverage=complete`. `host_usage.go:15,122-128` |
| 18 | cost는 host pricing basis 있을 때만, 없으면 null | pass | Claude `costUSD` → `host_reported_estimate`; Codex → 항상 `cost_usd:null, cost_basis:"unknown"`; Omo `cost.total:0` → null + `unknown`(`host_usage.go:347-353`) |
| 19 | prompt/text/thinking/tool payload 비노출 | pass | fixture와 추가 입력의 `SECRET-PROMPT-TEXT`, `SECRET-OUTPUT`, `SECRET-DELTA`, `SECRET-ANSWER`, `TOKEN=sk-SECRET-VALUE`, `PROMPT_SECRET`, `TEXT_SECRET`은 stdout·stderr 어디에도 없음. 단, 아래 "틈 1" 참조 |
| 20 | 기본 `issueops` 모드와 I1 동작 유지 | pass | 기본 모드 응답에 `usage` 키 없음(`types.go:24` `omitempty`). 70,000자 줄이 낀 issueops JSONL → `complete=false`, `warnings=["jsonl_scan_error","no_supported_trace_findings"]`, `--input-format issueops`를 명시해도 동일. `analysis.go:26` `no_supported_trace_findings`는 `input.Usage == nil`일 때만 붙으므로 host 모드에서는 나오지 않는다 |
| 21 | text 출력 | pass | `usage: 2 sample(s), coverage=unknown` / `usage warning: usage_turn_failed`. text 출력에는 sample 라벨이 찍히지 않는다 |

## 틈 (계약 문구는 지키지만 reviewer가 알아야 할 동작)

### 틈 1. `model`/`provider`/`version` 라벨은 원문이 그대로 통과한다

`host_usage.go:130-135` `label`과 `claude()`의 `claude_code_version` 처리는 길이만 256바이트로
제한하고 문자 제약이 없다. 입력에 `"modelUsage":{"MODEL_SECRET_sk-xyz\nline2":{"provider":"PROVIDER_SECRET_sk-123",...}}`,
`"claude_code_version":"VERSION_SECRET_sk-abc"`를 넣으면 JSON 응답에

```text
"version": "VERSION_SECRET_sk-abc",
"provider": "PROVIDER_SECRET_sk-123",
"model": "MODEL_SECRET_sk-xyz\nline2",
```

가 그대로 나온다. 계약은 "ID는 digest", "prompt/text/thinking/tool payload는 버린다"라고만
정하고 model/provider/version 라벨의 처리를 규정하지 않으므로 위반은 아니다. 다만 (a) export를
조작하면 세 라벨을 통해 임의 문자열 768바이트를 응답에 실을 수 있고, (b) 기존 trace 도메인은
자유 문자열에 `policy.RedactFreeform`을 적용하는데(`analysis.go:62` step 이름) 이 라벨에는
적용하지 않는다. 실제 host가 내보내는 값은 모델 이름과 버전 문자열이므로 운영 위험은 낮다.
Z 또는 후속 작업에서 라벨 allowlist(예: `[A-Za-z0-9._:/-]`)나 redaction 적용 여부를 결정하면 된다.

### 틈 2. Claude `is_error`는 JSON bool `true`만 인식한다

`host_usage.go:233` `event["is_error"] != true`는 타입까지 비교하므로 `"is_error":"true"`(문자열)는
성공으로 처리된다. `{"type":"result","subtype":"success","is_error":"true",...,"inputTokens":0,"outputTokens":0}`
→ `finality=final`, `input_tokens: 0, output_tokens: 0`(measured), `coverage=complete`.
Claude SDK는 bool을 내보내므로 정상 export에서는 재현되지 않고, `subtype == "success"` 조건이
따로 있어 계약의 "명시적으로 성공한 final"에는 부합한다. 보수적으로 가려면 `is_error`가 bool이
아닐 때 finality를 unknown으로 내리는 선택지가 있다.

### 틈 3. 빈 host 입력은 exit 1이지만 warning 코드가 없다

0바이트 파일을 `--input-format omo-json`으로 읽으면 `ok=false`, `complete=false`, `warnings` 키 없음,
stderr `trace analyze input is empty`(`service.go:45-47`). 다른 실패 경로(`trace_read_error`,
`unsupported_input_format`)는 코드를 남기므로 일관성이 떨어진다. 동작 자체는 안전하다.

### 참고. crash result가 직전 성공 snapshot을 대체한다

같은 session에서 성공 result(100/10) 뒤에 `is_error:true` result(0/0)가 오면 sample은 1개가 되고
그 값은 전부 null·`finality=partial`이다. 성공 snapshot의 100/10은 응답에서 사라지며 reset 경고도
없다(`domain/trace/host_usage.go:60,164-171`은 nil을 감소로 보지 않는다). 계약의 "마지막
cumulative snapshot만 보존"과 "crash zero는 unknown"을 문자 그대로 따른 결과이고, coverage는
`partial`로 내려가므로 잘못된 숫자가 생기지는 않는다. 정보 손실이 의도인지는 contract owner가
판단할 몫이라 판정에서 제외했다.

## 통합 미결 (이 작업 범위 밖, Z-integration)

1. **응답 골든에 `complete`와 `usage`가 없다.** `cmd/issueops/testdata/response_contracts.golden.json:6309`
   `trace_analyze` 항목은 `finding_count, findings, input, input_source, kind, ok, trace_types`만 가진다.
   `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden`은 현재 실패하지만, 첫 mismatch는
   `$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count`(I2 영역)이다. golden 비교가
   첫 불일치에서 멈추므로 `complete`(I1)와 `usage`/`input_format`(I9) 반영 여부는 이 실행으로는
   드러나지 않았다. `go test ./cmd/issueops/contractgolden -run Golden`은 통과했다.
2. **`internal/architecture` `TestDDDResponsibilityInventoryMatchesSource` 실패를 재현했다.** 출력에서
   `trace`를 grep하면 한 줄도 나오지 않으므로 신규 trace 파일 세 개가 원인에 포함되는지는 이 실행으로
   확정하지 못했다. producer가 밝힌 대로 다른 작업자의 미등록 파일과 함께 Z가 한 번에 갱신하면 된다.

## 범위 밖으로 남긴 것

- TRACEPARENT/`TraceContext`/`ParseTraceparent`, HTTP header 처리, `issueopsrecord` observer 상관관계:
  producer가 미구현이라고 명시했고 이번 DTO가 요구하지 않으므로 검증하지 않았다. 통과로 적지 않는다.
- MCP `trace_analyze` schema의 `input_format`/`usage` 노출, API 문서 게이트: Z 몫이며 확인하지 않았다.
- effort 조사는 지시대로 하지 않았다.
