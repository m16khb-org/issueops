# trace JSONL의 큰 행 뒤 이벤트 누락 재현

관측일: 2026-10-02.
실행 표면: `bin/issueops trace analyze --input - --json`.
현재 조사 바이너리와 HEAD는 [MCP baseline](mcp-baseline.md)의 환경을 따른다.
입력은 메모리에서 생성해 stdin으로 전달했다. 파일·설정·상태를 변경하지 않았다.

## 가설과 코드

`internal/adapter/trace/decode.go:26-38`은 기본 `bufio.Scanner`로 JSONL을 읽고
반복 종료 후 `scanner.Err()`를 확인하지 않는다.
큰 행에서 스캔이 중단되면 뒤의 이벤트가 사라질 가능성이 있다.

## 통제한 입력

아래에서 `padding`만 100자 또는 70,000자로 바꿨다. 각 객체를
`JSON.stringify`해 줄바꿈으로 연결하고 마지막 줄바꿈을 붙였다.

두 행:

```json
{"event":"diagnostic","padding":"<x를 N개 반복>"}
{"event":"step_end","step":"sentinel_failure","ok":false}
```

세 행:

```json
{"event":"step_end","step":"first_failure","ok":false}
{"event":"diagnostic","padding":"<x를 N개 반복>"}
{"event":"step_end","step":"sentinel_failure","ok":false}
```

`<x를 N개 반복>`은 설명용 표기다. 실제 실행은 `'x'.repeat(N)`으로 유효한 JSON
문자열을 생성했다.

## 실제 결과

| 입력 | exit | ok | finding_count | 관측된 실패 |
|---|---:|---|---:|---|
| 두 행, padding 100 | 0 | true | 1 | sentinel_failure |
| 두 행, padding 70000 | 0 | true | 0 | 없음 |
| 세 행, padding 100 | 0 | true | 2 | first_failure, sentinel_failure |
| 세 행, padding 70000 | 0 | true | 1 | first_failure |

모든 실행의 stderr는 비어 있었다. 큰 행이 중간에 있는 경우에는 warnings 필드도
없었다. 큰 행이 첫 번째일 때만 다음 경고가 있었다.

```json
["invalid_json:invalid character '{' after top-level value"]
```

이 경고는 먼저 전체 입력을 단일 JSON으로 읽으려다 실패한 메시지이며
scanner의 큰 행 오류나 입력의 불완전한 분석을 명시하지 않는다.

정상 대조에서 얻은 recurring_pattern은 다음과 같다.

```text
first_failure failed 1 time(s)
sentinel_failure failed 1 time(s)
```

초기 탐색에서 단일 step_end 객체 하나도 넣었으나, 이는 JSONL 분기가 아니라
단일 JSON 분기로 처리되어 `no_supported_trace_findings`를 반환했다.
그 입력은 비교 대조로 쓰지 않았다. 이후 위 표처럼 행 수·이벤트·순서를 맞추고
padding 길이만 바꿔 차이를 재현했다.

## 판정과 제안

**재현 확인:** 현재 CLI는 큰 JSONL 행 뒤의 지원되는 실패 이벤트를 누락할 수 있다.
성공 exit와 `ok:true`만으로 입력 전체가 분석됐다고 판단할 수 없다.

이는 성능 병목이 아니라 관측 데이터의 완전성 문제다.
우선 제안은 scanner 오류·건너뛴 행·분석된 행 수·불완전 상태를 명시하는 것이다.
큰 행의 허용 상한을 늘릴지, 제한 초과를 명시적으로 거부할지는 별도 설계 선택이다.
무제한 메모리 사용으로 바꾸는 제안은 아니다.

구현 시 수용 기준은 위 네 입력에서 결과 누락이 없거나 불완전함이 명시되고,
기존 malformed-line 복구·redaction·bounded input 계약이 유지되는 것이다.
이번 요청은 조사이므로 production code나 테스트를 수정하지 않았다.
