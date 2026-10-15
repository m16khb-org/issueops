# #561 verified-execution report

- lifecycle: `io-b5a7cbecca2a`, branch `561-korean-label-score`, base `bac123aac302fd423673e60be0268c06696f9e13`
- 계획: `.issueops/issues/561/plan.md`
- 게이트 원장: `.issueops/issues/561/gates.md`(G1~G4 모두 충족, EVIDENCE가 정본)

## 목표와 수용 기준

| 이슈 완료 기준 | 근거 |
|---|---|
| 1) 검증 집합(번호 232 이상 단일 라벨 이슈)에서 기본 threshold로 선택 50개 이상, 정답 비율 85% 이상 | G2: 선택 56, 정답 54, 96.43% |
| 2) 규칙 없는 라벨, 관련 이슈 점수, 명시 score 입력 동작 유지 | G3, `TestScoreIssueOpsRemoteLabelWithoutKeywordRuleKeepsOverlapScore` |
| 3) 실제 이슈 발췌 품질 표 테스트가 선택·기권을 고정하고 `go test ./...`가 통과한다 | G1(RED 확인 후 GREEN). 전체 테스트는 verify 단계 |

## 구현 요약

- `issueops_remote_score_tokens.go`: `issueOpsRemoteLabelKeywords`를 `map[string][]string`으로 바꾸고 키워드를 보강했다.
  `issueOpsRemoteKeywordMatches`(한글 접두 일치, 영어 정확 일치), `issueOpsRemoteLabelEvidence`(이슈 토큰마다 규칙별
  최대 한 번 집계), `issueOpsRemoteLabelEvidenceScore`(라벨 근거 / 전체 근거, 전체 근거 2개 미만이면 0)를 추가하고
  `issueOpsRemoteLabelHeuristic`을 지웠다.
- `issueops_remote_score.go`: 요청마다 근거를 한 번 계산해 `scoreIssueOpsRemoteLabel`에 넘긴다. 규칙 라벨의 evidence는
  두 줄(`type_evidence=<라벨>/<전체>`, `matched_tokens=<정렬, 쉼표 구분>`), 규칙 없는 라벨은 `0.65*overlap`과
  `token_overlap=…`, `type_heuristic=0.00`으로 기존과 같다. 명시 score는 그대로다.
- 테스트: 품질 표(`testdata/label_quality_cases.json`, 번호 232 미만 발췌 7건: 선택 4, 기권 3)와
  `TestScoreIssueOpsRemoteLabelQualityTable`, 규칙 없는 라벨 경계 테스트, 키워드 일치·근거 집계·기권 경계 테스트.
- `scripts/autoresearch/label_score_eval.py`: stdlib만 쓰는 측정 스크립트(`--check`면 마지막 줄 `PASS`/`FAIL`).
- DDD inventory와 `ddd_responsibility_test.go`의 스크립트 등록을 갱신했다.

## 측정 결과(두 구간 병기)

키워드와 최소 근거 2개 규칙은 번호 232 미만(개발 집합)으로 정했고, 검증 집합(232-560)은 규칙 확정 전에 한 번 측정했다.
검증 집합은 완전히 독립된 평가가 아니다.

| 구간 | 대상(단일 라벨) | 선택 | 정답 | 정답 비율 |
|---|---|---|---|---|
| 개발 집합 (0-231) | 114 | 43 | 36 | 83.72% |
| 검증 집합 (232-560) | 116 | 56 | 54 | 96.43% |

개발 집합은 기준(85%)에 못 미친다. 변경 전 229개 전체의 정답 선택은 4개였다.
검증 집합 대상이 계획 작성 시점의 115개보다 1개 많은 것은 측정 시점의 이슈 목록 차이다.

## 성능

hot path가 아니다(이슈를 만들 때 한 번 도는 CLI 경로, 근거 집계는 토큰 수 × 키워드 약 50의 문자열 비교). 계획대로 측정하지 않았다.

## Side effect

- 바뀌는 것: `remote score`의 bug·enhancement·documentation 라벨 `score` 값과 evidence 문자열 내용. evidence 줄 수와 JSON 필드는 그대로다.
- 추가·변경 파일: domain 두 파일과 테스트, `testdata/label_quality_cases.json`, `scripts/autoresearch/label_score_eval.py`,
  DDD inventory JSON과 등록 테스트, `.issueops/issues/561/` 추적 사본.
- 측정 스크립트는 실행 시 `gh issue list`와 `gh label list`로 이 저장소를 읽는다(읽기 전용, 네트워크). 요청 파일은 임시 디렉터리에만
  쓰고 지운다. 원격·저장소에는 쓰지 않으며 CI와 self-verify에 연결하지 않았다.
- record schema, 저장 데이터, CLI/MCP 응답 필드, threshold 기본값, 관련 이슈 점수 변화 없음.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/CONSTITUTION.md` | "재현 근거, 호출 경로, 상태 전이, 정책 경계, 문서 계약을 따라가며 문제가 발생한 근본 원인을 확인한 뒤 그 원인을 제거한다." | threshold를 낮추지 않고 점수 규칙(근거 집계)을 고쳤다 |
| `.issueops/ARCHITECTURE.md` | "`internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete" | 변경이 domain 순수 함수에 머물고 새 import는 `sort`, `unicode/utf8`뿐이다 |
| `.issueops/CONVENTIONS.md` | "새 dependency는 표준 라이브러리로 명확히 부족할 때만 추가한다." | 형태소 분석기 대신 한글 접두 일치를 썼다 |
| `.issueops/CONVENTIONS.md` | "이 표가 유일한 규정이다." (이슈 산출물 레이아웃) | 측정 스크립트를 `.issueops/issues/561/`이 아니라 `scripts/autoresearch/`에 뒀다 |
| `.issueops/TECH_STACK.md` | "Python 검증은 stdlib suite runner와 스킬에 이미 선언된 Pydantic/Typer를 사용한다." | 측정 스크립트는 stdlib만 쓴다 |
| `.issueops/testing/unit-and-contract.md` | "실패해도 원인을 알 수 없는 거대한 fixture나 broad snapshot만 둔다." (피해야 할 예) | 품질 표는 번호 232 미만 발췌 7건으로 두고 이슈 원문 전체는 커밋하지 않았다 |

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/CONVENTIONS.md` | "`internal/domain`은 contract와 순수 domain helper에만 의존하고," | 지킴 | `issueops_remote_score_tokens.go` import는 표준 라이브러리(`sort`, `strings`, `unicode`, `unicode/utf8`)뿐 |
| `.issueops/ARCHITECTURE.md` | "`internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete" | 지킴 | 같은 파일. adapter·`cmd/...` import 없음 |
| `.issueops/CONVENTIONS.md` | "`--json` field는 snake_case." | 지킴(변경 없음) | CLI/MCP 응답 필드와 `IssueOpsRemoteScoredItem`을 바꾸지 않았다 |
| `.issueops/CONSTITUTION.md` | "재현 근거, 호출 경로, 상태 전이, 정책 경계, 문서 계약을 따라가며 문제가 발생한 근본 원인을 확인한 뒤 그 원인을 제거한다." | 지킴 | threshold 기본값을 건드리지 않고 `scoreIssueOpsRemoteLabel`의 점수 규칙을 고쳤다 |
| `.issueops/CAUTIONS.md` | "게이트 원장 `CHECK:`는 argv 한 줄이다." | 지킴 | `gates.md`의 CHECK 네 줄은 모두 단일 argv이고 G1~G4가 met이다 |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | 해당 없음 | `.issueops/*.md` 편집 없음. 추적 사본과 report는 `.issueops/issues/561/` 아래다 |

## 문서 반영 판정

`remote score` 라벨 점수의 내부 규칙(휴리스틱, `token_overlap`, `type_heuristic`)을 설명하는 운영 문서나 스킬 문구는 없다
(`.issueops`, `skills/issueops*`, README 검색 결과 `--judge none`은 모두 `benchmark run`의 판정 모드이거나 호출 예시다).
새 구조 결정이나 재발 함정이 없어 ADR·CAUTIONS는 쓰지 않는다. 키워드 출처와 검증 집합 노출은 계획과 이 report에 남았다.

## 구현 리뷰

독립 서브에이전트(omp `reviewer` 에이전트, 빈 컨텍스트, 읽기 전용)가 diff와 계획을 함께 받아 `pass`를 냈다. 필수 결함은 없었다.
계획 리뷰가 남긴 다섯 주장(evidence 두 줄, 규칙 없는 라벨 불변, 노출 공개, 품질 표 범위, 근거 1개 기권)을 코드와 직접 실행으로 확인했다.
비차단 메모 두 건은 반영했다. 스크립트의 실제와 다른 종료 코드 주석을 지웠고, 이 report의 토큰 수 서술을 고쳤다.

리뷰어가 따로 잰 값(리뷰어 보고, 이 세션에서 재현하지 않음): 이슈 본문에 남은 라벨 판단 문장(라벨 이름과 선택·거절·threshold 어휘가 함께 있는 줄)을
지우고 다시 쟀을 때 검증 집합은 선택 60·정답 55(91.67%), 개발 집합은 선택 46·정답 39(84.78%)였다. G2의 PASS가 본문에 새어 나간 정답에
기대지 않는다는 확인이다. 재현율 한계(영어 키워드는 조사가 붙으면 맞지 않고, 한글 접두는 음절 단위라 `거부되`가 `거부된다`를 잡지 못한다)는
계획이 정한 규칙대로의 동작이다.

## 검증

| 요구한 문서 경로 | 그 문서의 해당 문장 인용 | 명령 | 결과 또는 Not Run 사유 |
|---|---|---|---|
| `.issueops/TESTING.md` | "Go 코드 변경의 기본 검증(`gofmt -l`, `go test -race ./...`, `go vet ./...`, architecture" | `./bin/issueops self-verify --base-ref bac123aa… --seed=100 --target-score=95 --llm-eval=false --json` (scratch venv의 pydantic·typer를 PATH 앞에 둠) | 27 step 중 27 통과, 최소 goal score 100. gofmt, Python 스크립트 테스트, `go test -race ./... -count=1`과 `go vet ./...`(risk QA tier), 빌드, golden 포함 |
| `.issueops/TESTING.md` | 같은 문장(`architecture ratchet`) | `go test ./internal/architecture/ -count=1` | PASS(게이트 G4와 self-verify의 전체 테스트) |
| 게이트 원장 | G1~G4의 CHECK | `issueops gates check --network --timeout-seconds 600 --file .issueops/issues/561/gates.md` | G1~G4 PASS, ALL MET(리뷰 반영 뒤 G2 명령 재실행도 PASS) |
| 스킬(`issueops-implement`) | RED 확인 | `go test ./internal/domain/issueopsremote/ -run TestScoreIssueOpsRemoteLabel` (구현 전) | FAIL(품질 표 4건 선택 실패), 경계 테스트는 PASS |
| 게이트 원장 | 개발 집합 측정 | `python3 scripts/autoresearch/label_score_eval.py --binary bin/issueops --max-number 231` | 선택 43, 정답 36 |

첫 self-verify는 이 머신의 python3에 pydantic이 없어 `skills/slack-delegate` Python 테스트 import에서 실패했다(`ModuleNotFoundError: No module named 'pydantic'`).
이번 변경과 무관한 환경 문제이며, `python3 -m venv`로 만든 임시 환경(저장소 밖)에 pydantic·typer를 설치해 같은 명령을 다시 실행했다. 사용자 홈과 저장소는 바꾸지 않았다.

## 실패와 재시도

- 규칙 없는 라벨 경계 테스트의 첫 기대값(overlap 0.43)이 틀렸다. 라벨 토큰 4개(작은 쪽) 중 3개가 이슈 토큰과 겹쳐 0.75가 맞다.
- 품질 표 후보 #71은 짧은 발췌에서 enhancement 2·documentation 1(2/3)이라 기권이 되어 뺐다. #181은 원문에서 문서 근거가 더 드러나는 문장을 골라 3/4로 맞췄다.
