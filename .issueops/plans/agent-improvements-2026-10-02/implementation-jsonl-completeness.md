# I1 JSONL completeness 구현 증거

## 결과와 실행 설정

I1에 할당된 구현, 결정적 회귀 테스트, 지정 패키지 검증과 실제 CLI 확인을
완료했다. `contract.md:311–324`의 경계를 따른다. 기본 Scanner cap을 늘리거나
큰 행 이후 입력을 복구했다고 주장하지 않는다. 중단·손상을 명시적으로 표시한다.

실제 세션 환경은 `PI_MODEL=gpt-6.1-sol`, `PI_REASONING_LEVEL=high`였다.
설계 계약 머리말의 `gpt-6-astra`는 이 구현 세션의 관측값이 아니다.
환경이나 모델 설정을 변경하지 않았다.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/adapter/trace/decode.go` | 손상된 nonblank 행과 `scanner.Err()`를 기록한다. 행 손상 경고는 입력당 한 번만 기록한다. 원문 parser/scanner 오류를 반환하지 않는다. |
| `internal/adapter/trace/analyze_test.go` | 정상 JSON/JSONL, blank 행, 처음·중간·마지막 손상 행, finding 없는 손상, 단일 JSON 손상, 처음·중간 큰 행, 손상과 scan 오류 조합, 반복 손상의 12개 fixture를 추가했다. sentinel, 경고, 직렬화된 completeness와 원문 미노출을 검사한다. |
| `internal/domain/trace/observation.go` | `Input.Incomplete`, `Input.Warnings`, `Analysis.Incomplete`를 추가하고 원문 오류를 담던 `JSONError`를 제거했다. |
| `internal/domain/trace/analysis.go` | finding 유무와 무관하게 입력 경고와 불완전성을 보존한다. finding이 없으면 기존 `no_supported_trace_findings`도 제공한다. |
| `internal/domain/trace/analysis_test.go` | 새 입력 모델에 맞게 기존 검사를 강화하고 finding 유무에 따른 경고 보존 회귀를 추가했다. |
| `internal/application/trace/service.go` | `complete=!analysis.Incomplete`를 전달한다. Load 실패는 `ok=false`, `complete=false`, `trace_read_error`와 wrapped error로 반환하며 부분 body를 decode하지 않는다. |
| `internal/application/trace/service_test.go` | 분석 실행 성공과 입력 완전성을 분리하고 Load 실패의 error chain·decode 미실행을 검사한다. |
| `internal/contract/trace/types.go` | 항상 직렬화되는 `complete` boolean을 추가했다. |

기존 `invalid_json:` prefix는 단일 JSON 손상에서
`invalid_json:invalid_jsonl_line`이라는 입력과 무관한 코드로 유지했다.
정상 JSONL의 단일 문서 parse 실패는 경고가 아니다. `ok=true`는 분석 실행
성공을 뜻하며 입력 완전성을 보장하지 않는다. 기존 finding 판정·redaction은 유지했다.

## RED / GREEN

검증 명령의 PATH에는 parent가 준비한 Python venv의 `bin`을 앞에 붙였다.
아래 `$PYTHON_VENV`, `$TMP`는 각각 그 격리 환경과 이번 `mktemp` 디렉터리를 뜻한다.
새 dependency를 설치하지 않았다.

```sh
PATH="$PYTHON_VENV/bin:$PATH" go test \
  ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace \
  -run '^(TestTraceAnalyzeJSONLCompleteness|TestAnalyzePreservesIncompleteWarningsWithOrWithoutFindings|TestAnalyzeReportsCompletenessIndependentlyOfExecution|TestAnalyzeReportsReadFailureWithoutDecodingPartialBody)$' \
  -count=1 -v
```

- RED `mon_T7C3TBX1EC9EV542`: **exit 1**. DTO 필드를 먼저 추가한 상태에서
  실제 동작 assertion으로 실패했다. 정상 입력의 completeness 전달, 손상·큰 행의
  경고, domain의 incompleteness 전달과 읽기 실패 reason code가 모두 실패했다.
- 첫 RED `mon_XQAJKM5A4ACARX91`도 exit 1이었다. 정상 summary fixture의
  기대 pattern을 기존 `summary.go`에 맞게 바로잡고 production 수정 전에 다시
  RED를 실행했다. 기존 테스트를 약화하지 않았다.
- 수정된 RED의 전체 실패 출력은 monitor의 원본 session `bash_5`에서
  `bash_output`으로 읽어 이 작업자의 tool transcript에 생략 없이 보존했다.
  첫 RED의 원본 session `bash_4`도 전체 출력으로 읽었다.

```sh
PATH="$PYTHON_VENV/bin:$PATH" go test \
  ./internal/adapter/trace ./internal/domain/trace ./internal/application/trace \
  -count=1
```

GREEN `mon_15TGB6TDEN3AFYD6`: **exit 0**.

```text
ok issueops/internal/adapter/trace     0.473s
ok issueops/internal/domain/trace      0.379s
ok issueops/internal/application/trace 0.379s
```

손상 행 뒤 finding을 복구하던 기존 fallback 테스트도 이 실행에서 통과했다.
새 테스트에는 sleep, polling, wall-clock assertion이 없다.

## 정적 검사와 실제 사용

- 네 trace package의 LSP 검사: 오류·경고 0건. adapter 테스트의 기존
  `containsString` helper에 `slices.Contains` 단순화 힌트 1건이 있다.
  해당 helper는 수정하지 않았다.
- 위 변경 Go 파일 8개에 대한 `gofmt -l`: **exit 0**, stdout/stderr 없음.
- 같은 파일의 `git diff --check`: **exit 0**, stdout/stderr 없음.
- production 파일 최대 pure LOC는 87이다. 기존 adapter 테스트 파일은 이번
  추가 후 348 pure LOC다. 허용된 파일 밖으로 테스트를 분리하지 않았다.
- diff는 위 Go 파일 8개와 이 보고서로 한정했다. 다른 작업자의 변경은 유지했다.

```sh
PATH="$PYTHON_VENV/bin:$PATH" go build -o "$TMP/issueops" ./cmd/issueops
ISSUEOPS_STATE_DIR="$TMP/state" "$TMP/issueops" trace analyze \
  --input "$TMP/<fixture>" --json
```

빌드 `mon_1KE3VHHBABNGB3JG`: **exit 0**. 다음 표는 빌드한 실제 CLI의
stdout JSON을 parse하고 exit status와 함께 assertion한 결과다.
정상 입력에서 생략되는 `warnings`는 표에서 `[]`로 표시했다.

| fixture | exit | ok | complete | finding_count | warnings |
|---|---:|---|---|---:|---|
| 정상 단일 summary JSON | 0 | true | true | 1 | `[]` |
| 두 sentinel JSONL | 0 | true | true | 2 | `[]` |
| sentinel 사이 손상 행 | 0 | true | false | 2 | `["invalid_jsonl_line"]` |
| sentinel 사이 65,536자 padding을 담은 JSON 행 | 0 | true | false | 1 | `["jsonl_scan_error"]` |
| 손상된 단일 JSON | 0 | true | false | 0 | `["invalid_json:invalid_jsonl_line","no_supported_trace_findings"]` |
| 존재하지 않는 파일 입력 | 1 | false | false | 0 | `["trace_read_error"]` |

큰 행 fixture의 유일한 finding은 `first sentinel failed 1 time(s)`였다.
`last sentinel`은 누락되지만 `complete=false`로 명시했다. 손상 행 fixture에서는
두 sentinel을 모두 복구했다. 손상 입력의 marker는 어느 JSON 응답에도 없었다.
CLI 호출 6개에 대한 assertion 결과는 `CLI_ASSERTIONS=PASS`였다.
binary·fixture·state는 임시 디렉터리에만 두었다.

첫 빌드 `mon_34FZXTXHWY9Y48ZK`는 동시 작업 중인 out-of-scope 파일의 오류로
**exit 1**이었다. 전체 오류는 아래와 같으며 해당 담당자가 수정한 뒤 재빌드했다.
이 작업자는 그 파일을 수정하지 않았다.

```text
# issueops/internal/adapter/preflight
internal/adapter/preflight/helpers.go:12:6: out redeclared in this block
  internal/adapter/preflight/helpers.go:10:18: other declaration of out
internal/adapter/preflight/helpers.go:16:17: invalid append: argument must be a slice; have out (variable of type func(string, ...string) string)
internal/adapter/preflight/helpers.go:19:9: cannot use out (variable of type func(string, ...string) string) as []"issueops/internal/contract/preflight".RemoteInfo value in return statement
```

## 남은 통합 확인

1. `cmd/issueops/testdata/response_contracts.golden.json`의 `trace_analyze` 응답은
   새 `complete` 필드 반영이 필요하다. parent의 통합 golden/schema 담당 범위이며
   이 작업에서는 해당 파일을 수정하거나 전체 golden 통과를 주장하지 않았다.
2. I9 담당자는 `Input.Incomplete/Warnings`와 `Analysis.Incomplete`를 유지하면서
   host-format Load/Decode와 usage 필드를 추가해야 한다. `JSONError`는 더 이상
   입력 계약에 없다.
3. `api-doc static-check --json`과 `api-doc review --json`은 둘 다 exit 0,
   `skipped=true`, `no_api_doc_candidate_files`였다. 기본 검사는 staged diff를
   사용하며 이번 변경은 stage하지 않았으므로 DTO 문서 검증 근거로 삼지 않는다.
   parent의 통합 문서/schema 검증이 남는다.
4. `internal/adapter/trace/input.go`는 파일 읽기 실패 뒤 state-key 읽기를 시도한다.
   실제 missing-file CLI stderr는 원래 파일 오류 대신 invalid state key를
   포함했다. I1의 응답은 실패와 `trace_read_error`를 명시하지만 기존 두 source
   fallback의 원인 세분화는 이 파일의 별도 계약 결정이 필요하다.

전체 저장소 테스트·race·host 설치·MCP host parity의 최종 검증은 수행하지 않았다.
commit, push, 사용자 HOME 설치, config 수정이나 I2 duration 작업은 하지 않았다.
