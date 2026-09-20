# IssueOps 전체 속도·효율·품질 보존 개선 계획

## 요청과 목표

사용자 요청은 “네이밍이나 적대적 리뷰/검증 효율성등을 검토해서 개선해줘”이며,
후속 요청은 “계획까지 작성해줘”다. 리뷰 범위를 이해하기 쉬운 이름으로 드러내고,
중복 실행과 지침 충돌을 줄이되 필수 계약 검증과 독립 리뷰는 유지한다.

- 산출물: 리뷰·검증 스킬 수정, 표시 이름 정리, 시나리오 검토 결과, 검증 증거.
- 예상 규모: 작은 문서·스킬 변경. 새 런타임 기능이나 데이터 마이그레이션은 없다.
- 실행 담당: 메인 에이전트. 독립 시나리오 검토만 빈 컨텍스트 리뷰어에 위임한다.
- 순서: T1 이름과 범위 → T2 판정·실행 규칙 → T3 독립 검토 → T4 저장소 검증.
- 작성일: 2026-09-19. 기준 커밋: `e914ea88`.

## 1차 변경의 완료 상태

아래 네 파일의 수정과 검증을 완료했다. 검증 중 확인한 메타데이터 누락과
스냅샷 불일치도 아래 기록대로 보완했다. 커밋·푸시는 하지 않았다.

| 파일 | 변경 | 검증 상태 |
|---|---|---|
| `skills/issueops-review/SKILL.md` | plan/diff 렌즈 분리, 필수 검증 공백, 결과 수집 뒤 기록, delta 리뷰 조건 | 스킬 검사·독립 검토·전체 검증 통과 |
| `skills/issueops-review/agents/openai.yaml` | 표시 이름을 `IssueOps Plan and Implementation Review`로 변경 | 표시 이름·slug 보존·전체 검증 통과 |
| `skills/issueops-verify/SKILL.md` | 증거 재사용 선행, 독립 작업만 병렬화, 실패 증거 보존, 지표 의미 교정 | 스킬 검사·독립 검토·전체 검증 통과 |
| `skills/design-review/SKILL.md` | 리뷰어는 판정을 반환하고 호출자가 조건 확인 뒤 기록 | 스킬 검사·독립 검토·전체 검증 통과 |

## 조사 근거와 문제

1. 기존 `issueops-review`는 선택된 렌즈만 적용하라는 지시와 네 렌즈를 모두 전달하라는
   지시를 함께 갖고 있었다. `internal/domain/issueops/change_paths.go`의
   `ReviewLensesForTier`는 docs-only에 `side-effect` 하나만 반환한다.
2. `issueops-verify`의 동시 실행 지시가 증거 재사용보다 먼저 나왔다. 같은 파일의
   재사용 규칙을 먼저 적용해야 이미 통과한 명령을 다시 실행하지 않는다.
3. 기존 `design-review`는 라운드 직후 기록을 요구했지만, `issueops-verify`는 병렬
   배터리 실패 시 기록하지 말라고 했다. 리뷰 결과 수신과 durable 기록을 구분해야 한다.
4. CHECK 없는 지적을 필수 결함에서 제외하면, 미검증 인증 계약도 비차단으로 오해할 수
   있다. 한 줄 명령의 유무와 필수 검증 여부를 분리한다.
5. `internal/application/issueopsnext/service.go`의 `applyReviewTier`는 implement
   이전에 git을 조회하지 않고 default를 사용한다. 계획의 인증·스키마 위험은 계획
   내용으로 확인해야 한다.
6. `internal/domain/issueops/review_metrics.go`의 `AggregateReviewMetrics`는 계획
   리뷰의 revise 비율을 계산한다. 이 값을 테스트 배터리 실패율로 사용할 수 없다.

수정 전 독립 리뷰는 네 시나리오 모두에서 모순 또는 실행 규칙 누락을 확인했다.
검토 대상은 위 세 SKILL.md 전문이며, 시나리오는 아래 T3에 그대로 고정한다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md`: 안전·정확성을 유지하고 측정 근거 없이 성능 향상을 주장하지 않는다.
- `.issueops/ARCHITECTURE.md`: 공용 스킬 원본은 `skills/`에 두며 host별 로직을 복제하지 않는다.
- `.issueops/AGENT_WORKFLOW.md`: 현재 파일과 명령 결과를 근거로 검증·완료를 보고한다.
- `.issueops/cautions/issueops-stages.md` §6: untracked 파일도 fingerprint에 포함된다.
- `.issueops/TESTING.md`, `.issueops/testing/self-verification.md`: 저장소 필수 검증과
  다단계 시나리오의 단일 성공 run 계약이 일반적인 증거 재사용보다 우선한다.
- `skills/design-review/SKILL.md`: 독립 리뷰는 필수 계약 결함과 검증 공백을 판정한다.

## 선택한 접근과 범위

공개 명령을 대규모로 바꾸는 대신 표시 이름·설명·절차의 의미를 맞춘다.
`issueops-review`는 계획과 구현의 독립 리뷰 실행·기록을, `issueops-verify`는 봉인된
변경의 증거 확인·결과 수집·readiness를, `design-review`는 독립 판정 기준을 소유한다.

CLI 명령, 스킬 slug, JSON 필드, 온디스크 schema, 모델 기본값은 유지한다.
이름의 취향 차이만으로 저장소 전체를 rename하지 않는다. 이번에 확인된 문제는 스킬
지시의 모순이므로 Go core에 캐시·스케줄러·새 지표를 추가하지 않는다.

### Gap analysis

- 필수 검증을 건너뛰어 실행 횟수를 줄이면 목표 위반이다. 재사용은 명령·입력·환경·
  의존성·외부 상태 유효성이 확인된 성공 증거에만 허용한다.
- 같은 DB·브라우저 상태를 쓰는 검증은 파일을 수정하지 않아도 충돌할 수 있다.
  독립성이 확인된 작업만 병렬화한다.
- 리뷰 pass가 먼저 도착해도 필수 검증과 fingerprint 확인 전에는 기록하지 않는다.
- CHECK 없는 필수 검증 공백은 revise/stop이다. 보완 증거는 delta 리뷰로 전달한다.
- 실제 처리 시간 단축률은 이번 정적 수정만으로 입증할 수 없다. 같은 시나리오에서
  중복 실행 요구가 없어졌는지 검증하고, 초 단위 개선 수치는 주장하지 않는다.

## 작업과 완료 기준

### T1. 이름과 책임을 일치시킨다

- 상태: 완료. 표시 이름과 H1 일치, slug와 공개 식별자 보존 확인.
- 담당/분류: 메인 / quick. T2와 순차 수행.
- 대상: `skills/issueops-review/SKILL.md`, `skills/issueops-review/agents/openai.yaml`,
  `skills/issueops-verify/SKILL.md`의 description.
- 방법: 표시 이름은 `IssueOps Plan and Implementation Review`, slug는
  `issueops-review`로 유지한다. description은 사용 시점을 설명하고 본문이 절차를 소유한다.
- 확인: `python3 scripts/validate-skill.py skills/issueops-review`와
  `python3 scripts/validate-skill.py skills/issueops-verify`가 종료 코드 0이다.
- 정상 QA: 표시 이름과 H1을 비교해 둘 다 계획·구현 리뷰를 명시하는지 확인한다.
- 경계 QA: `git diff --name-only`에 명령 catalog나 contract 파일이 없고,
  frontmatter name과 `$issueops-review` 참조가 유지되는지 확인한다.
- Commit: 이 작업 단독 커밋 없이 T4 이후 변경 전체를 한 문서 커밋 후보로 묶는다.

### T2. 실행 범위와 판정 기록 순서를 정리한다

- 상태: 완료. 네 시나리오 검토와 마지막 문구 변경의 독립 delta 리뷰 pass.
- 담당/분류: 메인 / quick. T1 이후, T3 이전.
- 대상: 위 세 SKILL.md. 근거: `applyReviewTier`, `ReviewLensesForTier`,
  `AggregateReviewMetrics`와 해당 기존 테스트.
- 방법: plan/diff 렌즈를 구분하고, 증거 재사용 → 필요한 독립 작업 실행 → 결과 수집 →
  fingerprint 재확인 → 기록 순서를 명시한다. 실패 출력과 지적은 수정 근거로 보존한다.
  추가로 `issueops-review/SKILL.md` 입력 절의 “정리와 재검증이 끝난 diff”를
  “정리와 문서 반영이 끝나 봉인된 diff”로 고친다. 검증과 리뷰의 동시 실행은
  `issueops-verify`를 따르고, 기록은 모든 필수 결과 확인 뒤라는 문장을 연결한다.
  이 문장은 수정 후 리뷰에서 확인한 기존 충돌이며, 이번 범위에서 함께 해소한다.
- 확인: `python3 scripts/verify-skill-shell.py skills/issueops-review skills/issueops-verify skills/design-review`
  및 `git diff --check`가 종료 코드 0이다.
- 정상 QA: docs-only의 유효한 성공 증거를 재사용하고 side-effect 렌즈만 선택한다.
- 실패 QA: 배터리 실패 후 먼저 도착한 pass를 기록하지 않으며 실패 출력은 보존한다.
- 경계 QA: 한 줄 CHECK를 못 쓰는 필수 인증 공백은 차단하며, 확인 절차와 통과 조건을
  받은 뒤 delta 리뷰한다. 전체 리뷰 반복 사유는 구조·범위 변경이다.
- Commit: T4 이후 통합 문서 커밋 후보에 포함한다.

### T3. 수정 전후 같은 시나리오로 독립 검토한다

- 상태: 완료. 수정 후 네 시나리오와 T2 추가 수정의 독립 delta 리뷰 모두 pass.
- 담당/분류: 메인은 자료·판정 확인, 빈 컨텍스트 독립 리뷰어 / deep.
- 참조: 세 SKILL.md 전문, 현재 diff, 위 Go 근거 세 파일. 구현과 무관한 선택적
  개선은 차단 조건으로 삼지 않는다.
- 실행: 아래 네 입력을 그대로 전달하고 실행할 작업·재사용할 증거·판정·기록 시점을 받는다.

| 시나리오 입력 | 통과 조건 |
|---|---|
| docs-only, lenses=[side-effect], 동일 fingerprint의 유효한 테스트 성공 증거 | 중복 테스트 없음, 필요한 리뷰는 side-effect, readiness 유지 |
| 구현 리뷰 pass 뒤 병렬 배터리 실패 | pass 기록 없음, 실패 증거 보존, 구현 단계로 복귀 |
| 필수 인증 계약 미검증, 한 줄 CHECK 불가 | revise 또는 stop, 최소 확인 절차·통과 조건 제시 |
| 구현 전 인증/schema 변경 계획 | default를 계획 위험 분류로 오해하지 않고 계획의 필수 계약 검토 |

- 실패 QA: 지침 간 충돌 또는 필수 계약 누락이 하나라도 있으면 revise로 남기고 T2로 복귀한다.
- 완료 기준: 네 상황 모두 일관된 행동을 도출하고 새 필수 결함이 없다. 수정이 필요하면
  직전 지적·수정 delta·영향 계약만 재검토한다. 최대 3라운드 뒤 남은 결함을 보고한다.
- 증거: 원문 결론과 파일 위치를 이 계획의 검증 결과에 요약한다.
- Commit: 별도 커밋 없음.

### T4. 저장소 검증과 범위 확인

- 상태: 완료. 스냅샷과 메타데이터 보완 뒤 전체 검증을 재실행해 모두 통과했다.
- 담당/분류: 메인 / quick. T3의 최종 수정 뒤 실행.
- 참조: `.issueops/TESTING.md`, `internal/adapter/skillcontract/skill_contract_test.go`.
- 실행 명령:

```bash
python3 scripts/validate-skill.py skills/issueops-review
python3 scripts/validate-skill.py skills/issueops-verify
python3 scripts/validate-skill.py skills/design-review
python3 scripts/verify-skill-shell.py skills/issueops-review skills/issueops-verify skills/design-review
git diff --check
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops docs --json
./bin/issueops inspect --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

- 정상 QA: 모든 명령이 성공하고 self-verify의 각 목표 점수가 95를 초과한다.
- 실패 QA: 실패한 명령·전체 출력을 보존하고 원인을 확인한다. 필수 다단계 검증은
  실패 후 첫 게이트부터 다시 실행한다. 실패한 run의 부분 통과를 완료 증거로 합치지 않는다.
- 완료 기준: 마지막 단일 성공 run과 독립 검토 결과를 보고한다. 검증 실행 전의 상태를
  통과로 기록하지 않는다. 커밋·원격 게시 여부는 사용자 요청 범위에 따른다.
- Commit: 요청 시 `docs(review): clarify review scope and verification ordering` 후보를
  사용하고 `.issueops/COMMIT_POLICY.md`의 Lore body를 적용한다.

## 1차 변경의 최종 검토

- [x] F1. T1~T4 완료 여부와 명령 결과를 현재 파일 상태로 확인했다.
- [x] F2. 공개 계약 변경, 무관한 리팩터링, 필수 검증 면제가 없다.
- [x] F3. 네 시나리오에서 실행·판정·기록 규칙이 일관된다.
- [x] F4. 검증 범위를 넘는 성능·완료 주장을 하지 않는다.

## 계획 근거 요약

Repo grounding: 세 리뷰·검증 스킬, 표시 메타데이터, change_paths.go,
issueopsnext/service.go, review_metrics.go 및 프로젝트 검증 규칙을 확인했다.

Decision-complete plan: 메인이 T1~T4를 순차 수행하고 T3만 독립 리뷰어가 판정한다.

Assumptions/defaults: 공용 스킬 지침과 표시 이름 개선으로 범위를 정하고 공개 식별자는 유지한다.

Unresolved questions: 실행을 막는 질문은 없다. 실제 벽시계 개선량은 측정하지 않았다.

Acceptance criteria: 스킬 검사, 네 시나리오 독립 검토, 저장소 기본 검증과 scope 확인.

## 검증 결과

- 수정 전 독립 검토: 네 상황 모두 기존 지침의 충돌 또는 누락을 확인했다.
- 수정 후 독립 검토: `pass`/`proceed`. docs-only 증거 재사용, 배터리 실패 시
  pass 기록 보류, CHECK 없는 필수 인증 공백 차단, 계획 내용에 따른 위험 검토를 확인했다.
- 추가 발견: `issueops-review` 입력 절의 “재검증이 끝난 diff” 문구와 병렬 실행
  규칙의 기존 충돌은 T2에 구체적인 교체 문장으로 반영했다. 파일에 적용했고 독립 delta 리뷰 pass를 받았다.
- 실행 완료: 세 스킬의 `validate-skill.py`, 세 스킬 대상 `verify-skill-shell.py`,
  `git diff --check` 모두 종료 코드 0.
- 1차 변경의 남은 필수 작업: 없음. 아래 성공 run으로 T4를 완료했다.
  후속 요청으로 확장한 전체 개선안은 아래 P0~P8이며 아직 구현하지 않았다.

### 검증 중 확인한 메타데이터 누락

첫 `go test ./... -count=1`에서 `TestResponseContractsGolden`이 스킬 수
52 대 51로 실패했다. 기준 커밋 `e914ea88`이 추가한 `issueops-explain` 항목이
스냅샷에 빠져 있었고, 이번 변경의 두 description도 갱신 대상이었다.
`inspect`는 `issueops-explain`의 `has_openai_yaml=false`를 반환했으며,
`cmd/issueops/validationcli/qagate/validation_qa_gate.go`는 이 누락을 오류로 처리한다.

T4의 필수 검증을 복구하는 최소 보완으로
`skills/issueops-explain/agents/openai.yaml`에 표시 이름·설명·기본 프롬프트를 추가하고,
`go test ./cmd/issueops/issueopsapp -run '^TestResponseContractsGolden$' -update -count=1`로
`cmd/issueops/testdata/response_contracts.golden.json`을 갱신한다. 갱신 diff가 스킬
메타데이터와 목록에만 한정되는지 확인하고 전체 검증을 처음부터 다시 실행한다.
스킬 본문이나 런타임 동작은 바꾸지 않는다.

### 최종 성공 run

2026-09-19, 수정 완료 후 T4의 모든 명령을 처음부터 순차 실행해 종료 코드 0을 확인했다.
추가한 `issueops-explain` 메타데이터도 `validate-skill.py`로 검사했다.

- `go test ./... -count=1`: 통과. 첫 실패 run의 부분 결과를 완료 증거로 사용하지 않았다.
- `go build -o bin/issueops ./cmd/issueops`: 통과.
- `docs --json`, `inspect --json`: 각각 `ok=true`.
- `self-verify --seed=100 --target-score=95 --llm-eval=false --json`:
  `ok=true`, `termination_eligible=true`, 26/26 steps 통과, 최소 목표 점수 100.
- 표시 이름과 H1 일치, 기존 slug·기본 프롬프트 참조 보존, 세 스킬의 로컬 링크 존재를 확인했다.
- golden diff는 두 스킬의 description과 누락된 `issueops-explain` 목록·메타데이터만 바뀌었다.
- Go 소스 변경이 없어 self-verify의 risk QA가 race/static 추가 실행을 생략했다.
  이번 완료 주장은 스킬 절차와 메타데이터 개선에 한정하며 실제 시간 단축률은 주장하지 않는다.

로컬 원본 로그는 `/tmp/issueops-review-efficiency-final/results.json`과 같은 디렉터리의
`01.log`~`11.log`에 있다. 최초 실패 출력은
`/tmp/issueops-review-efficiency-tests.log`에 보존했다. 이 임시 로그 경로는 이 머신의
검증 기록이며 새 체크아웃에서는 위 명령으로 재현한다.


## 후속 요청: 전체 하네스 개선 조사와 실행 계획

사용자 요청: “적대적 리뷰 뿐만아니라 전체적인 issueops의 성능이나 퀄리티를
감소시키지 않으면 속도 효율성 아키텍처들을 개선할 수 있는 방안들에 대해서
조사하고 계획에 반영해줘”. 이번 후속 작업의 산출물은 조사와 계획이다.
아래 최적화 구현은 수행하지 않았으며, 앞의 T1~T4 완료와 구분한다.

별도 요청인 `issueops-explain` → `explain` 확장은 일반 개념·코드·계획·판단 근거·
작업 결과 설명으로 적용했다. 현재 원본은 `skills/explain/`이며 앞 절의
`issueops-explain` 경로는 1차 검증 당시의 기록이다. IssueOps router와 호스트의
스킬 링크도 새 이름을 사용한다.

### 조사 방식과 확인 수준

- 런타임: CLI/MCP의 공용 application·adapter 호출 경로에서 실제 중복 관측을 확인했다.
- 검증: 이전 성공 run의 단계 시간과 패키지 시간을 읽고, 반복 subprocess 호출 지점을 조사했다.
- 문서/에이전트: 문서 라우터의 잘못된 분기를 실제 CLI로 재현했다. 필수 자료의
  일괄 축약이나 모델 하향으로 토큰을 줄이는 접근은 채택하지 않는다.
- 구조: Go core와 얇은 host adapter 경계를 유지한다. 이번 조사만으로 새 daemon,
  전역 cache, SQLite lock 분할, 공개 DTO 변경의 필요성은 입증되지 않았다.
- 벽시계 측정은 이 머신의 관찰값이다. 반복 횟수로 예상되는 절약과 실제 end-to-end
  개선률을 구분한다. 성능 개선 완료 판정은 아래 P0 비교 실험을 통과해야 한다.

### 현재 확보한 근거

| 영역 | 확인한 사실 | 해석의 한계 |
|---|---|---|
| 자체 검증 | 기존 성공 run 152.976초 중 `go test` 149.216초 | 단일 run이며 테스트 내부 원인까지 증명하지 않음 |
| inventory | 같은 repo 문자열마다 `Normalize`가 Git subprocess 실행 | 전체 `next` 지연은 아직 측정하지 않음 |
| local readiness | fingerprint·schema·review tier가 변경 경로를 반복 관측 | fetch나 mutation 경계를 넘는 재사용은 금지 |
| architecture tests | 같은 package inventory의 반복 `go list` 호출 | 독립성 검사가 목적인 재실행은 유지해야 함 |
| install tests | 같은 실물 바이너리를 반복 빌드하는 fixture | 각 테스트의 파일 변경 격리는 유지해야 함 |
| 문서 선택 | `performance profiling`이 `pr` 부분 문자열로 commit 분기 선택 | 잘못된 분기는 재현했지만 토큰 절감률은 측정하지 않음 |

실측 원본: `/tmp/issueops-review-efficiency-final/11.log`, 같은 폴더의 `07.log`,
`results.json`. 문서 선택 재현은 `/tmp/issueops-performance-route.json`이며 명령은:

```bash
./bin/issueops project route-docs --repo "$PWD" --task 'performance profiling' --json
```

현재 출력에 `COMMIT_POLICY.md`와 commit/PR 목적의 reason이 포함된다.
원인은 `internal/adapter/projectdocs/project_docs_route.go:91`의 `strings.Contains(task, "pr")`다.

### 성능·품질을 보존하는 채택 기준

1. **정확성:** 변경 전후 같은 fixture에서 필수 필드·오류·경고·정렬·권한 판단·
   stale 판단·외부 쓰기 횟수를 비교한다. 의도한 문서 라우팅 수정만 별도 기대값으로 명시한다.
2. **안전성:** schema 거부, workspace 경계, redaction, actor/generation fence,
   lease·provider 최신 관측, 실패 시 차단, 취소·timeout·복구 계약을 유지한다.
3. **검증 품질:** 테스트·assertion·필수 gate 수를 줄이지 않는다. 공유할 것은 불변
   준비 자료뿐이며 오류 상태·파일·DB·소켓·프로세스는 테스트별로 격리한다.
4. **성능:** CPU 시간, 할당량, peak RSS, subprocess 수, end-to-end 중앙값/p95를
   함께 비교한다. 빠른 단위 benchmark만으로 사용자 대기 시간이 줄었다고 보고하지 않는다.
5. **측정 판정:** 입력/도구 버전/환경을 고정하고 변경 전후를 번갈아 측정한다.
   Go benchmark는 각각 20회, CLI 시나리오는 warm-up 5회 뒤 각각 30회로 사전 고정한다.
   cold start는 별도 집계한다. 유리한 결과가 나올 때까지 반복하지 않는다.
6. **통과 조건:** subprocess 중복은 예상 수까지 줄고, 대응하는 시간 개선이 관측되며,
   비대상 시나리오의 지연·메모리 악화와 품질 손실이 없어야 한다. baseline 반복 측정의
   변동 범위를 먼저 기록한다. 차이가 불분명하면 개선 입증 실패로 두고 배포하지 않는다.
   통계적으로 차이가 없다는 사실만으로 동등성이 증명됐다고 주장하지 않는다.

유한한 fixture만으로 모든 입력에서 무손실을 증명했다고 말하지 않는다. 보존할 계약을
명시하고 회귀·실패·동시성 사례를 늘려 검증 범위를 확보한다.

### 실행 순서와 책임

메인이 구현과 측정을 담당한다. 독립 리뷰어는 계약 보존과 측정 해석을 확인한다.
P0를 먼저 완료하고 P8→P1→P2→P3→P4→P5→P6 순으로 한 변경씩 측정·채택한다. P7은 그 결과를
통합 검증한다. 서로 다른 후보의 벤치마크를 동시에 실행해 측정을 오염시키지 않는다.
각 후보는 별도 작은 커밋 단위이며 실패하면 그 후보만 되돌릴 수 있어야 한다.

| 작업 | 우선순위 | 의존성 | 현재 상태 |
|---|---|---|---|
| P0 기준선·품질 비교 harness | 선행 필수 | 없음 | 계획 |
| P8 hostprobe 테스트 helper의 불필요한 subprocess 감축 | 높음, 최장 패키지의 측정 후보 | P0 | 계획 |
| P1 inventory 경로 정규화 중복 제거 | 높음 | P0 | 계획 |
| P2 architecture 테스트의 불변 package inventory 공유 | 높음 | P0 | 계획 |
| P3 install 테스트의 빌드 산출물 공유·복사 | 높음 | P0 | 계획 |
| P4 local readiness 내부 관측 통합 | 중간, 경계 위험 있음 | P0 | 계획 |
| P5 문서 선택 정확성 및 에이전트 입력 비용 개선 | 높음, 오분류 재현됨 | P0 | 계획 |
| P6 최종 검증 배터리의 단일 실행 계약 | 높음, 규칙 변경 선행 | P0 | 계획 |
| P7 통합·host parity·독립 품질 평가 | 최종 필수 | 채택된 P1~P6·P8 | 계획 |

#### P0. 기존 측정 도구와 계약 테스트를 기준선으로 고정한다

- 담당/분류: 메인 / deep. 새 상시 telemetry 서버는 만들지 않는다.
- 재사용: self-verify JSON의 `steps`, `goal_scores`, `slowest_steps`, Go benchmark,
  현재 golden, architecture fitness tests, install rollback tests.
- 산출물: ignored `.issueops-runtime/efficiency/` 아래 baseline/candidate 결과와
  도구 버전·입력 크기·환경·명령·원본 출력·실행 횟수. 추적 문서에는 요약만 남긴다.
- 먼저 기존 go test stdout과 `go test -json`의 package/TestCase 시작·종료를 대조한다.
  기존 run의 최장 package는 hostprobe 92.320초, remoteverify 74.942초, GitLab provider
  66.548초, GitHub provider 64.479초다. 겹쳐 실행된 package 시간을 합산하지 않는다.
  P2/P3의 약 15초 package가 전체 critical path라는 근거는 없으므로 전체 wall time도
  반드시 비교한다. hostprobe의 subprocess 대기·fixture 비용은 아래 보충 조사로 분해한다.
- Go 명령 형태: `go test <해당-package> -run '^$' -bench <대상> -benchmem -count=20`.
  CLI는 application fixture 기반 1/100/1000개 cycle, clean/dirty/untracked worktree로
  측정한다. 실제 사용자 record나 remote write를 성능 실험에 쓰지 않는다.
- 분석: `benchstat`으로 시간·할당 비교, CLI 원본 duration으로 중앙값/p95 비교.
  도구 버전을 기록하고 일괄 설치·런타임 의존성 추가는 하지 않는다.
- 정상 QA: 같은 revision을 baseline/candidate 양쪽으로 실행하면 계약 결과가 일치한다.
- 실패 QA: 출력 한 필드·오류 한 건·secret redaction 차이를 넣은 fixture는 차이를 검출한다.
- 완료: 개선 전 자료 확보, 품질 차이 탐지 검증, 측정 조건 확정. 이 단계에서 속도 개선을 주장하지 않는다.

#### P1. 목록 조회의 저장소 경로 정규화를 요청 내에서 공유한다

- 대상: `internal/application/issueopsinventory/service.go:34,48`,
  `internal/adapter/outbound/issueopsinventory/runtime.go:16`.
- 기존 비용: record N개마다 Git 정규화. 별도 read-only 20회 측정에서 이 저장소의
  단일 rev-parse 중앙값 16.333ms(최소 10.709ms, 최대 38.166ms). 전체 지연의 예측값은 아니다.
- 방법: 함수 지역 map으로 동일한 입력 문자열의 정규화 결과만 재사용한다. service 필드나
  전역 cache는 만들지 않는다. 요청 종료 때 버리고 다음 요청은 새로 관측한다.
- 보존: 전체 record scan, schema 검증, invalid diagnostics, 정렬, repo 필터,
  Git 실패 fallback. 다른 경로 문자열을 근거 없이 같은 repo로 취급하지 않는다.
- 정상 QA: N=1/100/1000, 고유 경로 U=1/10/N에서 Normalize 호출이 U회 수준으로 감소하고
  기존 entry·diagnostics 결과가 같다. repo 필터 자체의 호출은 별도 집계한다.
- 실패 QA: 삭제된 repo, Git 실패, 다른 worktree, 요청 사이 `.git` 연결 변경, 동시 repo
  조회를 비교한다. 경로가 요청 중 바뀌는 경우 허용할 snapshot 의미를 독립 리뷰에서
  검증하고 기존 권한 판단을 약화시키면 채택하지 않는다. 쓰기 직전 권한 관측은 공유하지 않는다.
- 재사용/제외: record store는 이미 `GetAllExisting` bulk scan이다. DB N+1 제거를 새로 만들지 않는다.
- 완료: 호출 수와 end-to-end 시간을 함께 개선하고 계약 비교 통과. Commit 후보: `perf(inventory): reuse path normalization within a list request`.

#### P2. architecture 테스트의 package inventory를 공유한다

- 대상: `internal/architecture/dependency_test.go:1274,1304`,
  `internal/architecture/orphan_package_test.go:126`. production cache는 만들지 않는다.
- 근거: Edges 13회, Packages 10회, ModulePackages 2회로 동일 go list 총 25회.
  기존 package 시간은 15.856초/14.937초. 별도 go list 3회는 0.722/0.419/0.441초,
  출력은 각각 828,121 bytes였다. 작은 탐색 표본이므로 P0의 통계적 비교를 대신하지 않는다.
- 방법: 같은 테스트 실행의 불변 `go list -json ./...` 결과를 한 번 읽고 Edges/Packages/
  ModulePackages 관점을 파생한다. 호출자가 map/slice를 바꾸지 못하도록 복사 또는 읽기 전용 접근을 사용한다.
- `dependency_test.go:225,231`의 byte-stability를 검증하는 독립 두 번째 조회는 유지한다. 한 결과를 두 번 비교하면 해당 테스트가 무의미해진다.
- 정상 QA: 모든 기존 import graph·레이어·소유권 assertion을 유지한 채 목록 호출 수가
  25→2로 줄어든다. orphan의 TestImports/XTestImports를 production edge에 섞지 않는다.
- 실패 QA: 의존성 위반 fixture가 계속 실패하고, go list 실패·잘린 JSON이 그대로 실패로 전달된다.
- 완료: architecture 패키지와 전체 테스트의 시간·메모리 비교 통과. Commit 후보: `perf(test): share immutable architecture package inventory`.

#### P3. install fixture에서 실물 바이너리를 한 번 빌드한다

- 대상: `cmd/issueops/installcli/install_command_test.go`의 `buildManagedCommandAt`,
  `install_orchestration_test.go`의 `installOrchestrationFixture`, 기존 `wiring_main_test.go`.
- 근거: `install_command_test.go:275,295,321`의 직접 호출 3개와
  `install_orchestration_test.go:420,428` fixture를 쓰는 11개 테스트가 총 14회 빌드한다.
  기존 package 시간은 13.732초/15.780초이며 전부 절약 가능한 시간은 아니다.
- 방법: 테스트 실행 안에서 같은 소스·빌드 옵션의 실물 바이너리를 한 번 만들고,
  각 테스트의 TempDir에 독립 파일로 복사한다. 기존 TestMain의 wiring·cleanup을 보존한다.
- 파일을 공유하는 hardlink는 쓰지 않는다. staged candidate를 손상시키는 테스트가
  다른 테스트의 원본까지 바꾸면 검증 품질이 떨어진다.
- 정상 QA: build 수 14→1, 복사본 hash/실행 권한 동일, 각 테스트의 real-binary 검사 유지.
- 실패 QA: 한 복사본을 손상시켜도 다른 복사본은 정상. 최초 build 실패는 전체 fixture 실패로
  전파하고 성공처럼 재사용하지 않는다. adoption·rollback·abort·dry-run assertion 모두 유지.
- 완료: install 패키지 시간 감소 및 race/순서 변경 실행 통과. Commit 후보: `perf(test): reuse the build and isolate install fixtures`.

#### P4. local readiness의 변경 경로 관측을 통합한다

- 대상: `internal/adapter/issueops/implementation/evidence.go:81,238`,
  `issueops_pr_readiness_strict.go:41`, `issueops_schema_evidence.go:67`,
  `internal/application/issueopsnext/service.go:118,174`.
- 순서: 먼저 local readiness 안에서 경로와 fingerprint를 함께 구해 기존
  `schemaEvidenceMissingForPaths`에 전달한다. 그 변경의 효과·정확성 확인 뒤 같은
  `Next` 호출의 review tier에도 전달할지 결정한다. 공개 JSON/port 확장은 첫 단계에 넣지 않는다.
- 보존: base/root 선택, 정렬, 내용 해시, fingerprint 바이트, 실패 경고, stale 판정.
  strict readiness의 fetch 경계를 넘거나 mutation의 권한 판정에 재사용하지 않는다.
- 정상 QA: clean/dirty/untracked/deleted/renamed/schema fixture의 missing/warnings/
  fingerprint/tier가 기존과 같다. 관측 묶음의 Git subprocess 수를 계측한다.
- 실패 QA: BaseSHA 불량·fallback ref, 읽기 실패, 동시에 파일 변경, 서로 다른 repo 요청.
  관측 중 변경은 재관측 또는 기존 실패 경로로 처리하며 검증되지 않은 snapshot으로 승인하지 않는다.
- 완료: 같은 결과와 낮은 지연을 입증한 단계만 채택. Commit 후보: `perf(readiness): share local change observations`.

#### P5. 문서 라우팅의 오분류를 고치고 입력 비용을 측정한다

- 대상: `internal/adapter/projectdocs/project_docs_route.go:91`,
  `internal/adapter/projectdocs/projectdocs_test.go`, `cmd/issueops/projectcli/project_cli_test.go`.
- 방법: `pr`·`ci` 같은 짧은 영문 약어를 단어 경계로 인식한다. 공식 다단어 표현과
  기존 명확한 trigger는 유지한다. performance/profiling 전용 경로는 ARCHITECTURE,
  TECH_STACK, TESTING으로 명시하고 한국어 “성능”, “프로파일링”도 같은 경로로 검증한다.
  AGENTS와 실제 적용되는 필수 규칙은 계속 읽는다.
- 정상 QA: `performance profiling`, `성능 프로파일링`은 커밋 전용 reason을 받지 않고,
  `PR review`, `CI test`, `pull request`, `openapi endpoint`는 각각 기존 의도에 맞는 문서를 받는다.
- 실패 QA: `profile`, `improve`, `principal`처럼 약어가 포함된 다른 단어가 잘못 매칭되지
  않는지 확인한다. 복합 요청은 필요한 범주를 누락하지 않도록 기존 우선순위를 명시하고 테스트한다.
- 품질 비교: 고정된 12개 요청(구현·검증·설계·API·VCS·운영 각 2개)의 필요한 문서 집합을
  먼저 작성한다. 필수 문서 누락 0, 불필요 문서 수·읽은 bytes·토큰과 답변의 근거 정확성을 비교한다.
  문서 수가 늘어도 정확성에 필요하면 유지하며, 줄어든 bytes를 토큰 절감으로 동일시하지 않는다.
- 완료: CLI 재현 해소와 전체 라우팅 회귀 통과. Commit 후보: `fix(docs): route performance requests without abbreviation collisions`.

#### P6. 최종 검증 배터리의 실행 책임을 하나로 정리한다

- 대상: `.issueops/TESTING.md:32`, `.issueops/testing/self-verification.md`,
  `skills/self-verify/SKILL.md`, `cmd/issueops/selfworkflow/steps/self_verify_steps.go:62,110`,
  `cmd/issueops/selfworkflow/verifyloop/loop.go:85`.
- 근거: 기존 run에서 외부 전체 테스트 85.92초 뒤 self-verify 내부 전체 테스트를 다시
  실행했다. 두 시간의 차이를 wrapper 비용으로 해석하지 않는다. 현재 문서가 두 실행을
  나열하므로 먼저 문서의 필수 명령과 self-verify의 실제 실행 항목을 대응시킨다.
- 방법: **최종 검증 배터리 한 run과 self-verify 한 호출을 구분한다.** 같은 revision·
  환경에서 self-verify가 실제 수행한 test/build/golden/docs/inspect만 중복 제거한다.
  self-verify가 실행하지 않은 필수 `go vet ./...`·`go test -race ./... -count=1` 등은
  같은 배터리에서 별도로 실행해 결과를 연결한다. 검증 문서의 정규 소유 절차를 이
  포함 관계로 정리한다. 새 실행기나 영속 cache는 추가하지 않는다. 개발 중 focused tests는 계속 수행하지만 최종 검증 직전의
  동일 전체 battery를 의무적으로 한 번 더 실행하지 않는다.
- golden은 이미 전체 test의 결과를 재사용하고, race 성공 뒤 일반 test 재사용도 이미
  구현돼 있다. 이 로직을 새로 만들지 않는다. run 사이 영속 성공 cache도 추가하지 않는다.
- 정상 QA: 기존 필수 명령/검사 항목 전부가 최종 run의 실제 실행 증거와 1:1 또는 명시적
  포함 관계로 연결되고 전체 go test가 의도 없이 두 번 호출되지 않는다. Go 변경을 커밋한
  clean working tree에서도 필수 vet/race가 누락되지 않아야 한다. 검증 범위는 현재
  unstaged 파일만이 아니라 검증 대상 base~head diff와 보존된 작업 범위를 기준으로 정한다.
- 실패 QA: test 실패, 중도 취소, revision·환경 변경, prompt-only LLM 평가, 불완전 결과는
  완료 판정을 만들지 못한다. 다음 시도는 기존 단일-run 원칙에 따라 처음부터 수행한다.
- 근거 보충: `cmd/issueops/riskqa/risk_qa_plan.go:20`은 변경 경로가 없으면 검사 명령이
  없는 계획을 반환하지만 `.issueops/testing/unit-and-contract.md:13`은 Go 변경에
  vet/race를 요구한다. self-verify `ok` 하나로 두 항목까지 실행됐다고 추정하지 않는다.
- 완료: 검사 범위 동일성에 대한 독립 리뷰와 실제 단일-run 성공 증거. 문서 변경은
  `project-docs-update`의 SHA-CAS 절차를 사용한다. Commit 후보: `docs(testing): use one complete verification run as final evidence`.

#### P7. 통합 검증과 채택 판정을 남긴다

- 채택된 변경별 baseline/candidate와 근거를 독립 리뷰한다. 후보가 효과를 입증하지
  못하면 보류한다. 계획을 전부 구현했다는 이유로 채택하지 않는다.
- Go 변경은 기존 필수 test/vet/race/build/architecture/golden과 self-verify를 실행한다.
  CLI·MCP 결과, Codex·Claude·Omo 공용 경로를 확인하고 실제 host 호출은 격리된 fixture로 검증한다.
- 에이전트 품질은 알려진 결함이 있는 사례와 결함 없는 사례를 함께 사용해
  발견률·잘못된 지적·잘못된 통과를 비교한다. 네 시나리오의 단발성 pass로 일반 정확성을 주장하지 않는다.
- 완료: 모든 필수 계약 보존, 채택 항목의 실측 개선, 비대상 경로의 성능 악화 없음,
  미입증 후보·검증 범위를 보고. 원격 게시·배포는 이 조사 요청에 포함하지 않는다.

#### P8. hostprobe 테스트의 일반 Python 실행을 직접 전달한다

- 대상: `internal/adapter/hostprobe/child_host_smoke_script_test.go:867`의
  `fakePythonScript`(줄 번호는 구현 전 확인). production host probe는 변경하지 않는다.
- 근거: helper는 Python stdin 실행마다 mktemp → sed → 실제 Python → trap rm을 거친다.
  프로그램 치환이 필요한 조건은 `FAKE_SCENARIO=activated-digest-blank`와
  activated.json 대상으로 한정된다. 기존 shell fixture 호출은 7 단일+18 drift+4 authority로
  총 29개이며 내부에 여러 Python 호출이 있다. 이미 t.Parallel을 쓰므로 무조건 병렬도 증가는 제외한다.
- 방법: 이 특수 조건은 현재 치환 경로를 유지하고, 나머지는 같은 `REAL_PYTHON`과 argv·
  stdin·env로 직접 exec한다. fixture의 200줄 이상 script와 production 스크립트를 복제하지 않는다.
- 측정: P0의 hostprobe testcase 시간과 mktemp/sed/python/rm 수, tempfile 생성 수를
  전후 비교한다. 92.320초에서 이 비용의 비율은 아직 분리하지 못했으므로 절약량을 추정하지 않는다.
- 정상 QA: 일반 scenario의 exit/stdout/stderr·host 결과가 같고 불필요한 임시파일·shell
  subprocess가 줄어든다. 기존 29개 invocation과 assertion은 모두 유지한다.
- 실패 QA: activated digest blank를 계속 재현하고, Python 비정상 종료·stdin 전달·
  취소·tmp cleanup 동작을 대조한다. 기존 failure가 pass로 바뀌면 거부한다.
- 완료: helper 호출 수 감소와 package·전체 wall time의 개선을 확인한 경우만 채택.
  Commit 후보: `perf(test): bypass unused Python fixture rewriting`.

### 지금 채택하지 않는 접근

- test 선택 실행·샘플링으로 최종 검증 축소, timeout 단축으로 성공처럼 보이게 하기.
- `-count=1` 제거만으로 검사 생략을 속도 개선으로 보고하기.
- 모든 gate 동시 실행: self-verify 시간의 약 97.5%가 단일 go test다. 이 run에서
  나머지를 모두 겹쳐도 순차 시간 3.760초가 상한이고 실제 절약은 의존성 때문에 더 작다.
  작은 상한을 위해 fail-fast·취소·progress 순서를 바꾸는 scheduler 개편은 보류한다.
- 전역 Git/provider/권한 cache, 요청 사이 stale 정보 재사용.
- SQLite writer lock을 측정 없이 쪼개거나 WAL이면 여러 writer가 동시에 쓸 수 있다고 가정하기.
- 리뷰 모델 하향, 필수 성공 기준 생략, 모든 문서를 손실 요약으로 대체하기.
- 이미 적용된 bulk record scan, local/strict readiness 분리를 새 개선처럼 다시 구현하기.

### 외부 1차 자료와 적용 범위

조회일: 2026-09-19. 공식 문서의 기능·측정 원칙과 저장소 실제 구현을 대조했다.

- [Go diagnostics](https://go.dev/doc/diagnostics): CPU·heap·block·mutex profile과
  tracing으로 비용을 구분한다. profile 자체의 비용 때문에 겹쳐 측정하지 않는다.
- [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat): 반복 A/B 비교,
  중앙값·신뢰구간, before/after 교차 실행, 유리한 결과가 나올 때까지 재시도하지 않는 기준을 사용한다.
- [go test flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags): `-count=1`은 결과 cache를
  끄는 명시적 방법이다. 검증을 없애는 대신 반복 fixture 준비를 줄인다.
- [SQLite isolation](https://www.sqlite.org/isolation.html): writer 직렬화와 snapshot
  격리를 고려해 lock/cache 변경을 독립적인 정확성 검증 없이 추진하지 않는다.

Source fan-out: 런타임 호출 경로, 검증 실측, 문서 라우팅 재현, 공식 Go/SQLite 자료.
Source index: 위 URL과 조회일, 각 P 항목의 소스 경로 및 명령.
Claim verification: 중복 호출·오분류는 코드/실행으로 확인. 후보별 개선률은 P0 이후 입증 대상.
Access boundary: 공개 공식 문서만 사용했으며 로그인·인증 우회는 없다.


### 후속 계획 완료 기준

- [x] 런타임·검증·문서 선택의 실제 호출 경로와 관찰 근거를 조사했다.
- [x] 이미 적용된 최적화와 신규 후보를 구분했다.
- [x] P0~P8의 범위·순서·채택 기준·실패 시나리오·rollback 단위를 정의했다.
- [x] 독립 계획 리뷰 통과. P6의 미포함 필수 검사 보존을 보완한 뒤 delta 판정 pass.
- 런타임 최적화 구현은 이 후속 요청의 완료 기준이 아니다. 조사·계획 완료와 실행 완료를 구분한다.


### explain 확장 검증

- 원본 경로·frontmatter·표시 이름·기본 호출을 `explain`으로 통일하고,
  `skills/issueops/SKILL.md`의 공용 보고 링크를 새 경로로 수정했다.
- 일반 개념 설명과 계획의 속도·정확성 근거 설명 두 요청을 독립 세션에서 실행했다.
  IssueOps 명령 없이 답했고, 가상 예와 실측되지 않은 효과를 구분했다. 필수 결함 없음.
- `validate-skill.py skills/explain`, 해당 shell fence 검사, golden 갱신과
  `self-verify --seed=100 --target-score=95 --llm-eval=false --json` 통과.
  자체 검증 26/26, 최소 목표 점수 100. 원본 로그: `/tmp/issueops-explain-selfverify.json`.
- Codex·Claude·Omo·agy의 기존 사용자 skill 경로가 같은 `skills/explain` 원본을 가리키며,
  이번 저장소의 삭제된 원본을 가리키던 `issueops-explain` 링크만 정리했다.
- `skills/`와 현재 golden에서 `issueops-explain` 활성 참조가 남지 않았음을 확인했다.
  이 계획의 이전 명칭은 과거 작업 기록으로 유지한다.


### 후속 계획의 독립 리뷰 결과

첫 판정은 revise였다. clean working tree에서 self-verify가 vet/race를 선택하지 않는
조건을 지적했고, P6에 최종 배터리와 self-verify 호출을 구분하는 절차를 추가했다.
실행하지 않은 필수 검사는 같은 배터리에서 별도로 수행하며 base~head 변경 범위를 유지한다.

수정 후 독립 delta 리뷰는 proceed/pass였다. P0의 critical path 우선 측정,
P6의 필수 검사 보존, P8의 특수 Python fixture 변조·취소·cleanup 유지 조건을 확인했다.
현재 완료한 범위는 전체 개선 조사와 P0~P8 계획 작성이며, 이 후보의 런타임·테스트 코드
최적화는 아직 실행하지 않았다. `explain` 확장만 별도 사용자 요청에 따라 적용·검증했다.

## 추가 요청: 메인·인계 세션과 런처·호스트 호환성

사용자 요청은 Orca·Herdr·cmux와 Codex·Claude Code·Omo 호환성까지 조사해
효율·품질 개선을 계획하는 것이다. 추가 제약은 **메인→인계 세션의 병렬 작업과
발생 가능한 문제를 고려할 것**이다. 아래 H0~H5는 조사에 근거한 후속 구현 계획이다.
실제 세션 생성·프롬프트 전송·lease 변경·외부 도구 설치/업데이트는 이번 조사에서 하지 않았다.

### 런처, 호스트, 권한을 구분한다

- 런처는 터미널·pane·프로세스·입력을 다루는 Orca/Herdr/cmux다.
- native host는 실제 작업을 수행하는 Codex/Claude Code/Omo다.
- IssueOps의 direct/orca execution mode는 런처 이름과 같은 분류가 아니다.
  기본 direct 사이클을 Orca나 Herdr에서 이어받는 경로도 있다.
- terminal/pane 생성, 입력 접수, host turn 시작, IssueOps owner claim 성공은
  서로 다른 사실이다. 하나의 `ready`/`success`로 합치지 않는다.
- machine/server/runtime도 대상 identity의 일부다. 이번 설치 관측은 로컬 머신 기준이며,
  원격 연결에서는 로컬 pane ID·PID·파일 경로를 그대로 재사용하지 않는다. 원격 artifact
  접근과 actor 증명을 확인하지 못한 조합은 unavailable로 남기고 로컬로 몰래 전환하지 않는다.
- host를 바꿀 때 native 대화 세션 자체를 이식한다고 가정하지 않는다. 새 host 세션에
  사용자 요구·제약·자료·검증 근거를 넘기고 실제 수신자 identity로 권한을 다시 획득한다.
  기본은 같은 host 인계이며 임의의 모델·host 하향 전환으로 비용을 줄이지 않는다.

### 현재 환경과 지원 근거: 2026-09-19 관측

| 대상 | 직접 확인한 상태 | 확인하지 않은 범위 |
|---|---|---|
| Orca 1.4.200 | `orca status --json`: runtime ready/reachable, prompt-delivery capability 존재 | 세 host의 새 세션 인계 E2E는 이번 조사에서 실행하지 않음 |
| Herdr 0.9.0 | server running, protocol 22 호환, worktree open/pane run/agent start help 확인 | agent prompt의 실제 새 turn 시작·claim E2E 미실행 |
| cmux 0.64.10 | 앱 번들 CLI 존재, PATH에는 없음. help·Info.plist·binary 문자열 확인 | 기본 socket과 `/tmp/cmux.sock` 모두 없음. ping/capabilities는 socket not found로 실패 |
| Codex CLI 0.155.1 | 설치된 help의 prompt·resume·fork·model 옵션 확인 | 살아 있는 사용자 세션 재개/중단은 실행하지 않음 |
| Claude Code 2.1.272 | 설치된 help의 prompt·resume·fork-session·session-id 확인 | 동일 |
| Omo 5.0.0-0.beta.22 | engine senpi 2026.8.26-2, 초기 prompt·session-id·resume·json/rpc 모드 확인 | 동일 |

cmux는 미설치가 아니라 **설치됐으나 현재 제어 연결 불가**다. 조사 목적으로 앱을
자동 실행하거나 socket 접근 모드를 완화하지 않았다. CLI 경로 검출은 PATH와 정규
번들 경로를 구분하되 다른 앱/서버로 몰래 fallback하지 않는다.

| 런처 × host | 저장소/설치 CLI에서 확인한 수준 | 후속 호환성 검사 |
|---|---|---|
| Orca × Codex | core launch 및 host runner 존재 | 입력 receipt, exact session/claim, 재시작·취소 |
| Orca × Claude | core launch 및 host runner 존재 | 동일 |
| Orca × Omo | core launch 존재, dispatch 후 별도 prompt send, 모호한 복구 거부 | 2회 호출 사이 crash·응답 유실·owner claim 대조 |
| Herdr × Codex | skill recipe와 `agent start --kind codex` 지원 | 준비 상태·prompt 결과·native claim |
| Herdr × Claude | skill recipe와 `agent start --kind claude` 지원 | 동일 |
| Herdr × Omo | Omo 초기 prompt를 `pane run`으로 실행하는 skill recipe | generic pane 실행·정확한 PID/session 확인 |
| cmux × Codex | terminal 제어 CLI 존재, IssueOps cmux 경로 없음 | 명시적 workspace/surface·cwd·접수/claim 증거 |
| cmux × Claude | 동일 | 동일 |
| cmux × Omo native | generic terminal 후보. `cmux omo`는 별개 OpenCode wrapper | native `omo` 절대경로 확인 후 generic terminal 경로만 검증 |

위 표는 live 호환 인증표가 아니다. 특히 Herdr의 `omp`와 IssueOps Omo native,
cmux의 `omo [opencode-args...]`는 서로 같은 실행 파일이라고 가정하지 않는다.
cmux 번들 CLI의 문자열에 `oh-my-opencode.json`이 있으며 root help도 opencode args를 명시한다.
`rg 'cmux|herdr' internal cmd --glob '*.go'`에서는 전용 adapter를 찾지 못했고,
Herdr 통합은 현재 `skills/issueops/references/session-choice.md`가 소유한다.

### 코드에서 확인한 안전장치와 공백

1. **이미 있는 보호:** claim transaction, 같은 actor 재시도의 멱등성, 다른 actor·generation
   거부, 동시 reseed 단일 성공. `internal/adapter/outbound/issueopslease/sqlite.go:56,90`,
   `internal/application/issueopslease/reseed_test.go:571`. 새 분산 lock을 도입하지 않는다.
2. **병렬 작업 정리 공백:** `session-choice.md:69`는 인계문 → release → launch지만
   agent/test/generator의 종료·수집 절차가 없다. `internal/application/issueopslease/release.go:36`
   는 holder/token을 비우며 이미 실행 중인 프로세스는 멈추지 않는다. late write 가능 조건이며
   실제 사고가 관측됐다는 뜻은 아니다.
3. **전달 복구 공백:** 선택 기록은 launch 이전 환경을 담지만 실제 pane/native session,
   prompt digest, 접수/시작 관측을 지속 기록하는 절차가 부족하다(`session-choice.md:48,88,154`).
4. **direct 자료 대조:** `internal/adapter/outbound/issueopslease/claim_context.go:49`는
   direct preflight를 건너뛴다. 이는 현재 설계 차이이며 일반 direct claim 결함으로 단정하지 않는다.
   우선 인계 양식과 수신자 자료 대조를 보강한다. Orca packet 요건을 모든 direct 호출에 강제하지 않는다.
5. **Omo 실행 검증 공백:** 설치 matrix는 세 host를 검사하지만
   `cmd/issueops/issueopsapp/tool_conformance_facade.go:29`와
   `scripts/verify-child-host-smoke.sh:608,888`의 실행 검증은 Codex/Claude 두 host다.
   `internal/adapter/omo/install_test.go:62`의 문자열 검사는 extension 실행 증거가 아니다.
6. **Omo 전달의 모호성:** `internal/adapter/orca/execution.go:344`의 dispatch와 send는
   별도 호출이다. `:258`은 전달이 입증되지 않은 inspect 복구를 거부하고,
   `internal/adapter/orca/client.go:617`은 send의 accepted만 확인한다. 실패 차단을 유지하며
   제공되는 receipt와 exact owner claim으로 모호성을 해소할 수 있는지 검증해야 한다.

### 병렬 인계 절차: 먼저 겹칠 수 있는 일과 기다려야 할 일을 나눈다

| 순서 | 메인 세션 | 병렬 허용 | 다음 단계의 필수 조건 |
|---|---|---|---|
| A 인계 준비 | 새 쓰기 작업·새 하위 작업 dispatch를 멈추고 현재 작업을 열거 | 런처 capability/help/status 읽기, 자료 목록 정리 | 작업별 소유자·실행 handle·입력 revision·쓰기 범위·결과 위치 확보 |
| B 작업 정리 | writer는 완료하거나 취소하고 실제 종료를 확인 | 격리된 입력의 읽기 전용 조사, 영향 없는 capability 조회 | canonical worktree와 공유 state의 writer 0; 결과·실패 출력 수집 |
| C 인계 자료 확정 | 요구·범위·종료점·plan digest·HEAD/diff·검증·미완료 작업 기록 | 확정 snapshot의 읽기 검사 | 자료와 파일 상태가 일치; 동작 중인 작업이 있으면 허용 조건 명시 |
| D 권한 반납 | 기존 actor로 release하고 결과 재조회 | transport 상태 읽기만 | 기존 holder 해제 확인. 새 세션을 미리 owner로 만들지 않음 |
| E 모드별 수신 준비·전달 | direct는 별도 수신 세션, orca execution은 core 복구 체인으로만 owner 생성 | 수신자의 bootstrap 읽기 | 아래 모드별 복구 절차와 exact target/receipt 준수 |
| F 수신자 대조·복구·claim | 송신자는 구현을 적용하지 않음; orca coordinator 복구만 허용 | 수신자 내부의 독립 읽기 검사 | claimable 상태·현재 generation·자료/cwd/scope·실제 새 identity 확인 후 claim |
| G 작업 재개 | 메인 세션은 종료 | 수신자만 새 writer 작업 시작 | 새 권한·필수 근거가 유효함. 전달과 인계 완료 상태를 구분해 보고 |

D 이후에는 **execution mode별 기존 복구 체인**을 반드시 따른다. release만으로
claimable이 되지 않는다(`internal/domain/issueopslease/claim.go:36`, `reseed.go:52`).

- direct: 기존 worktree에 새 수신 세션을 연다 → 수신자가 자료와 현재 status를 대조한다 →
  `next/status`가 반환한 replace preview와 exact 복구 명령을 순서대로 따른다 →
  새 generation과 claimable 상태를 실제 조회한다 → 자기 native actor로 claim한다.
  direct에는 Orca 전용 `execution resume`을 사용하지 않는다.
- orca execution: release한 준비 세션이 status의 replace/reseed/resume 체인을 수행한다 →
  core가 봉인된 새 owner를 생성한다 → 그 owner만 봉인된 claim을 사용한다.
  E에서 별도 terminal/pane/agent를 먼저 만드는 일반 런처 절차는 생략한다.
  준비 세션의 coordinator 복구와 source 구현 재개는 구분하며, 수동으로 연 세션은
  core가 지정한 owner를 대신 claim하지 않는다.

이 분기는 `skills/issueops/references/session-choice.md:63,103`을 재사용한다.
상태 이름을 수동 수정하거나 예전 token/generation을 복사하는 새 우회 경로는 만들지 않는다.

B 단계에서 “테스트니까 읽기 전용”이라고 분류하지 않는다. build, golden update,
fixture 생성, formatter, generator, child process가 어떤 파일·DB를 쓰는지 확인한다.
확인할 수 없는 작업은 writer로 취급한다. source 밖의 immutable snapshot과 독립
임시 디렉터리를 쓰는 작업만 계속 실행할 수 있다.

이전 세션의 메모리 mailbox나 in-process sub-agent handle은 새 host에서 이어받을 수
있다고 가정하지 않는다. 공유 가능한 결과 artifact·실행 handle이 없으면 완료를 기다려
결과를 저장하거나 작업을 취소한다. 늦게 도착한 결과는 입력 revision과 출처를 붙여
수신자가 평가하며, 이전 세션은 그 결과로 코드를 수정하거나 통과를 기록하지 않는다.

송신자 종료는 수신 작업 전체 완료를 기다리는 감독 루프로 바꾸지 않는다. 전달 receipt를
확인하고 필요한 상태를 남기면 종료할 수 있다. 다만 `input_accepted`를 `owner_claimed`로
보고하지 않는다. 새 owner claim은 수신자가 수행하며 다음 상태 조회로 별도 확인한다.

### 인계 실패·경쟁 조건과 복구

| 상황 | 올바른 처리 | 검증할 불변식 |
|---|---|---|
| generator가 늦게 파일을 씀 | B 단계에서 종료/취소 확인 전 release 금지 | sender writer와 receiver writer의 활동 구간이 겹치지 않음 |
| writer 취소 실패 또는 후손 프로세스 생존 | 한도 내 관측 뒤 인계를 보류하고 기존 권한 상태를 보고 | 단순 signal 성공을 프로세스 종료로 간주하지 않음 |
| 조사 결과가 claim 뒤 도착 | receiver가 revision 대조 후 사용/폐기 | sender가 작업 재개하지 않음 |
| release 뒤 launcher 시작 실패 | released 상태 보존, core가 허용하는 claim/recovery로만 재개 | 옛 token/actor로 쓰기 재개 금지 |
| prompt 전송 성공 후 응답 유실 | 같은 request/terminal incarnation을 재관측 | 새 pane·새 런처로 재전송해 이중 작업을 만들지 않음 |
| TUI startup timeout | readiness 미확인으로 기록, 원본 handle 확인 | 입력 유실을 성공으로 보고하지 않음 |
| 런처 setup hook이 claim 전에 source를 수정 | 인계용 시작 경로의 setup 부작용을 사전 확인하고 비활성/격리 가능한 경로만 사용 | 새 세션 시작을 읽기 전용이라고 추정하지 않음 |
| Herdr working 중 prompt 대기 | active turn 종료를 새 prompt 완료로 해석하지 않음 | turn 식별 없는 상태 wait는 exact receipt가 아님 |
| 런처 재시작으로 handle 무효 | runtime·process identity와 새 목록 대조 | 화면 이름·active pane만으로 다른 세션에 전송하지 않음 |
| 수신자 두 개가 동시에 claim | 기존 transaction/fencing으로 한 owner만 승인 | loser는 파일 수정·remote write 0 |
| plan/HEAD가 release~claim 사이 바뀜 | 인계 자료 stale 표시, 새 scope/자료 확인 뒤 검증 | 이전 성공 증거를 현재 대상으로 승격하지 않음 |
| 사용자 취소·범위 축소 도착 | 최신 지시를 우선 기록·전달, 새 dispatch 중지 | 저장된 옛 인계문이 새 지시를 덮지 않음 |
| socket/permission/host version 부적합 | 해당 조합 unavailable, 현재 세션 경로 유지 | 자동 설치·권한 완화·다른 서버 오접속 금지 |

모든 wait는 timeout을 명시하고 최대 두 번의 관측으로 제한한다. 결과가 모호하면
성공/실패를 만들어 내지 않고 현재 상태와 다음 read-only 확인 경로를 남긴다. 시간 만료는
미전달 증거가 아니다. 정확한 delivery retry를 지원하지 않는 런처에는 자동 재전송을 추가하지 않는다.

### H 작업 목록과 완료 기준

#### H0. capability·호스트 호환성 기준선을 만든다

- 담당: 메인, quick. P0 측정 방식 재사용. 기존 CLI help/status와 runtime capability를
  read-only로 수집해 설치됨/연결됨/기능 지원/실행 검증을 각각 기록한다.
- 실제 버전·실행 경로·runtime identity에 묶인 한 인계 시도의 관측만 재사용한다.
  프로세스 재시작이나 대상 변경 뒤에는 다시 조회한다. 승인·권한 정보는 캐시하지 않는다.
- 테스트: 런처 3종×수신 host 3종의 9조합, launcher 없는 direct 3조합을 계약 fixture로 검사.
  지원하지 않는 조합의 조용한 fallback도 실패로 검증한다. cross-host 6방향은 host session
  이식이 아니라 새 identity/자료 인계로 검증하고, 사용자가 선택한 경우에만 사용한다.
- 모드 QA: direct released→replace preview→반환 복구 체인→claimable→claim과
  orca released→replace/reseed/resume→봉인 owner claim을 각각 검사한다.
  released 즉시 claim, stale generation claim, orca의 별도 세션 이중 생성을 실패 사례로 둔다.
- 완료: 모든 조합에 supported/unsupported/unavailable/not-run과 근거가 있고 빈 칸이 없음.
  live 미실행을 supported E2E로 표시하지 않는다.

#### H1. 병렬 작업 종료·결과 수집을 인계 전 필수 단계로 만든다

- 대상: `skills/issueops/references/session-choice.md`, `skills/issueops-plan/SKILL.md`,
  `skills/issueops-implement/SKILL.md`의 인계·하위 작업 경계. 필요 시 기존 host process
  runner와 cancellation 경계를 재사용한다. 일반 process manager를 core에 복제하지 않는다.
- 인계 자료에 outstanding 작업의 owner/handle/revision/write scope/result location을 기록한다.
  B의 writer 종료 확인과 C의 최종 상태 재확인을 거친 뒤 release한다.
- 정상 QA: reader 2개와 writer 1개가 동시에 실행될 때 reader의 독립 결과는 병렬 수집하고
  writer 종료 전에는 release가 진행되지 않는다.
- 실패 QA: 늦은 generator, 취소 무시 자손, old agent callback, 사용자 취소. filesystem
  변경 시각·프로세스 종료·claim 기록을 함께 확인한다. lease 검사만으로 통과시키지 않는다.
- 완료: pending writer=0 증거와 late result 격리. 권한 반납 시간을 줄이려 안전 절차를 생략하지 않는다.

#### H2. 전송 사실과 작업 권한을 분리해 복구 증거를 남긴다

- 대상: `session-choice.md`, `internal/adapter/orca/execution.go`, `client.go`, 기존 trace/audit 경계.
- 새 orchestration authority를 만들지 않는다. 사용자 runtime 영역의 인계 관측 artifact에
  attempt ID, prompt digest, launcher/runtime, exact terminal/pane/process, source generation,
  input accepted/turn observed/owner claimed/ambiguous 상태와 시각·원본 receipt 경로를 남긴다.
  이 기록은 권한·자동 retry의 근거가 아니며 IssueOps claim만 작업 권한을 부여한다.
- Orca는 지원 capability를 확인한 뒤 durable request ID와 `--retry-request` 의미를 이용한다.
  prompt 내용 또는 process incarnation이 달라지면 같은 ID를 재사용하지 않는다.
- Herdr는 `--wait`의 state 기반 한계를 유지한다. `agent_prompt_stalled`·timeout에서
  무조건 재전송하지 않는다. cmux raw send는 native turn 시작 receipt로 취급하지 않는다.
- Omo dispatch→send 간 crash도 현재 fail-closed 거부를 유지하며, 실제 receipt 또는
  일치하는 exact owner claim이 있을 때만 복구 완료로 판정할 수 있도록 테스트한다.
- 실패 QA: 모드별 replace/reseed/resume 각 경계의 crash와 generation 전환도 포함한다.
  released 상태에서 잘못된 직접 claim이나 중복 owner 생성을 유도해 거부를 확인한다.
  각 외부 호출 전후 crash, accepted 응답 유실, stale handle, 다른 payload 재시도,
  두 launcher의 중복 수신 시도. 성공 기준은 owner 하나와 중복 실제 작업 0이다.
- 완료: 모호한 전송에서 blind retry가 없고 관측만으로 재개 경로를 선택할 수 있다.
  receipt 보존은 기존 record schema의 자동 migration이나 새 token 전달을 만들지 않는다.

#### H3. 인계 자료로 조사를 이어받고 stale 증거를 걸러 낸다

- 대상: `session-choice.md:93` 인계 양식, 기존 owner context/packet 렌더링.
- 자료: 사용자 목적·비목표·승인된 종료점, source/canonical worktree, base/HEAD/diff,
  plan 경로·digest, 완료한 조사와 검증의 입력/명령/시점/환경, 미완료 작업과 결과 위치,
  현재 상태·실패·재개 명령. 비밀·인증정보·old token을 prompt에 넣지 않는다.
- 수신자는 최신 자료와 대조해 유효한 근거를 재사용하고 변경된 부분만 다시 조사한다.
  필수 문서를 손실 요약으로 대체하지 않으며 원본 경로와 필요한 구절을 함께 제공한다.
- direct 인계에 자료 기대값 검사를 추가하되 일반 standalone direct claim을 막는 새
  Orca packet 필수 조건으로 확대하지 않는다. 공용 core schema 변경은 별도 계약 검토 후 결정한다.
- QA: 계획 변경·다른 HEAD의 테스트·기록 누락·cross-host session ID 혼동을 주입한다.
  관련 자료를 다시 확인하지만 같은 유효한 조사 전체를 반복하지 않는지 측정한다.
- 완료: 성공 기준 누락 0, stale 근거 채택 0, 입력 토큰/중복 조사/claim까지 시간 비교.

#### H4. Omo runtime 검증을 Codex·Claude와 같은 기준으로 확장한다

- 대상: `cmd/issueops/issueopsapp/tool_conformance_facade.go`, `internal/adapter/hostprobe/`,
  `scripts/verify-child-host-smoke.sh`, `internal/adapter/omo/extension.go`와 테스트.
- 기존 HostProbeRunner port에 Omo runner를 추가하는 최소안을 사용한다. 설치 matrix와
  live host probe를 구분하고 Omo native를 OpenCode/OMP로 대신 실행하지 않는다.
- 우선 mock pi에서 startup·compact 허용/거부·JSON 오류·CLI 실패·`triggerTurn:false`를
  검사한다. 이후 격리된 live probe에서 세 host의 context 주입, MCP 호출, 응답 digest,
  종료 코드, 취소 후 자손 프로세스 종료, timeout을 같은 기준으로 비교한다.
- live는 외부 host/account가 준비된 opt-in 경로다. 일반 install/update/self-verify의
  readiness에 외부 companion 도구·실계정을 새 필수 요건으로 넣지 않는다.
- 완료: Omo를 제외한 성공을 세 host 실행 호환성이라고 보고하지 않으며, 실제 probe가
  없는 결과는 Not Run 사유와 설치·mock 검사 결과를 구분해 표시한다.

#### H5. cmux를 검증 가능한 런처 후보로 추가하고 종단 인계를 비교한다

- 우선순위: H0~H3 뒤. 기존 Orca→Herdr→현재 세션 기본 순서는 유지하고,
  cmux는 명시적으로 선택한 환경에서 capability와 live proof가 확보된 경우만 활성화한다.
- 구현 범위는 기존 worktree를 지정 cwd의 workspace/surface로 여는 얇은 CLI 절차다.
  execution authority와 canonical worktree 생성은 IssueOps가 계속 소유한다.
- bare active/focused target을 쓰지 않고 exact workspace/surface/runtime을 지정한다.
  `cmux omo`를 Omo native launcher로 사용하지 않는다. shell 초기 command의 인용·argv·
  prompt 경계를 검증하고 PID/env/scope를 직접 대조한다.
- 현재 socket 부재는 unavailable로 보고했다. 앱 시작·설치·socket 접근 변경을 이 조사에서
  자동 수행하지 않는다. live 실행이 가능한 환경에서만 해당 조합의 호환성을 인증한다.
- QA: 세 launcher×세 host의 same-host 인계 계약, 6개 cross-host 자료 인계 방향,
  권한 거절·socket 끊김·runtime 재시작·잘못된 cwd·prompt 긴 문자열/줄바꿈/따옴표.
- 성능: startup, input receipt, owner claim, 첫 유효 작업까지 시간을 각각 기록한다.
  작업 정리 시간과 런처 시작 시간을 분리하고 반복 prompt·중복 세션·누락된 결과·
  고아 프로세스·불필요한 재조사를 함께 센다. 더 빠르더라도 이 오류가 늘면 채택하지 않는다.
- 완료: 유효한 owner 하나, old sender 후속 쓰기 0, 미회수 자손 0, 성공 기준 누락 0,
  관측으로 설명할 수 있는 실패 복구. 라이브 미실행 조합은 인증 목록에서 제외한다.

### 의존성·효율·검증 범위

H0와 기존 P0는 읽기 조사로 병렬 진행할 수 있다. H1/H2/H3의 파일·권한 경계 변경은
순서대로 검증한다. H4 mock 검사는 독립 실행 가능하지만 live probe와 성능 측정은
리소스 경합 없이 실행한다. H5는 H0~H3의 안전 계약 이후에만 진행한다.
인계 전 단순 상태 조회·자료 읽기를 겹칠 수는 있지만 release/claim/쓰기 자체를
병렬화해 시간을 줄이지 않는다. 새로운 세션을 생성했다는 이유만으로 이미 유효한 검증을 반복하지 않는다.

조사 중 기존 domain claim/reseed, application claim 순서·동시 reseed·보상 재시도,
SQLite claim/context preflight, hostprobe Codex/Claude runner, Orca Omo 복구 거부,
설치 matrix, Omo·hook·gitworktree·lease·completion 격리 테스트가 통과했다.
이는 기존 보호의 근거이며 H1~H5가 구현·live 검증됐다는 증거는 아니다.

### 외부·로컬 근거

조회일: 2026-09-19. 설치 CLI와 공개 공식 문서가 다르면 실제 실행에서는 설치 버전의
help/capability를 따른다. 공개 문서는 지원 개념의 참고이며 버전 차이를 숨기지 않는다.

- `orca skills get orca-cli`, `orca terminal send --help`, `orca status --json`:
  accepted와 turn_started 분리, bounded wait, retry-request의 payload/process 결합을 확인했다.
- `herdr --version`, `herdr status`, `herdr --skill`, agent start/prompt·pane run help와
  [Herdr agent automation](https://herdr.dev/docs/agent-automation/): wait는 turn이 아닌
  상태 관측이며 working 중 prompt의 완료 증거로 사용할 수 없음을 확인했다.
- [Herdr CLI](https://herdr.dev/docs/cli-reference/): 기존 worktree 열기와 서버·프로토콜 상태.
- cmux 번들 CLI help/strings/Info.plist, ping/capabilities와
  [cmux CLI](https://cmux.com/docs/api),
  [CLI contract source](https://github.com/manaflow-ai/cmux/blob/main/docs/cli-contract.md):
  exact 대상 지정과 입력 전달의 범위. 공개 문서의 socket 경로와 설치 CLI 기본 경로가 달랐으며 둘 다 확인했다.
- 설치된 codex/claude/omo의 version/help와
  [Codex CLI](https://developers.openai.com/codex/cli/reference),
  [Claude CLI](https://code.claude.com/docs/en/cli-reference): native session 선택 방법을 구분한다.
  `--last`/`--continue`로 임의 최근 세션을 붙이지 않고 확인한 exact session만 재개한다.
- 로컬 원본: `/tmp/issueops-handoff-orca-guide.txt`, `issueops-handoff-orca-status.json`,
  `issueops-handoff-herdr-guide.txt`, `issueops-handoff-herdr-status.json`, 각 host help,
  `issueops-handoff-cmux-help.txt`. 모두 `/tmp/`이며 secret·기존 session transcript는 읽지 않았다.

### 인계 조사·계획 완료 기준

- [x] 메인·수신자·런처·native host·IssueOps 권한 경계를 구분했다.
- [x] 설치/기능/기존 테스트/live 실행 증거를 분리한 조합표를 작성했다.
- [x] 병렬 작업 정리, 늦은 결과, 중복 전달, 동시 claim, 취소·재시작 시나리오를 계획에 넣었다.
- [x] H0~H5 독립 계획 리뷰 pass. 모드별 복구 체인 보완 후 참조·docs·diff 검사 통과.
- 이번 단계의 완료는 조사·계획 작성이다. 외부 세션 실제 인계와 H 작업 구현은 별도 실행 단계다.


### 인계 계획 독립 리뷰 결과

첫 판정 revise의 지적은 release 후 곧바로 claim하는 순서와 Orca execution의 별도
세션 생성 가능성이었다. D 이후 direct의 exact replace 복구 체인과 현재 generation 확인,
Orca execution의 core resume·봉인 owner 경로를 분리했고 H0/H2 실패 QA에 반영했다.
수정 후 delta 리뷰는 proceed/pass이며 남은 필수 결함은 없다.

H0~H5는 계획 상태다. 실제 런처/host 인계 성능과 9개 조합의 live 성공을 이번 조사로
인증하지 않았다. 이번에 검증한 것은 설치 도구의 read-only 기능·상태, 기존 코드와 격리
테스트, 인계 계획의 순서·경쟁/복구 조건이다. 런타임·스킬 구현은 이 추가 요청에서 바꾸지 않았다.

## 최종 결과와 채택 판정

### 판정 전제

최종 판정은 보존된 P0 manifest, Task 0~13 보고서, 각 후보의 독립 리뷰와 Task 14의
격리 검증을 다시 대조한 결과다. 서로 다른 명령, 표본 계획, revision, 환경의 백분율은
합산하지 않았다. 과거 자료가 불완전하면 보존하되 채택 수치에서 제외했다. 설치 확인,
deterministic mock, runtime 관측, live 실행은 서로 다른 증거 등급으로 유지했다.

### 성능 후보와 계약 후보

아래 표의 P0 비교는 Task 14에서 새 benchmark를 실행한 결과가 아니다. 보존된 최종
manifest를 `scripts/measure_efficiency.py compare`로 다시 검증했으며, P0/P8/P1/P2/P3/
P4/P5의 채택 쌍은 모두 `ok=true`, `comparable=true`, `contract_equal=true`,
`drifts=[]`를 반환했다.

| 후보 | 최종 commit과 근거 | 같은 비교 안의 관측값 | 최종 판정과 독립 리뷰 |
|---|---|---|---|
| P0 | `02cc8ed4`, `bfa7d6a9`, `54a92d80`, `6699371e`; `p0-final-schema-001-baseline` ↔ `002-candidate` | wall 1.500685583초→1.132806875초, package 0.482초→0.203초 | **ADOPT-AS-CORRECTNESS-ONLY**. 최종 schema의 수집·비교와 시간 비의존 계약만 입증했다. 속도 주장은 없다. 세 차례 수정 뒤 독립 리뷰가 승인했다. |
| P8 | `8dd368e8`; preplanned hostprobe A/B 두 쌍 | A wall 29.924738초→26.199218초(-12.4%), package 29.345초→25.650초(-12.6%). B wall 26.754056초→25.274850초(-5.5%), package 26.170초→24.654초(-5.8%) | **ADOPT**. 일반 fixture Python 호출의 rewrite subprocess를 없앴다. production probe는 바뀌지 않았다. 일반 p95 주장은 없으며 독립 리뷰가 승인했다. |
| P1 | `61db492b`; N=1000/U=10 고정 benchmark | normalization 1001→11회, median 1.558110ms→0.2788135ms(-82.1%), p95 1.683155ms→0.372141ms(-77.9%), capture wall 7.258161초→1.932091초 | **ADOPT**. +3 alloc/op, +607.5 B/op, max RSS +1.35%를 수용했다. U=N에서는 호출 감소가 없다. 독립 리뷰가 승인했다. |
| P2 | `8e497780`; architecture package와 full-suite pair | `go list` 25→2회. package wall 9.724164초→2.045303초(-79.0%), elapsed 9.108초→1.806초(-80.2%). full wall 153.272060초→126.545508초(-17.4%), package 합 1321.028초→969.788초(-26.6%) | **ADOPT**. full max-child RSS +0.63%(+2.51MB)는 함께 기록한다. 한 번의 전체 run 밖으로 일반화하지 않으며 독립 리뷰가 승인했다. |
| P3 | `2c9d023a`; clean-commit installcli package pair | 실제 build 14→1회, package wall 10.670001초→4.478903초(-58.0%), elapsed 9.722초→3.425초(-64.8%), max RSS -3.8% | **ADOPT**. clean-commit package pair만 사용한다. 불완전 fingerprint의 full candidates는 거부했으므로 전체 suite 속도 주장은 없다. 증거 수정 뒤 독립 리뷰가 승인했다. |
| P4 | `16baaa44`, `a6c9c263`, `92518bf4`; Stage 1과 repaired Stage 2 pair | Stage 1 Git 18→15회, median -10.2%, p95 -13.7%, wall 6.756초→5.673초. Stage 2 Git 25→21회, median 308.383ms→256.722ms(-16.8%), p95 346.041ms→291.351ms(-15.8%), wall 11.903329초→9.955572초(-16.4%) | **ADOPT**. Stage 2는 metrics/result/git log가 receipt digest에 묶였다. strict fetch 뒤 schema path는 다시 읽는다. 수정 뒤 독립 리뷰가 승인했다. |
| P5 | `d529069f`, `2307dc89`; final review-fix pair와 12-request table | 누락 0→0, 불필요 문서 0→0, 읽은 bytes 711,848→711,848. 단일 wall 0.738874291초→1.147330458초(+55.28%) | **ADOPT-AS-CORRECTNESS-ONLY**. token은 측정하지 않았다. 단일 wall 표본으로 속도 개선이나 회귀를 주장하지 않는다. compound-routing 수정 뒤 독립 리뷰가 승인했다. |
| P6 | `2ef8c791`; testing/self-verification 계약과 실제 step mapping | 최종 전체 `go test` 소유자를 self-verify 한 번으로 정리하고, 포함되지 않은 vet/race는 같은 배터리에서 별도로 유지했다. | **ADOPT-AS-CORRECTNESS-ONLY**. 실행기나 영속 cache를 추가하지 않았고 속도 주장은 없다. 독립 리뷰가 승인했다. |

### 인계·호스트 안전 변경

H0~H5는 성능 후보가 아니다. 계약 테스트, 실제 wiring, deterministic mock과 독립 리뷰에
근거해 모두 **ADOPT-AS-CORRECTNESS-ONLY**로 판정한다.

| 범위 | commit과 채택 근거 | 비주장과 rollback 단위 |
|---|---|---|
| H0 | `03c90d13`, `483a1f3c`, `f98ab8f3`, `ec9de5ea`; 12-cell matrix, 6개 directed cross-host 자료 전달, exact failure set, 최종 리뷰 승인 | live E2E 성공을 뜻하지 않는다. capability contract/fixture 커밋 묶음으로 되돌릴 수 있다. |
| H1/H3 | `d49c9fe7`, `50d586dc`, `40eccf2a`, `b1bafe5f`; drain·release·receive·freshness를 순수 decision contract와 skill 경계로 검증하고 최종 리뷰 승인 | 새 process manager나 ownership authority가 아니다. 네 커밋 묶음이 rollback 단위다. |
| H2 | `e62b1e4b`부터 `f62c8e93`까지의 delivery observation/recovery 수리; production wiring, receipt/identity/crash·retry fencing, 최종 fresh 리뷰 승인 | observation은 권한이 아니며 claim CAS만 작업 권한을 부여한다. H2 commit 범위를 독립적으로 되돌릴 수 있다. |
| H4 | `09473153`, `88f7702d`, `880b477c`, `35096f1c`, `7c7e7ccc`; native Omo runner와 `goja` mock-pi, resume evidence validator, 최종 fresh 리뷰 승인 | 설치/mock/runtime 증거로 live Omo를 인증하지 않는다. live 상태는 `not-run`이다. H4 묶음이 rollback 단위다. |
| H5 | `434e7282`, `df68e43d`, `dd322b48`; explicit cmux CLI, exact target/path/prompt/UID/revalidation fencing, 최종 fresh 리뷰 승인 | 자동 선택은 Orca→Herdr→현재 세션이며 cmux는 명시적 선택에만 사용한다. socket은 disconnected이고 live 행은 인증하지 않았다. H5 묶음이 rollback 단위다. |

### 적대적 리뷰 fixture

production 문서를 훼손하지 않고 ignored 임시 packet 두 개를 같은 revision과 같은
`side-effect` lens로 만들었다. seeded packet에는 필수 인증 공백을 advisory로 낮추고,
검증 배터리가 끝나기 전에 pass를 기록하며, 실패 뒤에도 그 pass를 유지하는 결함을 넣었다.
clean packet은 현재 계약대로 최초 revise, 공백 해소, fresh delta review, 배터리와 QA 성공,
최종 fingerprint 일치 뒤에만 pass를 기록한다.

기대 답을 알려주지 않은 fresh isolated `gpt-5.6-sol`/max reviewer를 A 다음 B 순서로
실행했다. seeded verdict는 `revise`였고 의도한 세 결함을 모두 찾았다. clean verdict는
`pass`였으며 필수 finding은 없었다. 결과는 seeded detection `1/1`, clean false positive
`0`, false pass `0`이다. 이 한 쌍은 SHA-256으로 고정한 합성 packet 두 개만 평가하며
일반적인 리뷰 정확도를 추정하지 않는다. 원문 verdict와 제외한 두 calibration 초안은
`.issueops/tmp/task14/review-results.md`에 남겼다.

### 설치, parity, 최종 검증

tracked 계획은 최종 증거의 acceptance contract를 고정한다. Task 14 결과는 이 단락을
포함한 exact commit에서 물리적 임시 checkout과 mode 0700의 private `HOME`,
`CODEX_HOME`, `ISSUEOPS_STATE_DIR`, daemon dir를 사용해 수집하고, 정확한 revision,
환경, candidate binary digest, 명령별 duration과 raw 상태를 ignored
`.issueops/tmp/task-14-report.md`에 기록한다. 이 보고서의 exact-HEAD 결과가 최종 판정의
authoritative result다.

채택하려면 candidate binary를 해당 revision에서 한 번만 빌드하고 다음 조건을 모두
충족해야 한다.

- install dry-run, update dry-run, candidate install, self-verify에서 PATH 선두 cmux·Omo
  sentinel 호출이 각각 0이어야 한다. 이는 기본 경로의 비호출 증거이며 live 실행
  증거가 아니다.
- 공용 `skills/` 원본과 Codex·Claude·Omo·agy 링크, command/MCP 설정, Claude lifecycle
  설정, Omo extension이 candidate root/binary/digest에 묶여야 한다. 기본 install은 대상
  repository에 project-local 파일을 만들면 안 된다.
- deterministic fixture/mock의 CLI·MCP·native command/tool/schema/digest/exit 의미가
  같아야 한다. Codex, Claude, Omo live 행은 `not-run`으로 유지한다.
- `gofmt -l $(git ls-files '*.go')`, `git diff --check`, 관련 skill validator가 통과해야 한다.
- self-verify risk tier가 clean docs-only revision에서 실행하지 않는 `go vet ./...`와
  `go test -race ./... -count=1`은 별도로 통과해야 한다. architecture와 두 golden은
  self-verify가 소유한 전체 `go test ./... -count=1` 결과에 포함한다.
- `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`은
  `ok=true`, `termination_eligible=true`, 26/26 step, 최저 goal score 100이어야 한다.

어느 조건이든 실패하거나 tracked 파일·환경이 바뀌면 bundle 전체를 폐기하고 처음부터
다시 실행한다. 위 수치는 exact-HEAD 보고서에서 실제로 관측됐을 때만 Task 14 결과로
인용한다.

### 보류·거부와 한계

- 최종 test sampling, gate 약화, timeout 단축과 `-count=1` 제거는 **REJECT**다.
- 모든 gate를 병렬화하는 scheduler rewrite, 전역 Git/provider/permission cache,
  측정 없는 SQLite writer-lock/WAL 변경은 **PARK**다.
- reviewer model 하향, 성공 기준 생략, 원문 대신 손실 요약 사용은 **REJECT**다.
- 이미 있는 bulk scan과 local/strict readiness 분리를 새 개선처럼 다시 구현하는 방안은
  **REJECT**다.
- P3의 불완전 fingerprint full candidates, P4의 receipt 없는 초기 Stage 2 자료, P5의
  첫 fix candidate, P0의 superseded schema-v1/v2 자료는 **REJECT-AS-EVIDENCE**다.
  감사 추적을 위해 artifact는 삭제하지 않았다.
- 이번 작업은 외부 계정, live Codex·Claude·Omo episode, Orca/Herdr/cmux 실제 인계,
  cmux app/socket/workspace/send를 실행하지 않았다. live latency, token 절감, 일반 p95,
  모든 입력의 무손실, cmux path lookup의 남은 race closure를 주장하지 않는다.

이 판정은 Task 14 candidate다. 최종 독립 리뷰 전에는 자체 승인으로 간주하지 않는다.
