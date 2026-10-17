# #566 verified-execution report

- lifecycle: `io-61996bf15223`, branch `566-review-output-contract-and-pr-risk-rules`, base `6237393199944feb0d70d704e74e90dc9d2099a6`
- 계획: `.issueops/issues/566/plan.md`(봉인 원본 sha256 `20c0e613ceb039a4848598d1a361f31174b7cf407bfeedfd0182673cb97c44ba`)
- 게이트 원장: `.issueops/issues/566/gates.md`

## 목표와 수용 기준

Claude Code 팀의 코드 리뷰 관행 가운데 1인 하네스에 옮길 만한 것을 세 스킬 문서의 규칙으로 반영한다. Go 코드는 바꾸지 않는다.
게이트 결과는 원장의 EVIDENCE가 정본이다.

| 기준 | 게이트 | 결과 |
|---|---|---|
| issueops-review 출력 계약에 같은 유형 결함의 전체 발생 위치 나열 규칙이 있다 | G1 | `FOUND` |
| delta 리뷰어가 직전 지적을 해결됨/남음으로만 판정한다 | G2 | `FOUND` |
| 변경이 만들지 않은 결함을 기존 결함으로 표시해 비차단 메모로 둔다 | G3 | `FOUND` |
| 성공 기준과 관련된 결함은 기존 결함으로 내리지 않는다 | G4 | `FOUND` |
| 결함·회귀 수정 intent면 근본 원인 렌즈를 더한다 | G5 | `FOUND` |
| 근본 원인 렌즈의 허용 조건이 CONSTITUTION 원문과 같다 | G6 | `FOUND` |
| create-pr이 `schema-auth`·`contract` tier에서 위험과 되돌리기 절을 요구한다 | G7 | `FOUND` |
| sync-pr 표에 템플릿에 없는 옛 제목이 없다 | G8 | `stale_headings=0`(base는 7) |
| 바뀐 세 스킬이 `validate-skill.py`를 통과한다 | G9 | `ok: Skill is valid!` ×3 |
| 바뀐 세 스킬의 shell 코드 블록이 유효하다 | G10 | `passed: 3 file(s)` |
| response-contract golden이 바뀌지 않는다 | G11 | `ok` |

## 구현 요약

- `skills/issueops-review/SKILL.md`
  - `## 입력`의 원칙 발췌 대상에 결함·회귀 수정을 더하고, 그때 넣을 문장을 `.issueops/CONSTITUTION.md:69-70`으로 지정했다.
  - 출력 계약 4번: 비차단 메모 규칙 뒤에 기존 결함 표시 규칙(제외 조건 세 가지와 악화 시 필수 결함 유지), 최소 수정 문단 뒤에 같은 유형 결함 전체 나열 규칙을 넣었다.
  - 출력 계약 5번: frontend 렌즈 문장 뒤에 근본 원인 렌즈를 별도 문장으로 넣었다. tier·lenses와 상관없이 더한다는 점과 판정 조건(CONSTITUTION:70 원문)을 문장 안에 적었다.
  - 루프 규칙 첫 항목 끝에 delta 리뷰어의 직전 지적 상태 판정 규칙을 넣었다. 저자 측 세 점검, 5회 상한은 그대로다.
- `skills/issueops-create-pr/SKILL.md`: 선택 절 문단에 `review.tier`가 `schema-auth`·`contract`면 `위험과 되돌리기`를 필수로 두는 규칙을 넣었다.
- `skills/issueops-sync-pr/SKILL.md`: 낡은 본문 절 표의 오른쪽 열을 `internal/domain/artifacttemplate/template.go:339-346`의 PR 절 제목으로 바꿨다.
- frontmatter `description`, CLI/MCP 계약, Go 코드는 바뀌지 않았다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/CONSTITUTION.md` | "임시 우회, 출력 문구 보정, 테스트 기대값 완화, 문서만 변경하는 대응은 근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거가 있을 때만 허용한다." | 근본 원인 렌즈의 증상 목록과 허용 조건을 이 문장에서 그대로 옮겼다(G6). |
| `skills/design-review/SKILL.md` | "Naming preferences, optional refactors, minor duplication, speculative future scale, and unrelated pre-existing debt do not block delivery." | 기존 결함 규칙은 무관한 부채만 비차단으로 분류하고 성공 기준 관련 결함은 제외했다(G3, G4). |
| `.issueops/CONVENTIONS.md` | "티어 계산은 implement 이후 phase에서만 일어난다." | create-pr 규칙은 `pr.create` 단계에서 `review.tier`를 읽으므로 실제 변경 집합 tier가 적용된다. |
| `.issueops/conventions/go-and-packages.md` | "검증은 repo-owned `python3 scripts/validate-skill.py skills/<skill-name>`로 수행한다." | 세 스킬을 이 명령으로 검증했다(G9). |
| `.issueops/cautions/2026-10-07-check-expect-escape-re2.md` | "CHECK 안의 줄바꿈은 chr(10)으로 만들고 '\|'는 쓰지 않는다." | 원장 CHECK에 `\|`와 정규식 alternation을 쓰지 않았다. |

`.issueops/TESTING.md`와 `.issueops/CAUTIONS.md` 색인을 대조했다. 새 패키지·컴포넌트가 없어 `.issueops/ARCHITECTURE.md`, API 계약 변경이 없어 `.issueops/OPEN_API_SPEC.md`는 적용 대상이 아니다.

## 성능 영향

Go hot path 변경은 없다. 스킬 본문 바이트 수(`wc -c`):

| 파일 | 변경 전 | 변경 후 | 증가 |
|---|---|---|---|
| `skills/issueops-review/SKILL.md` | 22530 | 24791 | 2261 |
| `skills/issueops-create-pr/SKILL.md` | 8934 | 9201 | 267 |
| `skills/issueops-sync-pr/SKILL.md` | 6808 | 6832 | 24 |
| 합계 | 38272 | 40824 | 2552 |

계획 상한(합계 증가 3,000바이트 이하) 안이다.

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/CONSTITUTION.md` | "임시 우회, 출력 문구 보정, 테스트 기대값 완화, 문서만 변경하는 대응은 근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거가 있을 때만 허용한다." | 지킴 | `skills/issueops-review/SKILL.md` 출력 계약 5번의 근본 원인 렌즈가 이 문장의 증상 목록과 허용 조건을 그대로 쓴다(G6) |
| `.issueops/CONSTITUTION.md` | "문서 변경은 파일 존재, 링크/경로, grep 기반 결정 사항 확인으로 검증한다." | 지킴 | 원장 G1~G8이 공백 정규화 문구 검사와 옛 제목 수로 결정 사항을 확인한다 |
| `.issueops/CONVENTIONS.md` | "티어 계산은 implement 이후 phase에서만 일어난다." | 지킴 | `skills/issueops-create-pr/SKILL.md`의 새 규칙은 `pr.create` 단계에서 `review.tier`를 읽는다 |
| `.issueops/ARCHITECTURE.md` | "pioneer skills는 `skills/` 원본 하나로 host-neutral이며 hub-and-spoke cross-reference를 따른다." | 지킴 | 세 `skills/` 원본만 바꿨고 host별 사본이나 새 링크가 없다 |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | 지킴 | 이 사이클은 `.issueops/*.md` 운영 문서와 스킬 frontmatter를 바꾸지 않았고 G11(`TestResponseContractsGolden`)이 `ok`다 |

`.issueops/CAUTIONS.md`의 게이트 원장 항목("게이트 원장 `CHECK:`는 argv 한 줄이다")도 대조했다. G1~G11은 `python3 -c`·`awk`·`go test` 한 명령씩이고 `|`를 쓰지 않는다.

## side effect

- 파일: 위 세 스킬 원본, `.issueops/issues/566/`의 추적 사본(plan.md, intent.md, plan-review.md), 게이트 원장, 이 보고서.
- 원격: 8~9단계의 브랜치 push와 draft PR 하나(승인 범위).
- durable state: 이 사이클 record.
- 측정 산출물은 워크트리 밖 `/tmp/io-566-measure/`에 두었다(임시 spec, fixture, 결과 JSON, 측정용 binary).
- 설치된 host skill은 `skills/` 원본을 심볼릭 링크로 보므로 재설치가 없다.

## ai-slop-clean

정리 범위는 세 스킬 파일의 추가 30줄·삭제 8줄(`git diff --stat`)과 이 사이클의 `.issueops/issues/566/` 파일이다. 소스 코드 줄이 없어 code-quality-metrics의 네 지표(SNR, entropy, redundancy, overhead)는 N/A이고 줄 증감만 남긴다.

| 분류 | 확인 결과 |
|---|---|
| dead-code·needless-abstraction·boundary-violation | 해당 없음. 코드 변경이 없다 |
| duplication | 근본 원인 렌즈의 허용 조건은 `CONSTITUTION.md:70`을 그대로 옮긴 문장이다. 계획이 원문 일치를 G6으로 고정했으므로 남겼다 |
| weak-artifact | 새 규칙 두 곳의 이유 문장("한 위치만 고친 수정이…")은 같은 절의 기존 이유 문장과 같은 형식의 도메인 근거라 남겼다 |
| unsupported-claim | 보고서의 수치(바이트 수, 재현율, 게이트 결과)를 원장·측정 원문과 대조했다. 근거 없는 "보장·완전·검증됨" 표현은 없다 |

- 제거한 것(weak-artifact): 보고서 초안의 이 절이 `.issueops/conventions/go-and-packages.md` 9절의 "보조 문서를 만들지 않는다" 규칙을 측정 산출물 위치의 근거로 인용했다. 그 규칙은 스킬 디렉터리 안 보조 문서를 다루며 측정 산출물 위치를 정하지 않으므로 인용을 지웠다. 세 스킬 본문에서는 지울 것을 찾지 못했다.
- 범위 밖 발견: `skills/issueops-sync-issue/SKILL.md:32`의 `## 위험과 트레이드오프`가 템플릿 제목 `위험`과 어긋난다. 계획의 비목표라 고치지 않았다.
- 적용한 프로젝트 규칙: 이 정리를 정한 문서 규칙은 없다. `.issueops/CONVENTIONS.md`와 `.issueops/conventions/go-and-packages.md` 9절(Shared Skill 컨벤션)을 대조했다.

## 측정

계획의 측정 계획은 `scripts/autoresearch/procedure_review_eval.py`를 임시 spec(`review-diff`, `review-plan-clean`, `review-delta`, 새 `review-diff-bugfix`)으로 변경 전·후 두 번 돌리는 것이었다. 두 실행 모두 16회 전부가 `claude` CLI 인증 실패(`Failed to authenticate: OAuth session expired`)로 끝났고, `claude auth status`는 `loggedIn: false`였다. 재로그인은 사용자 대화형 인증이 필요해 이 세션에서 할 수 없었다. 그래서 기존 세 시나리오의 회귀 점수는 측정하지 못했다.

대신 같은 fixture로 빈 컨텍스트 리뷰어 서브에이전트 8개(현재 세션 모델, `review.model`과 같은 `anthropic/claude-opus-5-5`)를 띄워 리뷰어 출력만 비교했다. 변경 전은 base의 `skills/issueops-review/SKILL.md`, 변경 후는 워크트리 사본을 리뷰 규칙으로 줬고, 호출자 프롬프트에는 각 버전의 스킬이 호출자에게 요구하는 발췌만 넣었다. judge 모델 없이 출력 원문을 직접 대조했다. 원문과 요약은 워크트리 밖 `/tmp/io-566-measure/proxy-results.json`에 있다.

`review-diff-bugfix`: 결함 수정 intent, base 파일, diff. 심은 결함은 같은 유형 camelCase `--json` 태그 세 곳(E1~E3), 원인 함수 `Remaining`을 둔 채 CLI 보정과 테스트 기대값 완화로 덮은 우회(E4), diff 문맥의 기존 오류 무시(P1)다.

| 실행 | E1~E3 | E4 | E4 근거 | P1 | 결함별 다른 위치 검색 |
|---|---|---|---|---|---|
| 변경 전 1 | 한 결함에 세 곳 | 필수 | intent 성공 기준 | 비차단 | 없음 |
| 변경 전 2 | 한 결함에 세 곳 | 필수 | intent 성공 기준 | 비차단 | 없음 |
| 변경 후 1 | 한 결함에 세 곳 | 필수 | `CONSTITUTION.md:70` | 비차단, 기존 결함 표시 | 결함마다 검색 패턴과 결과 |
| 변경 후 2 | 한 결함에 세 곳 | 필수 | `CONSTITUTION.md:69-70` | 비차단, 기존 결함 표시 | 결함마다 검색 범위와 결과 |

재현율은 전후 모두 4/4라 낮아지지 않았다. 변경 후 두 실행은 E4를 CONSTITUTION 원문으로 필수 결함에 두고 P1을 기존 결함으로 표시했다. 네 실행 모두 fixture에 의도하지 않은 결함(`used > limit`에서 `+1` 보정이 1을 내는 회귀)을 함께 찾았다.

delta 프록시: `review-delta` fixture의 1라운드 지적 세 개에 대해, 지적 1은 한 곳(G2 테스트 이름)을 남기고 고치고, 지적 2는 반영하지 않고, 지적 3은 EXPECT를 `ok`로 바꾼 수정 계획을 줬다.

| 실행 | 직전 지적 상태 | 직전 지적을 새 필수 결함으로 다시 냄 |
|---|---|---|
| 변경 전 1 | 부분 처리 / 미처리 / 오히려 나빠짐 | 지적 1·2를 새 결함 D1·D2로 다시 냄 |
| 변경 전 2 | 부분 반영 / 미반영 / 반영했지만 새 결함 | 지적 1·2를 새 결함 1·2로 다시 냄 |
| 변경 후 1 | 남음 / 남음 / 해결됨(철회, 반증 근거) | 없음. 남은 지적은 "지적 N이 남은 이유"로 표시 |
| 변경 후 2 | 해결됨 / 남음 / 해결됨 | 없음. 다만 지적 1의 남은 테스트 이름을 지적 2 항목에 묶고 지적 1을 해결됨으로 판정했다 |

변경 후 두 실행 모두 직전 지적마다 해결됨·남음을 적었고 직전 지적을 새 결함으로 다시 내지 않았다. 변경 후 2의 지적 1 판정은 남은 위치를 적었지만 상태를 해결됨으로 붙인 오판정이다. 표본이 두 개라 문구 수정 근거로 삼지 않고 결과로만 남긴다.

측정하지 못한 것: 기존 `review-diff`, `review-plan-clean`, `review-delta` 시나리오의 Sonnet actor·Opus judge 점수와 invalid blocking 수. `claude` 로그인 뒤 같은 명령으로 다시 잴 수 있다(`/tmp/io-566-measure/spec.json`, `--bin /tmp/io-566-measure/issueops`).

## 의도 대조

봉인 intent의 성공 기준 일곱 가지를 원장과 대조했다. 1~6은 G1~G8이, 7의 `validate-skill.py`·`verify-skill-shell.py`는 G9·G10이, self-verify 성공은 아래 검증 표가 덮는다. 덮지 않는 성공 기준은 없다. 비목표(CI 경로 tier, Go 강제, 병렬 투표·라운드 상한 변경, `Required` 플래그)는 diff에 없다.

## 검증

| 요구한 문서 경로 | 그 문서의 해당 문장 인용 | 명령 | 결과 또는 Not Run 사유 |
|---|---|---|---|
| `.issueops/TESTING.md` | "문서만 변경한 경우에도 문서 링크·구조를 대조하고, 단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." | `./bin/issueops self-verify --base-ref <base> --seed=100 --target-score=95 --llm-eval=false --json` (테스트 venv를 PATH 앞에 둠) | 27단계 모두 통과, 최소 goal score 100, `termination_eligible` true. risk QA는 Go 변경이 없어 race/static 단계를 건너뛰었다(scope base·head 모두 봉인 base) |
| `.issueops/conventions/go-and-packages.md` | "검증은 repo-owned `python3 scripts/validate-skill.py skills/<skill-name>`로 수행한다." | 원장 G9 | 세 스킬 `ok: Skill is valid!` |
| 게이트 원장 | G1~G11 CHECK | `issueops gates status --file .issueops/issues/566/gates.md` | 11/11 met, 모두 `has_check` true. 마지막 `--write` 실행은 5단계 정리 뒤 |
| 스킬(`issueops-verify`) | 구현 리뷰 | 빈 컨텍스트 서브에이전트 diff 리뷰(tier `docs-only`, 렌즈 `side-effect`, 계획 리뷰 주장 목록 포함) | `pass`, 필수 결함 없음 |
| 스킬(`issueops-verify`) | base 전진 확인 | `issueops execution sync-base --preview` | `merge_needed` false, 충돌 없음. apply하지 않았다 |
| `.issueops/TESTING.md` | 같은 문장 | `go vet ./...`, `go test -race ./...` | Not Run: Go 변경이 없어 self-verify risk QA가 해당 tier를 건너뛰었다 |

첫 self-verify는 이 머신의 python3에 `pydantic`이 없어 `skills/slack-delegate` Python 테스트 import에서 실패했다(`ModuleNotFoundError`). 이번 변경과 무관한 환경 문제이며, `uv venv --python 3.13`에 `scripts/python_test_requirements.txt`를 설치한 저장소 밖 임시 환경을 PATH 앞에 두고 같은 명령을 다시 실행했다. 사용자 홈과 저장소는 바꾸지 않았다.

구현 리뷰(1분 11초)는 첫 self-verify 시도(1분 16초, 위 환경 문제로 실패)와 같은 fingerprint에 대해 동시에 실행했다(`parallel_speed`). 통과한 두 번째 self-verify(venv 준비 포함 2분 36초)는 리뷰가 끝난 뒤 실행했으므로 실제로 겹친 시간은 첫 시도 구간 약 1분이다.

## 발행

- draft PR: https://github.com/m16khb-org/issueops/pull/567 (`main` ← `566-review-output-contract-and-pr-risk-rules`, label `enhancement`, assignee `m16khb`, `Closes #566`)
- 게시 뒤 `remote verify-artifact`와 `gh pr view`로 draft 상태, head·base 브랜치, label, assignee, 연결 이슈, 본문을 다시 읽어 확인했다.
- 구현 커밋 `d395ace2` 하나와 이 보고서 갱신 커밋으로 끝난다. 머지와 워크트리·브랜치 정리는 별도 승인 단계다.
