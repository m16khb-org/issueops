# 2026-10-14 — `gates lint` checks gates without running them, and the author sweeps a fix before each delta review

← [ADR index](../ADR.md)

**결정:** `issueops gates lint`(MCP `gates_lint`)를 더한다. 게이트를 실행하지도 파일을 쓰지도 않고, 구현과 상관없이 met이 될 수 없는 게이트를 찾는다. 대상은 `--gate` spec, 계획 파일의 게이트 줄(`--plan`), 기존 원장(`--file`)이다. 찾는 문제는 다섯 가지다.

- `gates init`이 거부할 spec(셸 연산자, 정규식 안의 `|`가 spec을 자르는 경우)
- `gates check`가 보낼 것과 같은 요청으로 command policy가 거부하는 CHECK(예: `secret_like_argument`)
- CHECK 안이나 EXPECT를 감싼 markdown 백틱
- RE2가 컴파일하지 못하는 EXPECT 정규식
- CHECK가 없는 게이트

계획에서 게이트처럼 보이지만 두 형식 어디에도 맞지 않는 줄은 `unparsed_lines`에 남기고 clean이 아니다. 정책 판정은 `internal/application/gates/command.go`의 `checkPolicyRequest` 하나를 check와 lint가 함께 쓰고, EXPECT 정규식 해석은 `internal/domain/gates/evaluate.go`의 `expectRegex` 하나를 `ExpectMatches`와 lint가 함께 쓴다.

스킬은 세 곳이 바뀐다.

- `issueops-plan`: 게이트를 `gates init`에 그대로 넘길 한 줄 spec으로 쓰고, 스테이징 전에 lint한다.
- `issueops-review` 루프: delta 리뷰를 띄우기 전에 호출자가 세 가지를 점검하고 그 결과를 delta 입력에 넣는다.
  1. 수정이 바꾼 값의 옛 표현이 대상에 남았는지 검색한다.
  2. 수정이 새로 단언한 사실과 리뷰어 처방이 맞는지 명령으로 확인한다.
  3. `gates lint`로 게이트를 검사한다.
- `issueops-review` 출력 계약: 리뷰어는 처방한 최소 수정이 기대는 사실을 확인하고, 수정이 함께 다뤄야 할 입력·경로를 같은 결함에 적는다.

**근거:** 실제 사이클 기록(2026-10-08~11, liar-game·galpi·auth-gateway·ai-character-chat·monty-hall 등)에서 계획 리뷰가 3~5라운드까지 간 35개 리뷰어 세션을 읽었다. delta 라운드의 필수 결함 41건을 분류한 결과는 다음과 같다.

| 분류 | 건수 |
|---|---|
| 수정이 남긴 옛 표현·모순 | 10 |
| 수정이 다루지 않은 새 경계 사례 | 10 |
| 게이트 형식·정책 문제 | 9 |
| 수정 자체가 틀림 | 7 |
| 이전 리뷰어 처방이 틀림 | 3 |
| 기타 | 2 |

41건 중 19건은 검색·정책 판정·정규식 컴파일 같은 결정적 도구로 리뷰 전에 잡을 수 있었다. 라운드 하나는 리뷰어 세션만 1~12분(중앙값 약 5분)이 걸렸다.

**측정 (`scripts/autoresearch/procedure_review_eval.py` 시나리오 `review-delta`, Sonnet 5.5 actor, Opus 5.5 judge, 각 5회):** 1라운드 지적 세 건(이름 변경, 존재하지 않는 `--dry-run` 처방, `|` 정규식 처방)을 반영하고 2라운드 delta 리뷰를 띄우게 했다.

- 이전 스킬: 고친 계획의 게이트가 lint를 통과한 경우 1/5. 나머지는 게이트를 산문(`` `cmd` → EXPECT `x` ``)으로 남겨 `gates init`으로 옮길 수 없었다. 그런데도 2라운드 리뷰어는 5회 중 4회 `pass`를 냈다.
- 새 스킬: lint를 통과한 경우 5/5, judge 점수 82.7 → 100이었다. 세션 시간은 125초에서 150초로 늘었다. 늘어난 시간은 호출자 점검 몫으로, 리뷰어 한 라운드보다 짧다.

**대안:**

- 리뷰어 프롬프트에만 게이트 규칙을 더 넣는 방법: 지금도 리뷰어가 형식 결함을 놓치고 `pass`를 냈다. 결정적 검사로 잡을 수 있는 것을 LLM 라운드에 맡기면 같은 비용을 반복하게 된다.
- `gates check --dry-run`: 실행 경로와 섞이면 정책 audit와 증거 기록 규칙이 갈라진다. 별도의 읽기 전용 명령이 계약이 단순하다.

**결과:** CLI 명령과 MCP tool이 하나씩 늘었다. `mcp_tools.golden.json`, `response_contracts.golden.json`, native install contract matrix golden, omo·omp·agy 템플릿의 catalog digest를 갱신했다. gates 계약 스키마 버전(1)과 기존 명령의 동작은 그대로다.
