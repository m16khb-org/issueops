# 적대 리뷰 처리량 개선: 실행 가능한 지적, 티어별 리뷰, 읽기 병렬화

## TL;DR
> **Summary**: 적대 리뷰의 재리뷰 라운드를 "전체 재읽기 LLM 왕복" 대신 "CHECK 실행 결과를 입력으로 받는 delta 리뷰"로 바꾸고, 변경 집합 티어로 리뷰 범위·effort를 정하며, 검증 단계의 읽기 작업과 계획 전 조사를 병렬화한다. 먼저 라운드·판정·단계 소요를 읽는 측정 명령을 만든다.
> **Deliverables**: `issueops review-metrics` 명령, 변경 집합 티어 분류기와 `next.review` 확장, devils-advocate revise 라운드 hard cap, 스킬 4개 개정(issueops-review, issueops-verify, issueops-plan, issueops-create-issue), ADR·CAUTIONS·CONVENTIONS 반영
> **Effort**: Large
> **Parallel**: NO — 단일 세션에서 wave 순서대로 직렬 실행. T1·T6이 서로 다른 골든을 재생성하므로 병렬 에이전트는 골든 충돌을 만든다. wave는 의존 관계 묶음일 뿐이다.
> **Critical Path**: T2 티어 분류기 → T6 next.review 확장 → T7 verify 스킬 → T8 문서

## Context

### Original Request
"더 빠르고 효율적인 적대적 검증이 가능할까", "병렬성을 늘려서 속도를 높일 수도 있을까", "조사한 컨텍스트 기반으로 구현을 위한 구체적인 계획을 수립해줘". 직전 대화에서 Brooks 3회 상한은 유지하되 revise 루프 상한을 hard로 올리고 3라운드에서 리뷰어를 바꾸는 방향에 동의했다.

### Interview Summary
- 쓰기 사슬(5 정리 → 6 문서 → 7 검증)은 fingerprint 봉인 때문에 직렬을 유지한다. 병렬화 대상은 읽기 작업뿐이다.
- 리뷰 비용의 본체는 라운드 수가 아니라 라운드 하나가 전부 "전체 재읽기 LLM 왕복"이라는 점이다. 지적을 CHECK/EXPECT로 받아 호출자가 실행하고, 그 결과만 넣은 delta 리뷰가 판정하면 라운드가 짧아진다. 판정 주체는 언제나 새 컨텍스트 서브에이전트다.
- 리뷰 범위와 effort는 변경 집합에 비례해야 한다. schema 분류기(`pathIsSchemaChange`)가 선례다.
- 측정 없이 최적화하지 않는다. `history`, `regress_events`, `phase_ledger`에 원자료가 이미 있다.

### Design review (1라운드, subagent) 반영
- R1: 라운드 2 판정을 호출자가 기록하는 초안은 인라인 리뷰였다 → 라운드 2도 delta 리뷰 서브에이전트가 판정하고 호출자는 기록만 한다.
- R2: 계획 단계 라운드 2의 `gates check`는 존재하지 않는 원장을 가리켰다(원장 파일은 4단계 진입 `gates init`이 만든다, `skills/issueops-implement/SKILL.md:84-85`) → plan·diff 공통으로 `issueops verify-work --json -- <CHECK>`로 실행한다. 별도 `gates add` 명령은 불필요해 삭제했다. plan 리뷰의 CHECK/EXPECT는 4단계 진입의 단일 `gates init` spec에 얹는다(`internal/adapter/gates/init.go:39-46`이 반복 `--gate`를 받는다).
- R3: cap 오류 문구가 권한 `issueops regress`는 현재 verdict가 `revise`면 거부된다(`issueops_regress.go:69-72`) → 탈출 경로를 "stop 기록 → reflect → regress, 또는 waiver"로 정정했다.
- R4: `next`는 ai-slop-clean·feedback에서만 readiness를 부르고 경로를 돌려주지 않는다(`service.go:141-146`) → "관측 재사용" 주장과 "관측 1회" 수용 기준을 지우고 새 git 읽기임을 명시했다.
- Q7: 골든 재생성이 T1(usage)·T6(response contract, owner prompt)에 걸려 병렬 에이전트 실행이 충돌한다 → 직렬 실행으로 정직하게 바꾸고 각 태스크가 자기 골든을 재생성해 커밋마다 테스트가 통과하게 한다.
- Q8: `"parallel-allowed"` 렌즈 마커는 T7 스킬 문장과 규칙을 이중 소유한다 → 삭제. auth 경로 휴리스틱은 유지하되 오탐 케이스를 표 테스트에 넣는다.

### Design review (2라운드 delta, subagent) 반영
- R5: owner prompt에 티어를 치환하던 T6.6은 prepare 시점에 변경 집합이 비어 언제나 `default`였다 → 삭제하고, 프롬프트에 "검증 시점에 `next --json`의 `.review`를 읽는다" 한 줄만 추가. 리뷰어 effort의 소유자는 `next.review` 하나다.
- 선택 지적: `ChangedPaths` 포트가 nil/빈 슬라이스 구분에 기대지 않도록 `(paths, observed bool)`로 바꿨다.
- 3라운드 T6 delta 판정: proceed. 구현 메모: `changeGitRoot`는 `implementation` 패키지 비공개 함수라 래퍼도 그 패키지에 둔다.

### Gap Analysis
- **원장 발견 규칙**: strict readiness는 `issues/<n>/gates.md`와 `.issueops/gates/<n>-*.md`만 읽고 canonical·legacy 동시 존재를 `duplicate_issue_artifact`로 막는다(`gatesgate/gates_gate.go:54-89`). 리뷰 게이트는 별도 파일이 아니라 4단계 진입의 `gates init` spec에 들어간다.
- **봉인 시점**: 봉인은 ai-slop-clean 전이(`issueops_phase.go:150-152`)와 refresh(`issueops_phase_refresh.go:18`)에서만 찍힌다. 4단계 진입의 `gates init`은 그보다 앞이므로 append가 봉인 뒤에 오는 경로는 없다.
- **병렬 검증의 낭비**: 배터리가 실패하면 동시에 띄운 리뷰 토큰이 버려진다. 정리 단계가 관련 검증을 통과시킨 diff만 검증 단계로 오므로 실패는 예외여야 한다. 실패율은 T1 metrics가 보여 준다. evidence 파일은 `.gitignore:19`로 제외돼 fingerprint에 들어가지 않는다.
- **revise cap과 pass의 관계**: 4번째 기록이 `pass`, `stop`, waived면 허용한다. cap은 "4번째 비-waived `revise`"만 거부한다. regress가 `DevilsAdvocateReview`를 지우므로(`issueops_regress.go:128`) 재계획 뒤 카운트는 0부터다.
- **티어 계산의 위치**: `next`는 읽기 전용이고 이미 git을 읽는다(`ports.go:11-13`, wiring이 git을 읽음). implement 이후 phase에서 `ChangedPaths`를 부르면 git 읽기 두 번(diff, status)이 **추가**된다. 재사용은 없다.
- **스킬과 코드의 경계**: 모델 이름·effort 값은 코드(`port/orca.go`)가 소유한다. 스킬은 `next.review`를 읽기만 한다.
- **contract 표면 변경**: 새 명령 1개(`review-metrics`)와 `next.review` 필드 추가는 usage 골든·commandparse·response contract 골든·owner prompt 골든을 바꾼다. 각 태스크가 자기 골든을 재생성한다.

## 적용되는 결정과 주의사항
- `.issueops/adr/decisions/2026-08-27-session-start-owns-compaction-context.md`: hook은 컨텍스트 전용. 이 계획은 hook을 건드리지 않는다.
- `.issueops/adr/2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md`: stage 판별은 `issueops next`만 소유. 티어는 `next`의 출력 필드로만 노출한다.
- `.issueops/adr/2026-09-02-issueops-publication-evidence-gates-project-doc-reflection-a.md`: 판정은 fingerprint에 묶인다. 리뷰 CHECK는 4단계 진입 시 원장에 들어가고, 검증 단계는 실행만 한다.
- `.issueops/adr/2026-09-08-issueops-project-doc-gates-link-plan-checks-the-four-plan-se.md`: 강제는 CLI 게이트, 절차는 스킬. revise cap은 CLI, 리뷰어 교체 규칙은 스킬.
- `.issueops/SUB_AGENT_PATTERNS.md`: 병렬 서브에이전트는 slug `parallel-independent-research`와 기대 이득 `parallel_speed`를 계획에 적어야 한다. T5·T7이 그 기록을 스킬 본문에 요구한다. 적대 리뷰는 slug `devils-advocate-review`.
- `.issueops/CONSTITUTION.md:71`: 모든 반복은 유한 상한과 종료 경로를 가진다. revise cap의 오류 문구가 실제로 열려 있는 종료 경로를 안내해야 한다.
- `.issueops/CAUTIONS.md` "Update workflow 5": `.issueops/*.md`를 고치면 response-contract 골든을 재생성한다.
- `skills/design-review/SKILL.md` "Subagent-Only Mandate": 판정은 저자 세션이 내리지 않는다. 라운드 2도 예외가 아니다.

## 재사용하는 기존 구현
- `internal/adapter/issueops/implementation/evidence.go` `ChangedPaths`, `ChangeFingerprint` — 티어 분류 입력.
- `internal/adapter/issueops/issueops_schema_evidence.go:102` `pathIsSchemaChange` — 도메인으로 옮겨 티어 분류기가 재사용.
- `internal/adapter/issueops/issueops_regress.go:22-83` regress cap — revise cap의 오류 문구·구조 선례.
- `internal/adapter/issueops/devilsadvocate/devils_advocate.go` `Record`, `roundOf`, `History` — revise cap 삽입 지점.
- `internal/adapter/gates/init.go:39-46` 반복 `--gate` — 리뷰 CHECK/EXPECT를 얹는 지점. 새 명령 없음.
- `internal/application/issueopsnext/service.go:50-53`, `ports.go:22`, `cmd/issueops/issueopsapp/issueops_next_wiring.go:42` — `next.review` 확장 지점.
- `cmd/issueops/issueopscli/issueops_subcommands.go` `runIssueOpsList`(512행 부근) — `review-metrics`의 CLI 패턴.
- `internal/contract/issueops/types.go:303-310` `IssueOpsPhaseLedgerEntry` — 단계 소요 계산 원자료.

## 성능 영향
- `next`가 implement 이후 phase에서 `ChangedPaths`를 위해 `git diff --name-only`와 `git status --porcelain --untracked-files=all`을 추가로 읽는다. 기존 readiness 관측과 별개이며 재사용하지 않는다. `next`는 사용자 호출 명령이라 두 번의 로컬 git 읽기는 허용 범위다. implement 이전 phase에서는 호출하지 않는다.
- `review-metrics`는 record만 읽는다. `--repo` 집계는 사이클 수에 선형이다.

## 하위 호환성과 side effect
- `next.review`에 `tier`, `lenses` 필드가 **추가**된다. 기존 필드 `model`, `effort`의 의미는 유지하되 docs-only 티어에서 `effort`가 낮아진다. 낮추는 값은 코드 상수다.
- `devils-advocate review` 기록이 4번째 비-waived `revise`에서 실패하기 시작한다. 기존 record는 영향 없다(History 길이로만 판정).
- record JSON schema는 바뀌지 않는다(metrics는 파생, 티어는 next 출력). stable v1 shape 테스트 대상 아님.
- 리뷰 CHECK가 4단계 `gates init`에 들어가면 `gates.md`가 변경 집합에 들어가 커밋된다. 원장이 워크트리 안에 있어야 한다는 규칙과 일치한다.
- 롤백: 태스크당 커밋 하나라 `git revert` 단위로 되돌린다. 스킬 개정만 되돌리면 코드 게이트는 남아도 무해하다(cap은 4번째 revise에서만 동작).

## Work Objectives

### Core Objective
검증 단계의 벽시계 시간과 리뷰 라운드당 토큰을 줄이되, 독립 리뷰 원칙(판정은 항상 새 컨텍스트 서브에이전트)과 fingerprint 봉인 규칙을 그대로 둔다.

### Definition of Done
- `issueops review-metrics --repo . --json`이 사이클별 라운드 수·판정 분포·regress 수·단계 소요를 돌려준다.
- `issueops next --json`의 `review`에 `tier`와 `lenses`가 있고, docs-only 변경에서 `effort`가 host 기본값보다 낮다.
- 같은 plan phase에서 비-waived `revise` 3회 뒤 4번째 비-waived `revise` 기록이 거부되고, 오류 문구가 실제로 열려 있는 탈출 경로(stop → reflect → regress, 또는 waiver)를 안내한다. `pass`·`stop`·waived는 허용된다.
- issueops-review·issueops-verify·issueops-plan·issueops-create-issue 스킬이 CHECK/EXPECT 계약, delta 리뷰 라운드, 병렬 규칙을 본문에 가진다.
- `go test ./... -count=1`, `gofmt -l`, `git diff --check`, `issueops contract check --json` 통과. 골든 재생성 완료.

### Must Have
- 모든 코드 변경은 RED → GREEN 순서(AGENTS §4, TESTING.md).
- 새 CLI 명령은 usage 카탈로그·commandparse·usage 골든·`issueops_usage_flag_parity_test`를 통과한다.
- 스킬 본문은 `fluent-korean` 규칙으로 쓴다.

### Must NOT Have
- hook 추가나 hook 동작 변경.
- 코드 fingerprint와 문서 fingerprint 분리.
- 렌즈별 병렬 리뷰를 기본값으로 켜기(schema-auth 티어에서만 선택).
- 인라인 리뷰 허용. 저자 세션이 verdict를 정하는 어떤 문장도 넣지 않는다.
- 스킬 본문에 모델 이름 하드코딩.
- 병렬화 목적으로 lease·worktree 규칙 완화.
- 새 `gates` 하위 명령.

## Verification Strategy
> 사람 개입 없이 에이전트가 실행한다.
- Test decision: TDD. Go `testing`, 기존 fixture 헬퍼(`initIssueOpsRepo`, `planBodyForTest`, `recordIssueOps*ForTest`).
- QA policy: 태스크마다 happy path와 실패 시나리오를 bash로 실행한다.
- Evidence: `.issueops/evidence/task-{N}-{slug}.{ext}` (`.gitignore`로 제외).

## Execution Strategy

### Execution Waves (의존 관계 묶음, 실행은 직렬)
- Wave 1 (독립 기반): T1 review-metrics, T2 티어 분류기(도메인), T3 revise cap, T5 plan-prep fan-out 스킬
- Wave 2 (Wave 1 의존): T4 issueops-review 개정(T3), T6 next.review 확장(T2), T7 issueops-verify 개정(T4·T6)
- Wave 3: T8 문서·ADR·CAUTIONS·골든 최종 확인(전부)
- Final: F1–F4

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With(의존 없음 표시) |
|---|---|---|---|
| T1 review-metrics | — | T8 | T2 T3 T5 |
| T2 티어 분류기 | — | T6 | T1 T3 T5 |
| T3 revise cap | — | T4 | T1 T2 T5 |
| T4 issueops-review 개정 | T3 | T7 | T6 |
| T5 plan-prep fan-out | — | T8 | T1–T3 |
| T6 next.review 확장 | T2 | T7 | T4 |
| T7 issueops-verify 개정 | T4 T6 | T8 | — |
| T8 문서·골든 | T1–T7 | F1 | — |

## TODOs

- [ ] 1. `issueops review-metrics` 읽기 전용 명령

  **What to do**:
  1. `internal/contract/issueops/cli_result_types.go`에 `IssueOpsReviewMetricsResult` 추가: `ok`, `cycles []IssueOpsReviewMetricsCycle`, `aggregate`, `warnings`. cycle 항목은 `id`, `phase`, `devils_advocate_rounds`(1 + len(History), review 없으면 0), `devils_advocate_verdicts []string`(History 순서 + 현재), `regress_count`, `implementation_review_verdict`, `stage_durations map[string]float64`(phase_ledger의 entered_at→completed_at 초 단위, completed 없으면 생략), `review_round_gaps_seconds []float64`(연속 라운드 `recorded_at` 차). aggregate는 `cycles`, `mean_rounds`, `revise_ratio`, `stop_ratio`.
  2. `internal/domain/issueops/review_metrics.go`에 순수 함수 `ReviewMetricsForRecord(record) IssueOpsReviewMetricsCycle`와 `AggregateReviewMetrics([]cycle)`. 시간 파싱 실패는 해당 항목만 건너뛰고 `warnings`에 남긴다.
  3. `internal/adapter/issueops/issueops_review_metrics.go`에 `ReviewMetrics(stateRoot, id, repo string)`: `--id`면 한 record, `--repo`면 `ListIssueOpsCycles`와 같은 필터로 전체. 둘 다 없거나 둘 다 있으면 오류.
  4. CLI: `cmd/issueops/issueopscli/issueops.go` 디스패치에 `"review-metrics"`, `issueops_subcommands.go`에 `runIssueOpsReviewMetrics`(`--id`, `--repo`, `--json`), `issueops_dependencies.go`에 의존 함수. 카탈로그 `internal/domain/cli/issueops_catalog.go`, `internal/domain/commandparse/issueops.go`, `cmd/issueops/testdata/usage.golden.txt` 갱신. 이 태스크가 usage 골든을 재생성한다.
  **Must NOT do**: record를 쓰지 않는다. 새 필드를 record에 저장하지 않는다. `status --json`이 이미 노출하는 원자료(history, regress_events, phase_ledger)를 바꾸지 않는다.

  **Recommended Agent**: deep
    Reason: contract·domain·adapter·CLI 네 층을 함께 만지고 골든이 걸린다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 1 | Blocks: T8 | Blocked By: —

  **References**:
  - Pattern: `cmd/issueops/issueopscli/issueops_subcommands.go` `runIssueOpsList` — 플래그·출력 패턴.
  - Type: `internal/contract/issueops/types.go:257` `History`, `:303-310` `IssueOpsPhaseLedgerEntry`, `:319-325` `IssueOpsRegressEvent`.
  - `internal/contract/issueopsinventory` `ListEntry:22-46` — `--repo` 집계에 필요한 필드가 없어 새 명령이 필요한 근거.
  - Test fixture: `internal/adapter/issueops/issueops_project_docs_review_test.go` `gitRepoWithProjectDocsForTest`.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/domain/issueops/ -run ReviewMetrics -count=1` 통과. History 2개 + 현재 pass인 record에서 `devils_advocate_rounds == 3`, verdicts가 `[revise revise pass]`.
  - [ ] `go test ./cmd/issueops/issueopscli/ ./cmd/issueops/contractgolden/ -count=1` 통과.
  - [ ] `./bin/issueops review-metrics --repo . --json | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['ok']"` 성공.

  **QA Scenarios**:
  ```
  Scenario: 사이클 하나의 라운드와 단계 소요를 읽는다
    Channel: bash
    Steps: 테스트 픽스처로 devils-advocate revise 2회 + pass 1회, regress 1회를 가진 record를 만든 뒤
           ./bin/issueops review-metrics --id "$ID" --json
    Expected: exit 0, cycles[0].devils_advocate_rounds == 3, regress_count == 1, stage_durations.plan > 0
    Evidence: .issueops/evidence/task-1-review-metrics.json

  Scenario: 존재하지 않는 ID
    Channel: bash
    Steps: ./bin/issueops review-metrics --id io-missing --json
    Expected: exit 1, JSON {"ok": false, "error": "..."} (다른 issueops 명령과 같은 오류 형태)
    Evidence: .issueops/evidence/task-1-review-metrics-error.json
  ```

  **Commit**: YES | `feat(issueops): add read-only review-metrics projection` | Files: 위 contract·domain·adapter·CLI·usage 골든

- [ ] 2. 변경 집합 티어 분류기(도메인)

  **What to do**:
  1. `internal/adapter/issueops/issueops_schema_evidence.go:102` `pathIsSchemaChange`를 `internal/domain/issueops/change_paths.go`의 `PathIsSchemaChange`로 옮기고 adapter는 호출만 남긴다. 기존 테스트가 그대로 통과해야 한다.
  2. 같은 파일에 `type ChangeTier string`과 상수 `ChangeTierDocsOnly("docs-only")`, `ChangeTierContract("contract")`, `ChangeTierSchemaAuth("schema-auth")`, `ChangeTierDefault("default")`.
  3. `ClassifyChangeTier(paths []string) ChangeTier`: 빈 목록 → default. 모든 경로가 `.issueops/`, `*.md`, `skills/`, `docs/`, `README*` 중 하나 → docs-only. 하나라도 `PathIsSchemaChange`이거나 **디렉터리 세그먼트가 정확히** `auth`, `authz`, `authn`, `permissions`, `credentials` 중 하나면 → schema-auth(파일명 부분 문자열 일치는 쓰지 않는다). 하나라도 `cmd/issueops/testdata/*.golden*`, `internal/contract/`, `internal/domain/cli/`, `internal/domain/commandparse/`, `configs/`, `*.proto`, `openapi*`, `swagger*` → contract. 우선순위는 schema-auth > contract > docs-only > default.
  4. `ReviewLensesForTier(tier) []string`: docs-only → `["side-effect"]`, contract → `["compat","reuse","perf","side-effect"]`(compat 먼저), schema-auth·default → 네 렌즈. 병렬 허용 마커는 두지 않는다(그 판단은 T7 스킬 문장이 소유).
  **Must NOT do**: adapter에 분류 규칙을 두지 않는다. 대상 저장소 언어를 가정한 경로(예: `src/`)를 넣지 않는다.

  **Recommended Agent**: quick
    Reason: 순수 함수와 표 기반 테스트다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 1 | Blocks: T6 | Blocked By: —

  **References**:
  - Pattern: `internal/domain/issueops/plan_sections.go` — 도메인 규칙 파일 스타일.
  - Move source: `internal/adapter/issueops/issueops_schema_evidence.go:100-120`(의도적으로 뺀 패턴에 대한 주석 포함).
  - Test: `internal/adapter/issueops/issueops_schema_evidence_test.go`.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/domain/issueops/ -run 'ChangeTier|PathIsSchemaChange' -count=1` 통과. 표 테스트에 4개 티어, 우선순위 충돌(`.issueops/CAUTIONS.md` + `migrations/x.sql` → schema-auth), **auth 오탐 방지**(`internal/author/render.go`, `docs/authoring.md`, `pkg/oauthclient/x.go` → schema-auth 아님) 포함.
  - [ ] `go test ./internal/adapter/issueops/ -run 'SchemaEvidence|SchemaChange' -count=1` 통과.

  **QA Scenarios**:
  ```
  Scenario: docs-only 판정
    Channel: bash (go test)
    Steps: ClassifyChangeTier([]string{".issueops/ADR.md","README.md","skills/x/SKILL.md"})
    Expected: "docs-only"
    Evidence: .issueops/evidence/task-2-change-tier.txt

  Scenario: auth 오탐 방지
    Channel: bash (go test)
    Steps: ClassifyChangeTier([]string{"internal/author/render.go","pkg/oauthclient/x.go"})
    Expected: "default" (세그먼트 완전일치만 인정)
    Evidence: .issueops/evidence/task-2-change-tier-auth-false-positive.txt
  ```

  **Commit**: YES | `refactor(issueops): move schema path rule to domain and add change tiers` | Files: 도메인 신규 파일·테스트, adapter 호출부

- [ ] 3. devils-advocate revise 라운드 hard cap

  **What to do**:
  1. `internal/adapter/issueops/devilsadvocate/devils_advocate.go` `Record`에서 verdict 검증 뒤, `prev != nil`일 때 `prev.History`와 `prev` 자신 중 verdict가 `revise`이고 `Waived == false`인 라운드 수를 센다. 그 수가 `reviseRoundCap = 3` 이상이고 새 verdict가 `revise`이며 `Waived == false`면 오류: `revise round cap reached: cycle %s already recorded %d unwaived revise verdicts on this plan phase, so the plan is not converging; record a stop verdict, reflect it with issueops remote reflect-devils-advocate --id %s --confirm, then issueops regress --id %s --reason <TEXT>; or record this round with --waive --waiver-rationale <TEXT>`.
  2. `pass`, `stop`, waived 기록은 cap과 무관하게 허용된다. regress가 review를 지우면(`issueops_regress.go:128`) 카운트도 0부터다.
  3. CLI 도움말 문구는 바꾸지 않는다(플래그 변화 없음).
  **Must NOT do**: History를 잘라내거나 재작성하지 않는다. cap 값을 플래그로 열지 않는다. 오류 문구에 현재 verdict가 `revise`일 때 거부되는 `regress` 직접 호출을 권하지 않는다.

  **Recommended Agent**: quick
    Reason: 함수 하나에 분기 하나와 테스트다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 1 | Blocks: T4 | Blocked By: —

  **References**:
  - Pattern: `internal/adapter/issueops/issueops_regress.go:22-26,79-83` — cap 상수·오류 문구 스타일. `:69-78` — regress가 요구하는 선행 조건(stop + reflect).
  - Insert point: `devils_advocate.go:44-52` (History append 직전).
  - Waiver가 implement 진입을 통과하는 근거: `internal/adapter/issueops/issueops_readiness.go:169`.
  - Test: `internal/adapter/issueops/devilsadvocate/devils_advocate_test.go` 기존 케이스 스타일.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/issueops/devilsadvocate/ -run ReviseRoundCap -count=1` 통과: 비-waived revise 3회 뒤 4번째 revise 거부(오류에 `reflect-devils-advocate`와 `--waive` 둘 다 포함), 4번째 pass 허용, 4번째 stop 허용, 4번째 waived revise 허용, waived revise는 카운트에서 제외.
  - [ ] `go test ./internal/adapter/issueops/ -run 'DevilsAdvocate|Regress' -count=1` 통과.

  **QA Scenarios**:
  ```
  Scenario: 4번째 revise 거부
    Channel: bash
    Steps: 픽스처 사이클에 devils-advocate review --verdict revise를 3회 기록한 뒤 4번째 revise 기록
    Expected: exit 1, error에 "revise round cap reached"와 "reflect-devils-advocate" 포함, record의 history 길이는 2 그대로
    Evidence: .issueops/evidence/task-3-revise-cap-error.json

  Scenario: 4번째 stop 뒤 regress가 실제로 열린다
    Channel: bash
    Steps: 같은 사이클에 --verdict stop 기록 → remote reflect-devils-advocate(테스트 더블) → issueops regress
    Expected: regress exit 0, phase == "grill", devils_advocate_review == null
    Evidence: .issueops/evidence/task-3-revise-cap-exit.json
  ```

  **Commit**: YES | `feat(issueops): cap unwaived devils-advocate revise rounds at three` | Files: devils_advocate.go, 테스트

- [ ] 4. issueops-review 스킬 개정: 실행 가능한 지적, delta 리뷰 라운드, 3라운드 리뷰어 교체

  **What to do** (`skills/issueops-review/SKILL.md`):
  1. "실행" 절의 출력 계약 4항에 추가: blocking finding마다 `CHECK: <read-only 명령> | EXPECT: <문자열>` 한 줄을 요구한다. CHECK를 쓸 수 없는 지적은 blocking으로 세지 않고 "확인 요청"으로 분류해 호출자가 실험을 대신한다(design-review Blocking threshold와 연결).
  2. 새 절 "지적을 실행한다": 호출자는 각 CHECK를 `issueops verify-work --json -- <CHECK>`로 실행하고 결과(exit code, EXPECT 포함 여부)를 finding 옆에 적는다. plan 리뷰에서 살아남은 CHECK/EXPECT는 4단계 진입의 `gates init` spec에 `G(n+1)..`로 얹는다(원장 파일은 그때 처음 만들어지므로 여기서 `gates` 명령을 부르지 않는다). diff 리뷰의 CHECK는 원장에 넣지 않는다(봉인이 바뀜).
  3. "루프 규칙" 개정: 2라운드는 전체를 다시 읽는 리뷰가 아니라 **delta 리뷰**다. 입력은 직전 finding, 각 CHECK의 실행 결과, 대상의 delta(계획이면 diff of plan, 구현이면 수정 diff), 영향받은 계약뿐이다. 판정과 기록 내용은 이 delta 리뷰 서브에이전트가 정하고 호출자는 그 verdict를 그대로 기록한다. 호출자가 verdict를 정하는 경로는 없다. 구조나 범위가 바뀌었거나 CHECK 없는 blocking이 남았으면 전체 리뷰를 다시 띄운다. 3라운드 LLM 리뷰는 `next.review.model`과 다른 모델 또는 한 단계 높은 effort로 띄우고 그 사실을 `--reviewer-model/--reviewer-effort`(diff) 또는 finding 첫 줄(plan)에 적는다.
  4. revise cap 문구: "같은 plan phase의 비-waived `revise`는 3라운드까지다. 4번째는 CLI가 거부한다(`revise round cap reached`). 그때는 `stop`을 기록하고 `remote reflect-devils-advocate --confirm`으로 반영한 뒤 `regress`로 재계획하거나, 근거를 적은 `--waive --waiver-rationale`로 넘어간다. `revise` 상태에서 `regress`를 직접 부르면 거부된다."
  5. 리뷰어 프롬프트에 `next.review.tier`와 `lenses`를 넣고, docs-only 티어는 side effect 렌즈만 적용한다고 적는다(T6 결과 사용).
  6. 나쁜 예 표에 추가: "CHECK가 전부 통과했다는 이유로 호출자가 `pass`를 기록한다 — 판정 없는 기록이다."
  **Must NOT do**: 모델 이름을 적지 않는다. 인라인 리뷰 허용 문구를 넣지 않는다. 기존 나쁜 예를 지우지 않는다.

  **Recommended Agent**: deep
    Reason: 절차의 정합성이 게이트·봉인·독립성 규칙과 맞물린다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 2 | Blocks: T7 | Blocked By: T3

  **References**:
  - 현재 본문: `skills/issueops-review/SKILL.md` "실행", "루프 규칙", "나쁜 예"(139-140행).
  - 임계값과 독립성: `skills/design-review/SKILL.md:20-26` Blocking threshold, "Subagent-Only Mandate", Round policy(delta review).
  - 원장 소유: `skills/issueops-implement/SKILL.md:84-85`, `skills/issueops-plan/SKILL.md:122-125`.
  - 스킬 검증: `python3 scripts/validate-skill.py skills/issueops-review`.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-review` exit 0.
  - [ ] `rg -n "verify-work|delta 리뷰|revise round cap|review.tier|reflect-devils-advocate" skills/issueops-review/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "gpt-|claude-|gates add|호출자가 .*pass를 기록" skills/issueops-review/SKILL.md` — 나쁜 예 표의 한 줄을 제외하고 0건.

  **QA Scenarios**:
  ```
  Scenario: 문서 계약 확인
    Channel: bash
    Steps: 위 rg 세 명령 실행
    Expected: 첫 rg 매치 있음, 둘째 rg는 나쁜 예 행 1건만
    Evidence: .issueops/evidence/task-4-review-skill.txt

  Scenario: 스킬 형식 검증 실패 감지
    Channel: bash
    Steps: frontmatter의 name을 잠시 지우고 validate-skill 실행 후 복구
    Expected: exit 1로 실패가 감지된다
    Evidence: .issueops/evidence/task-4-review-skill-error.txt
  ```

  **Commit**: YES | `docs(issueops-review): make findings executable and judge re-reviews by delta subagent` | Files: skills/issueops-review/SKILL.md

- [ ] 5. plan-prep 조사 fan-out(issueops-create-issue, issueops-plan)

  **What to do**:
  1. `skills/issueops-create-issue/SKILL.md` "입력 세 가지" 뒤에 "조사를 나눠 띄운다" 절 추가: 코드베이스 조사, 관련 이슈 점수, 웹 조사, 선행 결정(ADR·CAUTIONS 대조) 네 항목을 **읽기 전용 서브에이전트 넷에 동시에** 맡긴다. 각 프롬프트에는 질문 한 개, 허용 도구(읽기·검색만), 돌려줄 형식(evidence 문자열 한 단락과 파일:라인 목록)을 넣는다. 메인 에이전트는 결과를 합쳐 `plan-prep record`의 네 evidence로 쓴다. 이 fan-out은 `SUB_AGENT_PATTERNS.md`의 `parallel-independent-research`·`parallel_speed`에 해당하며 그 slug를 이슈 본문 근거 절에 적는다. 항목이 하나뿐이거나 저장소가 작아 조사가 1분 미만이면 나누지 않는다(Brooks).
  2. `skills/issueops-plan/SKILL.md` "프로젝트 문서 확인"에 한 줄: route가 고른 문서를 `project_docs_read`로 **한 번에** 읽는다.
  **Must NOT do**: 서브에이전트에 쓰기 도구를 허용하지 않는다. 질문 규칙(blocking만 묻는다)을 바꾸지 않는다.

  **Recommended Agent**: quick
    Reason: 스킬 텍스트 두 곳의 절 추가다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 1 | Blocks: T8 | Blocked By: —

  **References**:
  - `skills/issueops-create-issue/SKILL.md:30-45`, `:83-85`.
  - `skills/issueops-plan/SKILL.md:39-60`.
  - `.issueops/SUB_AGENT_PATTERNS.md:35-52`.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-create-issue skills/issueops-plan` exit 0.
  - [ ] `rg -n "parallel-independent-research" skills/issueops-create-issue/SKILL.md` 1건 이상.

  **QA Scenarios**:
  ```
  Scenario: 계약 문자열 존재
    Channel: bash
    Steps: 위 rg와 validate-skill 실행
    Expected: exit 0, 매치 1건 이상
    Evidence: .issueops/evidence/task-5-planprep-fanout.txt

  Scenario: 쓰기 도구 허용 문구 부재
    Channel: bash
    Steps: rg -n "쓰기 도구를 허용" skills/issueops-create-issue/SKILL.md
    Expected: 허용을 뜻하는 매치 0건(금지 문구만 존재)
    Evidence: .issueops/evidence/task-5-planprep-fanout-error.txt
  ```

  **Commit**: YES | `docs(issueops-create-issue): fan out plan-prep research to parallel read-only subagents` | Files: 두 SKILL.md

- [ ] 6. `next.review`에 티어·렌즈·티어별 effort

  **What to do**:
  1. `internal/contract/issueopsnext/types.go` `Review`에 `Tier string json:"tier,omitempty"`, `Lenses []string json:"lenses,omitempty"` 추가.
  2. `internal/port/orca.go`에 `IssueOpsReviewEffortForTier(host, tier string) string`: docs-only → `"medium"`(Codex·Claude 공통), 그 외 → `IssueOpsPlannerDefaults`의 effort. 상수로 둔다.
  3. `internal/application/issueopsnext/ports.go`에 `ChangedPaths func(record) (paths []string, observed bool)`와 `ReviewEffortForTier func(host, tier string) string` 포트 추가. `observed`는 git 관측에 성공했는지이며, nil/빈 슬라이스 구분에 기대지 않는다(`evidence.go:35-37`은 비-git에서 nil, 빈 변경 집합에서 빈 슬라이스를 돌려준다). `service.go:50-53`에서 선택된 사이클이 있고 phase가 implement 이후면 `ClassifyChangeTier(paths)`로 tier를, 그 전 phase면 포트를 부르지 않고 `default`를 채운다. `Lenses`는 `ReviewLensesForTier`. `ChangedPaths`는 git을 두 번 읽는 새 관측이며 재사용이 아니다. 포트가 nil이거나 `observed == false`면 `default`와 warning.
  4. `cmd/issueops/issueopsapp/issueops_next_wiring.go:42` 부근에 두 포트 배선: `ChangedPaths`는 `implementation` 패키지 안에 두는 `implementation.ObservedChangedPaths(record) (paths []string, observed bool)`(같은 패키지의 비공개 `changeGitRoot`를 써서 `observed = changeGitRoot(record) != ""`, `evidence.go:41-50`), `ReviewEffortForTier: port.IssueOpsReviewEffortForTier`.
  5. `next`의 텍스트 렌더(`issueops_next.go`의 `stage %d/10` 출력)에 `review: <model> <effort> tier=<tier>` 한 줄 추가.
  6. owner prompt는 티어를 **박지 않는다**. owner 아티팩트는 계획 단계 끝의 `execution prepare`에서 만들어지고(`execution_prepare_bridge.go:137`, `execution_orca_intent.go:170`) 그때 변경 집합은 비어 있어 티어가 언제나 `default`이기 때문이다. `execution_owner_context.go:177,398`은 planner 기본값 그대로 둔다. 대신 `internal/adapter/issueops/testdata/execution_owner_prompt.txt` 147행 부근의 리뷰 지시에 "리뷰어 모델·effort·티어·렌즈는 검증 시점에 `issueops next --id <ID> --json`의 `.review`를 읽어 쓴다" 한 줄을 추가한다. 이 한 줄이 owner prompt 골든을 바꾸므로 그 골든과 response-contract 골든을 이 태스크가 재생성한다.
  **Must NOT do**: 기존 `model`·`effort` 필드를 제거하거나 이름을 바꾸지 않는다. `next`가 파일을 쓰게 하지 않는다. "관측 재사용" 같은 검증 불가 주장을 주석에 적지 않는다. owner prompt에 `REVIEW_TIER` 같은 정적 치환을 넣지 않는다.

  **Recommended Agent**: deep
    Reason: contract·port·application·wiring·골든까지 다섯 표면이 걸린다. 이 계획에서 가장 오래 걸릴 태스크다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 2 | Blocks: T7 | Blocked By: T2

  **References**:
  - `internal/application/issueopsnext/service.go:35-60,141-146`, `ports.go:11-24`.
  - `cmd/issueops/issueopsapp/issueops_next_wiring.go:42,52-53,85-90`.
  - `internal/port/orca.go:13-27`.
  - `internal/adapter/issueops/implementation/evidence.go:33` `ChangedPaths`.
  - `internal/adapter/issueops/testdata/execution_owner_prompt.txt:145-148` (한 줄 추가 위치). `execution_owner_context.go:177,398`은 건드리지 않는다.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/application/issueopsnext/ -run Review -count=1` 통과: docs-only 경로만 바뀐 implement phase 사이클에서 `review.tier == "docs-only"`, `effort == "medium"`, `lenses == ["side-effect"]`; plan phase 사이클에서 `tier == "default"`이고 `ChangedPaths` 포트가 호출되지 않는다(호출 카운트 0); `observed == false`면 `tier == "default"`와 warning 1건.
  - [ ] `go test ./internal/adapter/issueops/ -run 'OwnerPacket|OwnerPrompt' -count=1`와 `go test ./cmd/issueops/issueopsapp/ -run 'Golden|Next' -count=1` 통과(골든 재생성 포함).
  - [ ] `./bin/issueops next --json | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['review'])"`에 `tier` 키가 있다.

  **QA Scenarios**:
  ```
  Scenario: docs-only 사이클
    Channel: bash
    Steps: 픽스처 worktree에서 .issueops/CAUTIONS.md만 수정한 implement phase 사이클에 대해 ./bin/issueops next --id "$ID" --json
    Expected: review.tier == "docs-only", review.effort == "medium", review.lenses == ["side-effect"]
    Evidence: .issueops/evidence/task-6-next-review-tier.json

  Scenario: git이 아닌 worktree
    Channel: bash
    Steps: worktree_path가 일반 디렉터리인 implement phase 사이클에 대해 next 실행
    Expected: exit 0, review.tier == "default", warnings에 변경 집합을 관측할 수 없다는 항목
    Evidence: .issueops/evidence/task-6-next-review-tier-nongit.json
  ```

  **Commit**: YES | `feat(issueops): expose change tier and tiered reviewer effort in next.review` | Files: contract·port·application·wiring·implementation(`ObservedChangedPaths`)·owner prompt 한 줄·골든 2종

- [ ] 7. issueops-verify 스킬 개정: 읽기 작업 동시 실행과 계획 리뷰 주장 전달

  **What to do** (`skills/issueops-verify/SKILL.md`):
  1. "1 원장과 배터리" 앞에 "동시에 띄운다" 절 추가: 봉인된 fingerprint를 확인한 뒤 (a) `gates check`(`--write` 없이)·`verify-work` 배터리, (b) 스키마 실측(활성 시), (c) `issueops-review --target diff` 서브에이전트를 **같은 fingerprint에 대해 동시에** 시작한다. 셋 다 읽기 전용이므로 봉인을 바꾸지 않는다(`--write`는 4·5단계 소유). 배터리가 실패하면 리뷰 결과를 버리고 판정을 기록하지 않은 채 4단계로 돌아간다. 이 규칙의 근거는 정리 단계가 관련 검증을 이미 통과시켰다는 점이며, `review-metrics`의 revise 비율이 높으면 정리 단계를 먼저 고친다. 동시 실행은 `SUB_AGENT_PATTERNS.md`의 `parallel_speed` 기대 이득이며 그 slug를 verified-execution report에 적는다.
  2. "3 구현 리뷰"에 추가: 리뷰어 프롬프트에 `issueops status --json`의 `devils_advocate_review.findings`와 `history`의 finding을 "검증할 주장 목록"으로 넣는다. 리뷰어는 diff 전체 탐색보다 이 주장이 지켜졌는지를 먼저 확인한다. `next.review.tier`가 `schema-auth`이고 `git diff --stat`의 변경 줄 수가 500을 넘으면 렌즈 4개를 서브에이전트 넷으로 나누고 "blocking finding 하나라도 있으면 revise"로 합친다. 그 밖의 티어에서는 나누지 않는다. 이 판단은 이 스킬 문장만 소유한다(코드에 마커 없음).
  3. 리뷰어에게 `next.review.lenses`만 적용하도록 프롬프트 지시를 추가한다.
  **Must NOT do**: 정리·문서 단계와의 동시 실행을 허용하지 않는다. 배터리 실패 시 리뷰 판정 기록을 허용하지 않는다. `gates check --write`를 검증 단계에 넣지 않는다.

  **Recommended Agent**: deep
    Reason: 봉인·재사용·리뷰 규칙이 얽힌 절차 문서다.

  **Parallelization**: Can Parallel: NO(직렬 실행) | Wave 2 | Blocks: T8 | Blocked By: T4 T6

  **References**:
  - `skills/issueops-verify/SKILL.md:40-120`(현재 status/check/--write 구분 49-50행).
  - `skills/issueops-review/SKILL.md` (T4 결과).
  - `skills/gates-ledger/SKILL.md:86-87` `--write` 소유.
  - `.issueops/SUB_AGENT_PATTERNS.md` slug 표.

  **Acceptance Criteria**:
  - [ ] `python3 scripts/validate-skill.py skills/issueops-verify` exit 0.
  - [ ] `rg -n "동시에|review.tier|검증할 주장|리뷰 결과를 버리고" skills/issueops-verify/SKILL.md` 각 1건 이상.
  - [ ] `rg -n "문서 반영과 동시|정리와 동시|check --write" skills/issueops-verify/SKILL.md` 0건.

  **QA Scenarios**:
  ```
  Scenario: 계약 문자열
    Channel: bash
    Steps: 위 rg 두 명령과 validate-skill
    Expected: exit 0, 첫 rg 매치 있음, 둘째 rg 0건
    Evidence: .issueops/evidence/task-7-verify-skill.txt

  Scenario: 렌즈 분할 조건이 스킬에만 있다
    Channel: bash
    Steps: rg -n "parallel-allowed" internal/ skills/
    Expected: 0건
    Evidence: .issueops/evidence/task-7-verify-skill-error.txt
  ```

  **Commit**: YES | `docs(issueops-verify): run read-only verification and review concurrently on the sealed diff` | Files: skills/issueops-verify/SKILL.md

- [ ] 8. 문서 반영과 최종 골든 확인

  **What to do**:
  1. `issueops project append --kind adr`로 결정 기록: 제목 "Adversarial review throughput: executable findings, change tiers, and concurrent read-only verification". context·decision·consequences에 T1–T7 요약, 기각 대안(코드/문서 fingerprint 분리, 렌즈 병렬 기본화, 인라인 리뷰, `gates add` 신설, 렌즈 마커 코드 소유)을 적는다. consequences에 "`review-metrics`는 `--repo` 집계 때문에 새 명령이며 `--id` 단독은 `status --json` 파생으로 충분하다"와 "티어 계산은 implement 이후 phase에서 git 읽기 두 번을 추가한다"를 한 줄씩 남긴다. `ADR.md` 색인 첫 행에 추가.
  2. `issueops project append --kind caution`: "리뷰 CHECK를 검증 단계에서 원장에 넣으면 정리 봉인이 stale이 된다. 원장에는 4단계 진입의 `gates init` spec으로만 넣고, 검증 단계는 `verify-work`로 실행만 한다." 그리고 "`revise` 상태에서 `regress`를 직접 부르면 거부된다. stop 기록 → reflect → regress 순서다."
  3. `.issueops/CONVENTIONS.md` CLI 절에 `review-metrics` 한 줄. `.issueops/AGENT_WORKFLOW.md` "Verify"에 동시 실행 규칙 한 줄과 owner command 목록에 revise cap 안내.
  4. `README.md`·`README.en.md` "운영 문서는 어떻게 강제되는가" 표의 게이트 행에 revise cap 추가, 주요 명령 영역 표에 `review-metrics` 추가.
  5. `go test ./cmd/issueops/issueopsapp -run Golden -update -count=1`로 `.issueops/*.md` 변경분의 response-contract 골든 재생성. `go test ./... -count=1`, `gofmt -l cmd internal`, `git diff --check`, `./bin/issueops contract check --json`, `./bin/issueops docs --json`.
  **Must NOT do**: ADR 기존 항목 수정. README에 수치 표 추가.

  **Recommended Agent**: quick
    Reason: 문서와 골든 재생성이며 판단은 앞 태스크에서 끝났다.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: F1 | Blocked By: T1–T7

  **References**:
  - `.issueops/adr/2026-09-08-issueops-project-doc-gates-link-plan-checks-the-four-plan-se.md` — 형식 선례.
  - `.issueops/ADR.md:36-40` 색인 표.
  - `README.md` "운영 문서는 어떻게 강제되는가", "주요 명령 영역".

  **Acceptance Criteria**:
  - [ ] `rg -n "review-metrics" .issueops/CONVENTIONS.md README.md README.en.md` 각 1건 이상.
  - [ ] `ls .issueops/adr/ | rg "adversarial-review-throughput"` 1건.
  - [ ] 위 5번 검증 명령 전부 exit 0.

  **QA Scenarios**:
  ```
  Scenario: 골든 드리프트 없음
    Channel: bash
    Steps: go test ./cmd/issueops/issueopsapp ./cmd/issueops/contractgolden -count=1
    Expected: ok
    Evidence: .issueops/evidence/task-8-goldens.txt

  Scenario: ADR 색인 누락 감지
    Channel: bash
    Steps: rg -n "Adversarial review throughput" .issueops/ADR.md
    Expected: 1건 (없으면 태스크 미완)
    Evidence: .issueops/evidence/task-8-adr-index.txt
  ```

  **Commit**: YES | `docs(issueops): record the review throughput decision and refresh goldens` | Files: .issueops/*, README*, 골든

## Final Verification Wave
- [ ] F1. Plan Compliance Audit — T1–T8의 What to do가 diff에 그대로 있는가. `git log --oneline -8`의 제목이 Commit 항목과 일치하는가.
- [ ] F2. Code Quality Review — `issueops-clean` 절차로 AI slop 정리, `code-quality-metrics` 전후 비교, 죽은 코드·중복 판정(도메인 vs adapter, 코드 vs 스킬) 없음.
- [ ] F3. Real Manual QA — 각 태스크 QA 시나리오를 실행하고 evidence 파일 존재 확인: `ls .issueops/evidence/task-*`가 16개.
- [ ] F4. Scope Fidelity Check — Must NOT Have 일곱 항목이 diff에 없는가: `rg -n "PostToolUse|PreToolUse" configs/` 0건, fingerprint 분리 코드 없음, 스킬에 모델 이름 없음, `gates add` 없음, `parallel-allowed` 없음.

## Commit Strategy
태스크당 커밋 하나, 총 8개. 순서는 T1 → T2 → T3 → T5 → T4 → T6 → T7 → T8. 형식은 `.issueops/COMMIT_POLICY.md`의 Conventional + Lore. 각 커밋 시점에 `go test ./... -count=1`이 통과해야 하므로 골든은 그 태스크가 재생성한다(T1 usage, T6 owner prompt·response contract, T8 `.issueops` 변경분). 이 저장소에서 IssueOps 사이클로 진행하면 8단계 `atomic-commit-push`가 같은 분할을 쓴다.

## Success Criteria
- 새 사이클에서 `review-metrics`가 라운드·판정·단계 소요를 보여 준다.
- docs-only 변경의 검증 단계가 medium effort 리뷰어와 side effect 렌즈 하나로 끝난다.
- diff 리뷰 2라운드가 전체 재읽기 없이 delta 리뷰 서브에이전트로 종료되는 사례가 metrics에 나타난다(revise 뒤 pass 사이 간격 단축).
- 4번째 비-waived revise가 CLI에서 거부되고, 오류 문구의 탈출 경로가 실제로 실행된다.
- 검증 단계에서 배터리와 리뷰가 같은 fingerprint에 대해 동시에 실행됐다는 기록이 verified-execution report에 남는다.
