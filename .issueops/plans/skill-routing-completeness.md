# 스킬 라우팅 완결: 라우터 표·단계 본문·고아 스킬 정합

## TL;DR
> **Summary**: 라우터 표가 약속했지만 단계 본문이 부르지 않는 동반 스킬 7건을 맞추고, 파이프라인 안에서 써야 할 고아 스킬 4개(`review-agent-feedback`, `rebase-onto-parent`의 `sync-base` 경로, `ui-ux-craft`·`aside-web-qa`, `requirements-analysis`·`prompt-engineering`)에 조건 분기를 만든다. 코드 변경은 둘뿐이다: strict readiness의 base drift 경고와 `next.review.frontend` 신호.
> **Deliverables**: 스킬 6개 개정(issueops, issueops-create-issue, issueops-plan, issueops-implement, issueops-verify, references/review-feedback.md), 도메인 `PathIsFrontendChange`, `next.review.frontend`, strict readiness `base_advanced` 경고와 `next` 경고 passthrough, ADR·CONVENTIONS·골든
> **Effort**: Medium
> **Parallel**: NO — 단일 세션 직렬. T3·T4가 같은 골든을 재생성한다.
> **Critical Path**: T3 base drift 관측 → T4 frontend 신호 → T5 구현·검증 스킬 → T6 문서

## Context

### Original Request
"issueops의 skills들을 issueops 파이프라인에서 충분히 적절히 사용하도록 가이드 되고 있는지 확인" → 감사 결과 다섯 가지 권고 → "전부 반영하기 위한 구현 계획을 작성".

### 감사에서 확인한 사실 (2026-09-08)
- 스킬 51개 중 파이프라인 스킬 18개. 참조 그래프를 코드로 그려 대조했다.
- 라우터 표(`skills/issueops/SKILL.md:65-71`)가 약속했는데 본문에 없는 것: 1단계 `implementation-planning`, 3단계 `design-review`·`database-design`·`prompt-engineering`, 4단계 `issueops-debugging`·`code-quality-metrics`.
- `references/review-feedback.md:30`이 봇 리뷰 처리를 산문으로 다시 적어 `review-agent-feedback`과 같은 판단을 두 곳이 소유한다. 다만 같은 문서 42-63행의 `타당성:` 답글 형식과 `선택지` 블록은 **사람 리뷰어용**이며 `review-agent-feedback`은 자기 범위 밖으로 둔다(그 스킬 description).
- `sync-base`를 아는 곳은 `rebase-onto-parent`뿐이다. strict readiness의 `upstream_synced`는 feature 브랜치 자신의 upstream과의 ahead/behind다(`issueops_pr_readiness_strict.go:59-64`). base 브랜치가 앞서 나간 사실은 어떤 표면도 관측하지 않는다.
- `ui-ux-craft`·`aside-*`는 어느 단계도 부르지 않는다. 검증 단계는 스키마 파일이 있을 때만 `database-design`을 켜지만(`issueops-verify/SKILL.md:88`) frontend에는 그 대칭이 없다.
- `requirements-analysis`(Kordoc 기획서 분석)와 `prompt-engineering`은 파이프라인 어디서도 호출되지 않는다.
- owner prompt의 required skills(`execution_owner_context.go:148`)는 라우터가 단계 스킬로 안내하므로 설계상 맞다. 범위 밖.

### Design review (1라운드, subagent) 반영
- **F1** 구현 단계의 `next`는 readiness를 부르지 않는다(`service.go`의 phase 분기가 ai-slop-clean·feedback만). `base_advanced` 경고를 구현 시작 게이트가 읽을 수 없다 → 시작 게이트는 `execution sync-base --preview`를 **직접** 돌린다. `next` 경고는 5~8단계 힌트다.
- **F2** `sync-base --apply`는 base를 머지하지만(`execution_sync_base.go:329`) `BranchPrepare.BaseSHA`는 봉인된 채 남고, `diffBaseRef`는 그 SHA를 우선한다(`evidence.go:197-199`). 머지 뒤 변경 집합에 upstream이 바꾼 파일이 전부 들어와 fingerprint가 바뀌고 봉인 셋(`ai_slop_clean`·`implementation_review`·`project_docs_review`)이 stale이 되며, 티어·신호·리뷰어가 upstream의 diff를 이 사이클의 변경으로 본다 → 검증 단계는 **preview만** 한다. apply는 4단계 진입에서만 허용하고 그 결과(변경 집합 확장)를 명시한다.
- **F3** `타당성:`·`선택지` 블록은 사람 리뷰어 답글·보고 형식이라 지우면 정본이 사라진다 → 30행의 봇 산문만 교체하고 두 블록은 제목만 "사람 리뷰어"로 한정해 유지한다.
- **F4** aside 보고서가 worktree 안 미추적 파일로 떨어지면 fingerprint에 들어간다(`evidence.go:151`의 `--untracked-files=all`) → 경로를 ignored 영역이나 worktree 밖으로 고정하고, REQUIREMENTS·INTENT·SCOPE 매핑을 명시한다.
- 비차단 반영: `Signals []string` 대신 `Frontend bool`(값이 하나뿐인 슬라이스는 §2 위반); `.html` 오탐과 `.js`·`src/ui` 누락을 명시; `BaseBranch`가 `refs/heads/`·`origin/` 접두를 가지면 정규화하고 그래도 ref가 없으면 조용히 건너뛴다는 false negative 명시; 경고 passthrough로 `source_misdirect_warnings`·branch mismatch가 5~8단계 `next`에 새로 보인다는 행동 변화 명시; 디버깅 상한을 `verified-execution`의 3x 규칙과 같은 카운터로 묶음; `prompt-engineering` 트리거를 "LLM에게 주는 프롬프트 본문" 변경으로 좁히고 이 계획 자체가 그 절을 가진다.

### Gap Analysis
- **base drift 관측 표면은 둘이고 역할이 다르다.** fetch 없는 strict readiness 경고는 5~8단계 `next` 힌트, `sync-base --preview`(fetch 함, `:214`)가 진실, apply는 봉인이 없는 4단계 진입에서만. 이 모델 하나로 T3·T5를 쓴다.
- **차단 키가 아니라 경고.** `missing`은 건드리지 않아 PR 정책 불변.
- **`next`는 readiness 경고를 버린다**(`classify.go:17-20` `Readiness`에 `Warnings` 없음, `service.go`가 `Ready`·`Missing`만 매핑). passthrough를 뚫는다. 그 결과 fetch 없이 나오는 경고 둘(branch mismatch, `source_misdirect_warnings`)이 5~8단계 `next`에 처음 노출된다. 둘 다 사실이며 유용하다.
- **frontend는 티어가 아니다.** 티어는 위험 순위, frontend는 QA 라우팅 신호. `next.review.frontend` bool로 분리한다. 신호가 둘 이상 생기면 그때 슬라이스로 바꾸는 것이 골든 계약 변경이므로, 그 시점의 결정으로 미룬다.
- **frontend 휴리스틱의 오탐·누락.** 오탐: `.html` 골든·테스트 픽스처(이 저장소의 `skills/aside-*/testdata/*.html`), Go `embed` 템플릿, `public/` 정적 문서. 누락: `.js` React, `src/ui/**/*.ts`, Angular `.component.ts`. 신호는 힌트라 오탐 비용은 `Not Run` 한 줄이고 누락 비용은 QA 미제안이다. 확장자·세그먼트 목록을 계획에 고정하고 두 예시를 테스트에 넣는다.
- **`aside-web-qa`는 네 입력이 필요하다**(`aside-web-qa/SKILL.md:39-61`). 매핑이 없으면 에이전트가 지어낸다. TARGET=계획·이슈 본문의 로컬 실행 절차, SCOPE=frontend 변경 경로, REQUIREMENTS=이슈 성공 기준과 `gates.md`, INTENT=4단계 report에 적은 `ui-ux-craft` 판단. 하나라도 없으면 `Not Run`과 사유.
- **4단계 `code-quality-metrics`는 표의 오류다.** 전후 측정은 5단계 소유(`issueops-clean/SKILL.md:13,103`). 표에서 뺀다.
- **3단계 `design-review`는 간접 호출이다.** 본문은 `issueops-review`를 부른다. 표가 실제 호출 이름을 쓴다.
- **골든 충돌.** T3·T4가 response-contract 골든을 바꾼다. 직렬, 태스크별 재생성.
- **이 계획의 프롬프트 변경 측정 기준**(T6의 `prompt-engineering` 규칙을 이 계획에 먼저 적용): T5가 검증 스킬의 리뷰어 프롬프트 입력(`검증할 주장 목록`·렌즈)에 frontend 신호를 더한다. 실패 판정 기준은 "frontend 신호가 있는 사이클의 diff 리뷰 finding에 `ui-ux-craft` 렌즈(접근성·반응형·모션 감소) 언급이 0건"이며, 첫 frontend 사이클의 `review-metrics`와 리뷰 finding으로 확인한다.

## 적용되는 결정과 주의사항
- `.issueops/adr/2026-09-05-issueops-ten-stage-skills-with-auto-execution-mode.md`: stage 판별과 next 출력은 CLI가 소유한다. 신호·경고는 `next` 출력 필드로만 노출한다.
- `.issueops/adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md`: 변경 집합 분류는 도메인 `change_paths.go`가 소유. 봉인된 변경 집합으로 티어·리뷰를 판정한다 — 그래서 apply의 오염을 검증 단계에서 허용하지 않는다.
- `.issueops/adr/decisions/2026-08-27-session-start-owns-compaction-context.md`: hook 미접촉.
- `.issueops/CONSTITUTION.md:71`: 유한 상한. 디버깅 라우팅은 `verified-execution`의 "같은 기준 3회 실패 → 종료"와 같은 카운터의 2회째 뒤에 끼운다.
- `.issueops/cautions/2026-09-08-review-checks-enter-the-ledger-at-stage-4-entry-never-during.md`: 검증 단계는 파일을 쓰지 않는다. QA 보고서 경로 규칙의 근거.
- `skills/rebase-onto-parent/SKILL.md:31,305`: issueops 브랜치는 rebase하지 않고 `sync-base`로 간다. 이 계획은 그 반대편(파이프라인에서 보내는 문장)을 만든다.
- `skills/issueops/SKILL.md` 공통 불변식 (f): 계획의 네 필수 절.
- `AGENTS.md` §2: 값 하나짜리 슬라이스를 만들지 않는다. §3: 스킬 절 구조를 재편하지 않는다.

## 재사용하는 기존 구현
- `internal/domain/issueops/change_paths.go` `normalizeChangePath`, `pathTouchesAuth`(세그먼트 완전일치) — `PathIsFrontendChange`의 틀.
- `internal/adapter/issueops/issueops_pr_readiness_strict.go:26-66` `issueOpsObservedPRReadiness` — `warnings` 슬라이스 존재.
- `internal/adapter/issueops/issueops_cleanup_finish.go:448` `preparedBaseBranch`.
- `internal/adapter/issueops/execution_sync_base.go:214,244,329` — fetch·merge-base·merge 선례. preview는 actor 권위를 요구하지 않고(`:73-81`) `cwd_canonical`·`remote_branch_present`를 요구한다(`:170,:186-193`).
- `internal/application/issueopsnext/service.go` LocalReadiness 매핑과 `applyReviewTier`(관측 1회, `observed` 분기).
- `internal/domain/issueopsnext/classify.go:17-20,146` `Readiness`, 경고 append.
- `internal/contract/issueopsnext/types.go:73-76` `Review.Tier/Lenses`.
- `skills/review-agent-feedback/SKILL.md:21-116` 봇 스레드 파이프라인과 `판정:` 형식 — 봇 리뷰의 정본.
- `skills/issueops-verify/SKILL.md:88-113` 스키마 조건 분기 — frontend 분기의 문장 틀.

## 성능 영향
- base drift 경고: strict readiness에 `git rev-parse --verify origin/<base>`와 `git merge-base --is-ancestor` 두 번의 로컬 git 읽기 추가. fetch 없음. 검증 단계와 ai-slop-clean·feedback phase의 `next`에서만.
- frontend 신호: `applyReviewTier`가 이미 받은 경로 목록을 한 번 더 순회. git 읽기 추가 없음(`observed` 블록 안에서만 계산).
- 구현 시작 게이트의 `sync-base --preview`: fetch 한 번과 merge-tree. 사이클당 한 번이며 사용자 명령이다.

## 하위 호환성과 side effect
- `next.review`에 `frontend` bool **추가**(omitempty). `next.warnings`에 `base_advanced` 문장이 조건부 추가. 5~8단계 `next.warnings`에 branch mismatch·`source_misdirect_warnings`가 **처음 노출**된다(readiness가 이미 내던 경고의 passthrough).
- strict readiness `warnings` 항목 조건부 추가. `missing` 불변 → PR 게이트 정책 불변.
- record JSON schema 불변.
- 라우터 표: 4단계 `code-quality-metrics` 제거, 3단계 `design-review` → `issueops-review`, 4단계에 `ui-ux-craft`·`rebase-onto-parent`, 7단계에 `aside-web-qa`, 9단계에 `review-agent-feedback`·`pr-review` 추가.
- `review-feedback.md` 30행 교체. 사람 리뷰어 형식 블록 유지.
- 알려진 한계(비차단, 문서화): `BaseBranch`가 `refs/heads/main`·`origin/main` 형태면 `refs/heads/`·`origin/` 접두를 벗기고 비교하되, 그래도 로컬 ref가 없으면(한 번도 fetch 안 한 worktree) 경고 없이 건너뛴다. `sync-base --preview`가 그 공백을 메운다.
- 롤백: 태스크당 커밋 하나.

## Work Objectives

### Core Objective
각 단계가 동반 스킬을 실제 문장으로 부르고, 고아 네 스킬이 조건이 맞을 때 호출되며, 그 조건 중 코드로 관측할 둘(base drift, frontend)을 `next`가 돌려준다. base drift의 세 표면(경고·preview·apply)이 단계별로 하나의 모델을 따른다.

### Definition of Done
- 감사 스크립트 재실행 시 라우터 표의 모든 동반 스킬이 해당 단계 본문에 이름으로 존재하고, 파이프라인 관점 고아 목록에 `review-agent-feedback`·`rebase-onto-parent`·`ui-ux-craft`·`aside-web-qa`·`requirements-analysis`·`prompt-engineering`이 없다.
- `review-feedback.md`에 Kodus·Gemini 분류·검증 산문이 없고 `review-agent-feedback` 호출이 있으며, 사람 리뷰어용 `타당성:`·`선택지` 블록은 남아 있다.
- ai-slop-clean phase 사이클에서 origin/<base>가 앞서 있으면 `pr-readiness --strict --json`과 `next --json`의 `warnings`에 `base_advanced`가 있고 `missing`에는 없다.
- `.tsx`만 바뀐 implement phase 사이클에서 `next --json`의 `review.frontend`가 `true`다.
- 구현 시작 게이트가 `sync-base --preview`를 직접 돌리고, 검증 단계는 preview 기록만 하며 apply를 부르지 않는다.
- `go test ./... -count=1`, gofmt, `git diff --check`, `contract check`, 스킬 검증기, 문서 체커 통과. 골든 재생성 완료.

### Must Have
- 코드 변경은 RED → GREEN. 스킬은 `fluent-korean`.
- 새 조건 분기는 "조건 → 스킬 → 결과를 어디에 남기나" 세 요소.

### Must NOT Have
- base drift를 차단 키로 만들기.
- frontend를 `ChangeTier` 값이나 `Signals` 슬라이스로 만들기.
- 검증 단계에서 `sync-base --apply`, `gates check --write`, 원장 편집, worktree 안 보고서 쓰기.
- owner prompt required skills 변경. 새 CLI 명령. 스킬 절 구조 재편.
- `diffBaseRef`의 봉인 의미 변경(base 이동을 따라가게 만드는 설계는 별도 결정).

## Verification Strategy
> 사람 개입 없이 에이전트가 실행한다.
- Test decision: TDD. 스킬은 `scripts/validate-skill.py`와 `rg`.
- 감사 스크립트를 scratchpad에 두고 T1·T2·T5·T6 뒤 재실행.
- Evidence: `.issueops/evidence/routing-{N}-{slug}.{ext}`.

## Execution Strategy

### Execution Waves (의존 묶음, 실행은 직렬)
- Wave 1: T1, T2 (문서, 독립)
- Wave 2: T3, T4 (코드, 독립, 각자 골든)
- Wave 3: T5 (T3·T4의 출력을 읽는 스킬)
- Wave 4: T6 (조건 분기 둘 + ADR·CONVENTIONS·README·골든)
- Final: F1–F4

### Dependency Matrix

| Task | Depends On | Blocks | 비고 |
|---|---|---|---|
| T1 표↔본문 | — | T6 | 문서 |
| T2 review-feedback | — | T6 | 문서 |
| T3 base drift 경고 | — | T5 | 코드, 골든 |
| T4 frontend 신호 | — | T5 | 코드, 골든 |
| T5 sync-base·frontend 분기 | T3 T4 | T6 | 문서(재작성 수준) |
| T6 분기 둘 + 문서·골든 | T1 T2 T5 | F1 | 문서 |

## TODOs

- [ ] 1. 라우터 표와 1·3·4단계 본문 정합

  **What to do**:
  1. `skills/issueops/SKILL.md:65-71` 표: 3단계 `design-review` → `issueops-review`. 4단계 `code-quality-metrics` 제거. (4·7·9단계의 새 동반 스킬은 T5·T2가 넣는다.)
  2. `skills/issueops-create-issue/SKILL.md` "## 질문 규칙" 끝: blocking 질문이 둘 이상이거나 답에 따라 만들 것이 갈리면 [`implementation-planning`](../implementation-planning/SKILL.md)의 Phase 2 인터뷰 절차(드래프트 파일, 한 번에 한 질문, clearance check)를 쓴다. 드래프트의 "Requirements (confirmed)"가 `--interpreted-intent`의 원문이 된다.
  3. `skills/issueops-plan/SKILL.md:73-75` 표 `## 하위 호환성과 side effect` 행: 계획이 마이그레이션·엔티티·인덱스·쿼리를 바꾸면 [`database-design`](../database-design/SKILL.md)으로 설계를 검토하고 결과(정규화 판단, 인덱스 계획, 예상 row 수)를 이 절에 적는다. 7단계 실측은 이 값과 대조한다.
  4. `skills/issueops-plan/SKILL.md` 네 절 표 뒤에 `prompt-engineering` 분기(LLM에게 주는 프롬프트 본문 변경 시 측정 가능한 출력 기준). 표가 3단계에 그 스킬을 약속하므로 이것도 표↔본문 정합이며 T6가 아니라 여기서 한다.
  5. `skills/issueops-create-issue/SKILL.md` 3항(배경지식과 웹 조사)이 `--web-research-evidence` 플래그만 부르고 [`web-research`](../web-research/SKILL.md) 스킬 이름을 부르지 않았다. 스킬 호출로 바꾼다.
  6. `skills/issueops-implement/SKILL.md` "## 구현 루프" 끝: 같은 focused test가 **두 번** GREEN에 실패하면 세 번째를 추측으로 시도하지 않고 [`issueops-debugging`](../issueops-debugging/SKILL.md)으로 재현·격리·근본 원인을 먼저 잡는다. 이 카운터는 `verified-execution`의 "같은 기준 3회 실패 → 목표 종료"와 **같은 카운터**이며, 세 번째 실패가 그 종료다. 진단은 verified-execution report의 실패 항목에 적는다.
  **Must NOT do**: 절 구조 변경. 4단계 본문에 `code-quality-metrics` 추가.

  **Recommended Agent**: quick

  **Parallelization**: Wave 1 | Blocks: T6 | Blocked By: —

  **References**: `skills/issueops/SKILL.md:65-71`; `skills/issueops-create-issue/SKILL.md:45-62`; `skills/issueops-plan/SKILL.md:70-76`; `skills/issueops-implement/SKILL.md:94-122`; `skills/implementation-planning/SKILL.md` Phase 2; `skills/verified-execution/SKILL.md:297,374,490`; `skills/issueops-debugging/SKILL.md` Step 1 REPRODUCE.

  **Acceptance Criteria**:
  - [ ] 감사 스크립트: 1·3·4단계 "본문누락=없음".
  - [ ] `python3 scripts/validate-skill.py` 네 스킬 exit 0.
  - [ ] `rg -n "code-quality-metrics" skills/issueops/SKILL.md`가 5단계 행 1건. `rg -n "같은 카운터" skills/issueops-implement/SKILL.md` 1건.

  **QA Scenarios**:
  ```
  Scenario: 표와 본문 정합
    Channel: bash
    Steps: 감사 스크립트 실행
    Expected: 10개 단계 행 전부 본문누락=없음
    Evidence: .issueops/evidence/routing-1-table-body.txt

  Scenario: 디버깅 상한이 하나다
    Channel: bash
    Steps: rg -n "두 번|같은 카운터|3회" skills/issueops-implement/SKILL.md
    Expected: 세 표현이 같은 문단에 있고 verified-execution을 가리킨다
    Evidence: .issueops/evidence/routing-1-debug-cap.txt
  ```

  **Commit**: YES | `docs(issueops): align the stage table with the stage bodies` | Files: 스킬 4개

- [ ] 2. review-feedback.md의 봇 리뷰 이중 소유 제거 (사람 리뷰어 형식은 유지)

  **What to do**:
  1. `skills/issueops/references/review-feedback.md:30` 문단(Kodus·Gemini)을 교체: "자동 리뷰어(Kodus, CodeRabbit, Copilot, Gemini Code Assist 등)의 스레드는 [`review-agent-feedback`](../../review-agent-feedback/SKILL.md)이 소유한다. 목록·검증·판정·답글·반응·resolve의 순서와 `판정: 타당` 형식은 그 스킬을 따른다. 그 스킬이 `contract_change`를 발견하면 아래 이슈 본문 갱신 절로 돌아온다."
  2. 42-54행의 `타당성:` 답글 블록과 56-63행의 `선택지` 블록은 **유지**하되, 앞 문장을 "사람 리뷰어의 스레드에 답할 때는 이 형식을 쓴다"로 한정한다. 코드 펜스를 자르지 않는다.
  3. `feedback mark-issue-updated` 절은 그대로.
  4. `skills/issueops/SKILL.md`의 **Reference map** 행 `references/review-feedback.md`에 `review-agent-feedback`(봇 스레드)와 `pr-review`(사람 리뷰 요청)를 적는다. 9단계 "함께 쓰는 스킬" 열이 아니다 — 리뷰 피드백은 발행 뒤 phase이고 그 본문은 `issueops-create-pr`이 아니라 이 reference가 소유하므로, 표에 넣으면 감사가 새 표↔본문 불일치로 잡는다.
  **Must NOT do**: `review-agent-feedback` 본문 수정. 사람 리뷰 문장·블록 삭제.

  **Recommended Agent**: quick

  **Parallelization**: Wave 1 | Blocks: T6 | Blocked By: —

  **References**: `review-feedback.md:28-63`; `skills/review-agent-feedback/SKILL.md:3(description 범위),21-116`; `skills/pr-review/SKILL.md` description.

  **Acceptance Criteria**:
  - [ ] `rg -c "review-agent-feedback" review-feedback.md` ≥ 1(위임 문단 하나가 소유권과 contract_change 복귀를 함께 말하므로 문서를 늘려 2건을 만들지 않는다). `rg -c "Kodus" review-feedback.md`가 새 문장의 1건뿐. `rg -c "타당성: 타당|선택지:" review-feedback.md` = 2(유지).
  - [ ] `rg -n "review-agent-feedback|pr-review" skills/issueops/SKILL.md`가 9단계 행에 각 1건.

  **QA Scenarios**:
  ```
  Scenario: 봇 리뷰 산문 0건, 사람 리뷰 형식 유지
    Channel: bash
    Steps: rg -n "classify it, verify whether it is valid, stale, noisy" review-feedback.md; rg -c "타당성: 타당" review-feedback.md
    Expected: 첫 rg 0건, 둘째 1건
    Evidence: .issueops/evidence/routing-2-single-owner.txt

  Scenario: 계약 변경 join point 유지
    Channel: bash
    Steps: rg -n "feedback mark-issue-updated" review-feedback.md
    Expected: 1건 이상
    Evidence: .issueops/evidence/routing-2-join-point.txt
  ```

  **Commit**: YES | `docs(issueops): route bot review threads to review-agent-feedback` | Files: review-feedback.md, skills/issueops/SKILL.md

- [ ] 3. base drift 경고와 `next` 경고 passthrough

  **What to do**:
  1. `issueops_pr_readiness_strict.go` `issueOpsObservedPRReadiness`: `upstream` 판정 뒤에 base drift 관측. `base := preparedBaseBranch(record)`에서 `refs/heads/`·`origin/` 접두를 벗긴다. 비면 건너뛴다. `git rev-parse --verify origin/<base>^{commit}`가 실패하면 건너뛴다(로컬 ref 없음, false negative를 주석에 명시). `git merge-base --is-ancestor origin/<base> HEAD`가 0이 아니면 `warnings`에 `base_advanced: origin/<base> is not an ancestor of HEAD; run issueops execution sync-base --id <ID> --preview` 추가. `missing` 불변. fetch 없음.
  2. `classify.go` `Readiness`에 `Warnings []string`. `Classify`가 `in.Local != nil`이면 `in.Local.Warnings`를 `decision.Warnings`에 append(146행 근처).
  3. `service.go` LocalReadiness 매핑에 `Warnings: readiness.Warnings`.
  4. response-contract 골든 재생성(드리프트 없으면 그대로).
  **Must NOT do**: `missing` 추가. fetch. `upstream_synced` 의미 변경. owner_command 변경.

  **Recommended Agent**: deep

  **Parallelization**: Wave 2 | Blocks: T5 | Blocked By: —

  **References**: `issueops_pr_readiness_strict.go:26-66`; `issueops_cleanup_finish.go:448`; `execution_sync_base.go:244`; `classify.go:17-20,146`; `service.go` LocalReadiness 매핑; 픽스처 `issueops_readiness_test.go:103-166`(`initIssueOpsRepo`, 원격 있음).

  **Acceptance Criteria**:
  - [ ] RED→GREEN `go test ./internal/adapter/issueops/ -run 'BaseAdvanced' -v`: (a) origin/main에 커밋을 더한 worktree → `warnings`에 `base_advanced`, `missing`에 없음; (b) base가 조상 → 경고 없음; (c) `BranchPrepare == nil` → 관측 없음; (d) `BaseBranch = "refs/heads/main"` → (a)와 같은 경고; (e) `BaseBranch = "nope"`(로컬 ref 없음) → 경고 없음.
  - [ ] `go test ./internal/domain/issueopsnext/ ./internal/application/issueopsnext/ -run 'Warning' -v`: Local.Warnings가 decision.Warnings에 나온다.
  - [ ] 실측(ai-slop-clean phase 픽스처): `pr-readiness --strict --json`과 `next --json` 둘 다 `warnings`에 `base_advanced`.

  **QA Scenarios**:
  ```
  Scenario: base가 앞서 나감
    Channel: bash
    Steps: initIssueOpsRepo 픽스처 → BaseBranch=main으로 branch prepare → origin/main에 커밋 추가 → ai-slop-clean phase → pr-readiness --strict --json; next --id --json
    Expected: 둘 다 warnings에 base_advanced, missing에는 없음, exit 0
    Evidence: .issueops/evidence/routing-3-base-advanced.json

  Scenario: 로컬 ref 없는 base (조용한 false negative)
    Channel: bash
    Steps: BaseBranch를 존재하지 않는 이름으로 둔 record로 pr-readiness --strict --json
    Expected: warnings에 base_advanced 없음, 다른 판정 불변 (문서화된 한계)
    Evidence: .issueops/evidence/routing-3-base-advanced-noref.json
  ```

  **Commit**: YES | `feat(issueops): warn when the prepared base has advanced and pass readiness warnings through next` | Files: strict readiness, classify, service, 테스트, (골든)

- [ ] 4. `next.review.frontend`와 frontend 판정

  **What to do**:
  1. `change_paths.go`: `PathIsFrontendChange(rel) bool` — 확장자 `.tsx .jsx .vue .svelte .astro .css .scss .less .html` 또는 디렉터리 세그먼트 완전일치 `components`, `pages`, `public`, `styles`. `HasFrontendChange(paths) bool`. 티어와 독립.
  2. `types.go` `Review`에 `Frontend bool json:"frontend,omitempty"`. 주석: "QA 라우팅 신호. 위험 순위(tier)와 별개. 신호가 둘 이상 필요해지면 그때 계약을 바꾼다."
  3. `service.go` `applyReviewTier`: `observed` 블록 안에서 `result.Review.Frontend = issueopsdomain.HasFrontendChange(paths)`. 추가 관측 없음.
  4. `issueops_next.go` `review:` 줄에 `frontend=true`를 신호가 있을 때만 덧붙인다.
  5. response-contract 골든 재생성.
  **Must NOT do**: `ChangeTierFrontend`. `Signals` 슬라이스. `schema`를 여기 넣기(`schema_evidence`가 소유).

  **Recommended Agent**: deep

  **Parallelization**: Wave 2 | Blocks: T5 | Blocked By: —

  **References**: `change_paths.go` `normalizeChangePath`·`pathTouchesAuth`; `types.go:73-76`; `service.go` `applyReviewTier`; `issueops_next.go` `review:` 렌더; `review_tier_test.go` 포트 스텁.

  **Acceptance Criteria**:
  - [ ] RED→GREEN `go test ./internal/domain/issueops/ -run 'Frontend' -v`: `app/x.tsx`·`components/Button.go`·`public/index.html` → true; `internal/componentsx/a.go`·`pkg/pageset/b.go`·`README.md` → false; `skills/aside-functional-qa/testdata/client-qa-fixture.html` → true(문서화된 오탐); `src/ui/store.ts`·`app/Home.js` → false(문서화된 누락); 빈 목록 → false.
  - [ ] `go test ./internal/application/issueopsnext/ -run 'Frontend' -v`: `.tsx`만 바뀐 implement 사이클 `review.frontend == true`, 티어 `default`; docs-only 사이클 false; `observed == false`면 false.
  - [ ] 실측: `next --id --json | jq .review.frontend` = true.

  **QA Scenarios**:
  ```
  Scenario: frontend 신호
    Channel: bash
    Steps: src/pages/Home.tsx만 변경한 implement phase 사이클 → next --id --json
    Expected: review.frontend == true, review.tier == "default"
    Evidence: .issueops/evidence/routing-4-frontend-signal.json

  Scenario: 세그먼트 오탐 방지
    Channel: bash (go test)
    Steps: HasFrontendChange([]string{"internal/componentsx/a.go","pkg/pageset/b.go"})
    Expected: false
    Evidence: .issueops/evidence/routing-4-frontend-false-positive.txt
  ```

  **Commit**: YES | `feat(issueops): expose a frontend change flag in next.review` | Files: 도메인, contract, application, next 렌더, 골든

- [ ] 5. 구현·검증 스킬의 base drift·frontend 분기

  **What to do**:
  1. `skills/issueops-implement/SKILL.md` "## 시작 게이트" 끝, **base drift**: `next` 경고에 기대지 않고 시작 게이트에서 `issueops execution sync-base --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json`을 직접 돌린다(구현 단계의 `next`는 readiness를 부르지 않는다). 이 호출은 **claim 뒤, lease가 active(self)인 상태**에서 한다. claimable 상태에서는 `released_completion_authority`로 거부되므로(`execution_sync_base.go:278`) 재개된 Orca 세션도 claim을 먼저 끝낸다. `merge_needed`가 false면 계속. true이고 `conflict_files`가 비면 `--apply --confirm --fingerprint <preview의 fingerprint>`로 반영한다. **이 apply는 4단계 진입, 즉 봉인이 하나도 없을 때만 한다.** 반영하면 변경 집합이 `BranchPrepare.BaseSHA` 기준으로 잡히므로 base가 바꾼 파일이 이 사이클의 diff·티어·리뷰 대상에 포함된다는 것을 계획 `## 하위 호환성과 side effect`에 적는다. 미커밋 tracked 변경이 있으면 apply가 `worktree_clean`으로 거부되므로 먼저 커밋하거나 stash하지 말고 시작 게이트 시점(변경 없음)에 한다. `conflict_files`가 있으면 계획의 영향 범위를 다시 보고 사용자에게 알린다. rebase는 하지 않는다([`rebase-onto-parent`](../rebase-onto-parent/SKILL.md)).
     "## 구현 루프" 끝, **frontend**: `next --json`의 `review.frontend`가 true면 컴포넌트 출처·접근성·반응형·모션 감소는 [`ui-ux-craft`](../ui-ux-craft/SKILL.md) 규칙을 따르고, 그 스킬이 요구하는 디자인 시스템 확인 결과를 verified-execution report의 "UI 판단" 절에 적는다(7단계 QA의 INTENT가 된다).
  2. `skills/issueops-verify/SKILL.md` "## 0 동시에 띄운다" 네 번째 track, **frontend QA**: `review.frontend`가 true면 [`aside-web-qa`](../aside-web-qa/SKILL.md)를 동시에 띄운다. 입력 매핑: TARGET=계획·이슈 본문의 로컬 실행 절차가 준 URL, SCOPE=변경 집합의 frontend 경로, REQUIREMENTS=이슈 성공 기준과 `gates.md`, INTENT=4단계 report의 "UI 판단" 절. 넷 중 하나라도 없으면 QA를 `Not Run`으로 report에 사유와 함께 적고 지어내지 않는다. 보고서 출력 경로는 ignored 영역 `.issueops/issues/<n>/review/`(review-agent-feedback이 쓰는 곳)이나 worktree 밖으로 고정한다 — worktree 안 미추적 파일은 fingerprint에 들어가 봉인을 깬다. 제품을 바꾸는 시나리오(로그인 상태 변경, 데이터 생성)는 aside의 "allowed mutations and cleanup contract"로 한정하고 그 정리 영수증을 report에 적는다. 배터리가 실패하면 QA 결과도 버린다.
     "## 3 구현 리뷰", **frontend 렌즈**: `review.frontend`가 true면 diff 리뷰 프롬프트의 "검증할 주장 목록"에 4단계 report의 "UI 판단" 절을 넣고, 렌즈 목록에 `ui-ux-craft`의 접근성·반응형·모션 감소 세 항목을 덧붙인다. 이것이 Gap Analysis의 측정 기준(frontend 사이클의 리뷰 finding에 그 렌즈 언급이 0건이면 실패)이 가리키는 변경이다.
     "## 4 호환성 재확인과 readiness", **base drift**: strict readiness `warnings`의 `base_advanced`는 차단이 아니다. 이 단계에서는 `sync-base --preview`로 충돌 유무만 report에 기록하고 **apply하지 않는다** — apply는 봉인된 변경 집합에 base의 파일을 끌어들여 5단계부터 다시 밟게 만든다. 머지는 PR 병합 시점에 provider가 한다(PR 정책 불변). 충돌이 있으면 그 사실을 PR 본문 "위험" 절에 적는다.
  3. `skills/issueops/SKILL.md:65-71` 표: 4단계 `ui-ux-craft`(frontend), `rebase-onto-parent`(base drift 사유), 7단계 `aside-web-qa`(frontend).
  **Must NOT do**: URL 추측. `aside-*` 서브스킬 직접 호출. 검증 단계 `sync-base --apply`. worktree 안 보고서.

  **Recommended Agent**: deep — 봉인·병렬·차단 정책과 맞물린 재작성 수준의 문서 변경.

  **Parallelization**: Wave 3 | Blocks: T6 | Blocked By: T3 T4

  **References**: `skills/issueops-implement/SKILL.md:34-67,94-122`; `skills/issueops-verify/SKILL.md:38-57,137-154`; `skills/aside-web-qa/SKILL.md:39-61,62-80`; `skills/ui-ux-craft/SKILL.md`; `skills/rebase-onto-parent/SKILL.md:25-36`; `execution_sync_base.go:73-81(preview 권위),170(cwd_canonical),186-193(remote_branch_present),203-209(worktree_clean),214(fetch),244-265(merge_needed/conflict_files),329(merge)`; `evidence.go:193-212(diffBaseRef)`; `.issueops/cautions/2026-09-08-review-checks-*.md`.

  **Acceptance Criteria**:
  - [ ] 스킬 검증기 세 파일 exit 0.
  - [ ] `rg -n "sync-base --preview" skills/issueops-implement/SKILL.md skills/issueops-verify/SKILL.md` 각 1건 이상; `rg -n "apply하지 않는다|apply를 부르지 않" skills/issueops-verify/SKILL.md` 1건 이상; `rg -n "aside-web-qa|Not Run|TARGET|INTENT" skills/issueops-verify/SKILL.md` 각 1건 이상; `rg -n "ui-ux-craft" skills/issueops-implement/SKILL.md` 1건 이상.
  - [ ] `rg -n "sync-base --apply" skills/issueops-verify/SKILL.md` 0건. `rg -n "next.*base_advanced" skills/issueops-implement/SKILL.md` 0건(구현 단계는 `next` 경고에 기대지 않는다). `rg -n "UI 판단" skills/issueops-verify/SKILL.md`가 "## 3 구현 리뷰"와 QA track 두 곳(2건 이상).

  **QA Scenarios**:
  ```
  Scenario: 검증 단계는 preview만
    Channel: bash
    Steps: rg -n "sync-base" skills/issueops-verify/SKILL.md
    Expected: 모든 매치가 --preview이고 --apply 0건
    Evidence: .issueops/evidence/routing-5-verify-preview-only.txt

  Scenario: QA 입력 매핑과 Not Run
    Channel: bash
    Steps: rg -n "TARGET=|SCOPE=|REQUIREMENTS=|INTENT=|Not Run|review/" skills/issueops-verify/SKILL.md
    Expected: 여섯 표현 전부 존재
    Evidence: .issueops/evidence/routing-5-qa-inputs.txt
  ```

  **Commit**: YES | `docs(issueops): route base drift through sync-base preview and frontend changes to UI craft and browser QA` | Files: 스킬 3개

- [ ] 6. requirements-analysis·prompt-engineering 분기, ADR·CONVENTIONS·README·골든 최종

  **What to do**:
  1. `skills/issueops-create-issue/SKILL.md` "## 입력 세 가지" 1항 뒤: 사용자가 HWP·PDF·DOCX 기획서나 화면 캡처를 줬으면 [`requirements-analysis`](../requirements-analysis/SKILL.md)로 요구사항·모순·누락을 추출한다. 원문 요청은 그대로 `--raw-request`, 추출한 제약은 `intent record --constraint`, 모순·누락은 `--ambiguity`에 넣는다.
  3. `issueops project append --kind adr`: 제목 "Pipeline skill routing: companion skills called by name, base drift as a three-surface model, frontend as a next flag". context에 감사 결과, decision에 T1–T5와 base drift 3표면 모델(경고=5~8단계 힌트, preview=진실, apply=4단계 진입만), 기각: base drift 차단 키(PR 정책), frontend 티어(순위 오염), `Signals` 슬라이스(값 하나), `upstream_synced`→sync-base(의미 불일치), 검증 단계 apply(봉인 파괴·변경 집합 오염, `evidence.go:197`), owner prompt required skills 확장, `diffBaseRef`의 봉인 의미 변경(별도 결정으로 미룸). `ADR.md` 색인.
  4. `.issueops/CONVENTIONS.md` `next.review` 항목에 `frontend`와 `warnings`의 `base_advanced`·passthrough 한 줄씩.
  5. `README.md`·`README.en.md` 스킬 절 "UI/UX와 브라우저 QA"에 "frontend 신호가 있는 사이클의 4·7단계에서 호출" 구절.
  6. 골든 재생성, `go test ./... -count=1`, gofmt, `git diff --check`, `contract check`, `docs`, 문서 체커, 감사 스크립트 최종 재실행.
  **Must NOT do**: ADR 기존 항목 수정. README 수치 표.

  **Recommended Agent**: quick

  **Parallelization**: Wave 4 | Blocks: F1 | Blocked By: T1 T2 T5

  **References**: `skills/issueops-create-issue/SKILL.md:30-44`; `skills/issueops-plan/SKILL.md:62-80`; `usage.golden.txt:29-30`(`--constraint`·`--ambiguity`); `.issueops/adr/2026-09-08-adversarial-review-throughput-*.md` 형식; `.issueops/CONVENTIONS.md` `next.review` 항목.

  **Acceptance Criteria**:
  - [ ] 감사 스크립트 고아 목록에 여섯 스킬이 없다.
  - [ ] `ls .issueops/adr/ | rg "pipeline-skill-routing"` 1건, `rg -c "Pipeline skill routing" .issueops/ADR.md` 1건.
  - [ ] 6번 검증 명령 전부 exit 0.

  **QA Scenarios**:
  ```
  Scenario: 고아 해소
    Channel: bash
    Steps: 감사 스크립트 실행
    Expected: 파이프라인 관점 고아 목록에 review-agent-feedback, rebase-onto-parent, ui-ux-craft, aside-web-qa, requirements-analysis, prompt-engineering 없음
    Evidence: .issueops/evidence/routing-6-orphans.txt

  Scenario: 골든 드리프트 없음
    Channel: bash
    Steps: go test ./cmd/issueops/issueopsapp ./cmd/issueops/contractgolden -count=1
    Expected: ok
    Evidence: .issueops/evidence/routing-6-goldens.txt
  ```

  **Commit**: YES | `docs(issueops): record the skill routing decision and add the last two conditional routes` | Files: 스킬 2개, ADR, CONVENTIONS, README 2개, 골든

## Final Verification Wave
- [ ] F1. Plan Compliance Audit — T1–T6 diff 대조, 커밋 6개 제목 일치.
- [ ] F2. Code Quality Review — `issueops-clean` 절차. `PathIsFrontendChange`·`HasFrontendChange`·`Readiness.Warnings` 각각 프로덕션 호출자 존재.
- [ ] F3. Real Manual QA — 12개 시나리오 evidence 파일 존재.
- [ ] F4. Scope Fidelity Check — `base_advanced`가 `warnings` append에만 있고 `missing`에 없음; `rg "ChangeTierFrontend|Signals" internal/` 0건; `requiredSkills` 목록 불변; 검증 스킬에 `sync-base --apply` 0건; `diffBaseRef` 무변경.

## Commit Strategy
태스크당 커밋 하나, 총 6개. 순서 T1 → T2 → T3 → T4 → T5 → T6. 형식은 `.issueops/COMMIT_POLICY.md`. 골든은 T3·T4가 각자, T6이 `.issueops` 변경분을 마지막으로 재생성한다. 단일 세션 직렬.

## Success Criteria
- 감사 스크립트의 두 표가 파이프라인 관점에서 비어 있다.
- base drift가 세 표면 하나의 모델을 따른다: 5~8단계 `next` 경고는 힌트, `sync-base --preview`가 진실, apply는 4단계 진입에서만이며 그 결과(변경 집합 확장)가 문서에 있다.
- frontend 파일만 바뀐 사이클에서 `next.review.frontend`가 true이고, 구현 단계가 `ui-ux-craft` 규칙을, 검증 단계가 입력 넷이 갖춰졌을 때만 `aside-web-qa`를 동시 track으로 띄운다.
- 봇 리뷰 처리의 정본은 `review-agent-feedback` 하나이고, 사람 리뷰어 답글·보고 형식은 `review-feedback.md`에 남아 있다.
