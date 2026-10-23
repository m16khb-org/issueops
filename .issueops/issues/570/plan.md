# 계획 단계 앞에 아키텍처 단계를 두고 design 용어를 architecture로 바꾼다 (#570)

## TL;DR
- Summary: plan 단계 안의 하위 단계 순서를 `plan.architecture` → `plan.write` → `plan.review` → `plan.handoff`로 바꾸고, 아키텍처 문서(artifact `architecture`)를 계획보다 먼저 스테이징·검토·승인한다. 승인은 아키텍처 문서 sha256에 묶인다. 같은 변경에서 이 단계와 리뷰를 가리키는 design 이름을 architecture로 바꾼다.
- Deliverables: Go core/CLI 변경, record·CLI 계약 이름 변경, 공용 스킬 `architecture-review`(옛 `design-review`), `issueops`·`issueops-plan`·`issueops-review`·`implementation-planning` 스킬 절차, 현행 문서, golden, 새 ADR.
- Effort: Large
- Parallel: NO (한 세션이 순서대로 수행)
- Critical Path: T1 → T2 → T3 → T4 → T5 → T6 → T7

## 사이클 경계
- lifecycle ID: `io-75f917ecd7a1`, 이슈 #570, 브랜치 `570-plan-architecture-stage`, base `main`.
- 사용자 요청 범위: #570 본문 전체(순서 변경 + 이름 변경 "전체"). 승인된 종료점은 draft PR 발행과 `execution complete`다. merge·cleanup은 별도 승인이다.
- 인계: `execution prepare` 뒤 Herdr를 확인해 같은 worktree의 새 omp 세션으로 인계한다. 모델은 사용자 지시대로 `anthropic/claude-sonnet-5-5`(`issueops model resolve --host omp --role implement --model anthropic/claude-sonnet-5-5 --json`, effort `high`)다. Herdr를 쓸 수 없으면 현재 세션이 이어간다.
- 이 사이클 자체의 lifecycle 명령은 PATH의 `issueops`(source checkout의 `bin/issueops`, 옛 바이너리)로 실행한다. 아래 "하위 호환성과 side effect"의 자기 사이클 주의를 따른다.

## Context
### Original Request
"issueops를 사용할 때도 이렇게 시니어 엔지니어처럼 소프트웨어 설계하는 방식으로 설계 해줄 수 있어?" / "애초에 계획 단계 앞에 설계 단계가 있어야 하지않을까??" / "design이라는 워딩은 변경하면 좋겠어" / 새 이름 architecture, 바꿀 범위 전체.

### Interview Summary
- 새 durable phase 대신 plan 단계 안에서 순서를 바꾼다(이슈 #570 "대안과 선택 이유").
- 과거 기록(날짜 붙은 `.issueops/adr`·`cautions`·`issues`·`plans`·`research`·`verified-execution`·`archive`)과 과거 리뷰 라운드를 인용한 코드 주석(`design-review F3`, `design-review 1차 finding 2` 형식)은 바꾸지 않는다.
- `database-schema-architecture`, `diagram-design`, `ui-ux-craft`, intent class 값 `architecture`, 계획 필수 절 네 개, `.issueops/documentation/owner-design-review.md`(스킬이 아니라 소유권 검토 기록)는 바꾸지 않는다.

### Gap Analysis
- 열린 결정이던 artifact 이름은 새 이름 `architecture`로 정한다. `spec`은 "superpowers 스펙을 이슈에 붙일 때"(`.issueops/CONVENTIONS.md` 이슈 산출물 레이아웃)로 이미 다른 뜻이 있어 재사용하지 않는다.
- 아키텍처 리뷰 기록이 게이트이므로 caution 2026-08-28 규칙대로 digest·출처·이력을 담는다(아래 적용 결정).
- 자기 사이클 위험: 새 바이너리는 `design_review` 키가 있는 이 사이클의 record를 `invalid state`로 거부한다(lease 미러 `internal/contract/issueopslease/record.go:113` `DisallowUnknownFields`). 그래서 lifecycle 명령은 끝까지 옛 바이너리로 실행하고, smoke는 worktree 바이너리와 임시 `ISSUEOPS_STATE_DIR`에서만 한다.

## 적용되는 결정과 주의사항
- `.issueops/CONSTITUTION.md` 제5장: "CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다." → usage·response contract golden을 갱신하고 worktree 바이너리로 계획 단계 smoke를 실행한다.
- `.issueops/ARCHITECTURE.md`: "`internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete adapter나 `cmd/...`를 import하지 않는다." → 단계 판정·절 검사·stale 판정은 domain 순수 함수로 두고, 스테이징 읽기와 digest는 adapter/port로 주입한다.
- `.issueops/CONVENTIONS.md` 이슈 산출물 레이아웃: "이 표가 유일한 규정이다." → 추적 사본 `architecture.md`와 봉인 artifact 이름을 그 표에 추가한다.
- `.issueops/conventions/state-policy-and-hooks.md` State machine reducer contract: "`PlanReadinessMissing`/`GrillReadinessMissing`(`internal/domain/issueops/early_phase_readiness.go`)만 `IssueOpsRecord` 필드만 읽어 `Ready/Missing`을 돌려주며 clock·git·FS·network를 건드리지 않는다." → stale 판정은 이 record-only 함수에 넣지 않고 관측(digest)을 받는 implementation readiness와 next 입력에 둔다.
- `.issueops/CAUTIONS.md` 대조: Universal summary의 "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." → 문서 편집 뒤 golden 재생성. 모듈 `.issueops/cautions/issueops-lifecycle.md`: "골든 재생성은 원인이 된 변경과 같은 커밋에 넣어 red를 남기지 않는다." → 같은 커밋.
- `.issueops/cautions/2026-08-28-devils-advocate-record-was-unbound.md`: "리뷰 판정을 게이트로 쓰려면 기록이 **무엇을 검토했는지**(대상의 content digest), **누가 어떻게**(출처), **이전에 무엇을 냈는지**(이력)를 담아야 한다." → `architecture_review`에 `reviewed_architecture_digest`, `reviewer_context`, `history`를 둔다.
- `.issueops/cautions/2026-09-23-issueopsrecord-field-lease-record.md`: "IssueOpsRecord에 최상위 field를 추가하면 lease Record에도 추가한다." → `design_review`→`architecture_review` 키를 lease 미러와 같은 커밋에서 바꾼다.
- `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md`: "제거 커밋 전에 지운 이름(명령, 플래그, 경로, 디렉터리, MCP tool)으로 git grep을 Go 문자열·usage golden·testdata·skills·.issueops 현행 문서·configs까지 돌린다." 및 "날짜가 붙은 기록 문서는 고치지 않고" → G4 잔여 이름 검사.
- `.issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md`: "스킬 디렉터리를 추가하는 커밋은 `SKILL.md`와 `agents/openai.yaml`을 함께 넣고" → `skills/architecture-review/`를 `git mv`로 옮겨 두 파일을 함께 둔다.
- `.issueops/cautions/2026-10-18-gate-checks-that-run-go-tests-or-self-verify-need-lang-and-t.md`: "Run ledger checks with `--env HOME,PATH,LANG`" 및 "use `--timeout-seconds 900` for full race or self-verify CHECKs." → G7·G8 실행 옵션.
- `.issueops/ADR.md` → `.issueops/adr/2026-06-18-issueops-plan-prep-evidence-gate.md`: "design review는 plan-prep을 전제하지 않는다(plan phase 내부 활동이라 plan 진입 시 이미 강제됨)" → plan phase 내부에 두는 것은 유지하고 순서만 바꾼다. 새 ADR이 이 결정의 순서 부분을 대체한다.
- `.issueops/adr/2026-06-09-expose-issueops-gate-contracts.md`: "New or changed gates must update CLI guidance, MCP schema descriptions, shared skill instructions, and focused contract tests in the same change." → 게이트 키 이름 변경과 새 키는 CLI 도움말·스킬·계약 테스트를 같은 변경에서 갱신한다(MCP에는 design 도구가 없어 schema 변경 없음).
- `.issueops/adr/2026-10-07-readers-accept-only-the-current-record-shape.md`(색인 행 "Readers accept only the current record shape") → 옛 키 읽기 호환 경로를 두지 않는다.
- `.issueops/TESTING.md` → `.issueops/testing/unit-and-contract.md`: "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다." → 이번 변경은 의도적 계약 변경이므로 golden을 `-update`로 재생성하고 diff를 이름·순서 변경으로 한정한다.
- `.issueops/AGENT_WORKFLOW.md`: "구조 선택이나 대안 기각 사유가 생기면 MCP `project_docs_append(kind=adr)`로 `.issueops/ADR.md`에 남긴다." → T7.

## 재사용하는 기존 구현
- 단계 판정: `internal/domain/issueopsnext/classify.go:183-219` `classifyPreparation`의 분기 순서를 바꿔 쓴다. 새 분류 함수를 만들지 않는다.
- 스테이징 관측: `internal/application/issueopsnext/service.go:188-196`이 이미 `ports.StagedArtifacts`로 이름 목록을 읽는다. 포트 반환을 이름→내용 map(`readStagedArtifacts`, `internal/adapter/issueops/issueops_artifact_stage.go:88-105`)으로 바꿔 한 번 읽은 값으로 plan 존재와 architecture digest를 함께 계산한다.
- digest 바인딩: `internal/application/issueopsreview/plan_binding.go`의 `PlanDigestResolver` 패턴을 그대로 따라 `ArchitectureDigestResolver`를 둔다. 실행 전에는 staged `architecture` 내용, 실행 뒤에는 봉인 파일(`sealedArtifactPath(record, root, "architecture")`, `issueops_artifact_stage.go:36-38`)의 sha256이다. digest 함수는 `digestBytes`를 재사용한다.
- stale 판정: `internal/domain/issueopsreview/devils_advocate_readiness.go:23-37`과 같은 모양의 순수 함수로 `architecture_review_stale`을 낸다. delegated child의 부모 승계 예외는 `internal/domain/issueops/delegation.go:90-94`의 기존 판정을 이름만 바꿔 유지한다.
- 필수 절 검사: `internal/domain/issueops/plan_sections.go`의 `MissingPlanSections` 제목 매칭을 재사용해 `MissingArchitectureSections`를 같은 파일 옆에 둔다. 추가로 절 본문이 공백뿐이면 그 제목도 빠진 것으로 보고한다.
- 이력: `internal/contract/issueopsreview/devils_advocate.go`의 `History []DevilsAdvocateRound` 방식대로 재기록 때 직전 기록을 `history`에 append한다.
- artifact materialize: `materializeStagedArtifacts`(`issueops_artifact_stage.go:113-148`)는 이름과 무관하게 모든 staged 항목을 봉인하므로 수정하지 않는다. 이름 허용 목록 `internal/domain/issueopsartifact/artifact.go:14`에 `architecture`만 더한다.
- 추적 사본: `internal/application/issueopsremote/tracked_materials.go:77-100`의 `spec` 처리와 같은 방식으로 `architecture.md`를 더한다.
- 스킬 링크 정리: `internal/adapter/installutil/install_util.go:36-43` `PruneStaleSkillLinks`가 이름이 바뀐 스킬의 옛 링크를 지우므로 설치 코드는 바꾸지 않는다.

## 성능 영향
- hot path: `issueops next`는 사이클마다 staged artifact를 이미 한 번 읽는다(`service.go:188`). 바뀐 뒤에도 sqlite `Get` 한 번이며, 추가 비용은 architecture 내용(최대 `issueopsartifactcontract.MaxBytes`)의 sha256 한 번이다. 복잡도는 O(문서 크기)이고 실행 전 사이클에서만 계산한다.
- implementation readiness: 실행 뒤 digest는 봉인 파일 읽기 한 번이다. 기존 DA의 linked plan digest 읽기와 같은 수준이다.
- 측정: G1·G3 테스트와 smoke에서 `next` 응답 시간을 따로 측정하지 않는다. 추가 I/O가 0회(같은 Get 재사용)와 파일 읽기 1회뿐임을 코드 리뷰로 확인한다.

## 하위 호환성과 side effect
- CLI: `issueops design review`를 지우고 `issueops architecture review`로 대체한다. 플래그 `--proposed-design` → `--proposed-architecture`, 새 플래그 `--reviewer-context subagent|inline`(필수, 감사 필드). 별칭을 두지 않는다. usage golden(`cmd/issueops/testdata/usage.golden.txt`)과 `internal/contract/cli/issueops_catalog.go` 도움말을 함께 고친다.
- next 계약: stage 키 `plan.design` 삭제, `plan.architecture` 추가. missing 키 `design_review*` → `architecture_review*`, 새 키 `architecture_artifact`, `architecture_review_stale`. owner command: `architecture_artifact` → `issueops artifact stage --id ID --name architecture --file <PATH>`, `architecture_*` 접두사 → `issueops architecture review ...`.
- record 계약(schema_version 1 유지): 최상위 키 `design_review` → `architecture_review`. 내부 `proposed_design` → `proposed_architecture`, 새 필드 `reviewer_context`, `reviewed_architecture_digest`, `history[]`. 다른 필드 이름은 유지한다. lease 미러 `internal/contract/issueopslease/record.go:23`과 `TestLeaseRecordCarriesEveryPersistedTopLevelField`를 같은 커밋에서 고친다. `cmd/issueops/contractcli/contract.go:120`의 허용 필드도 바꾼다.
- 기존 데이터: 옛 바이너리가 쓴 record 중 `design_review` 키가 있으면 새 바이너리에서 `invalid state`다. 변환 명령은 만들지 않는다. 작성 시점 `issueops list` 결과는 이 사이클 하나뿐이었다.
- 자기 사이클: 이 사이클(`io-75f917ecd7a1`) record에는 옛 키가 남는다. 구현·검증·발행·완료·정리를 모두 PATH의 옛 바이너리로 실행한다. source checkout의 `bin/issueops`를 다시 빌드하거나 `issueops update`를 실행하는 것은 이 사이클의 cleanup이 끝난 뒤로 미룬다. worktree에서 만든 `bin/issueops`는 임시 `ISSUEOPS_STATE_DIR`와 임시 Git 저장소에서만 실행한다.
- 공용 스킬 이름: `skills/design-review` → `skills/architecture-review`. 사용자 홈의 옛 링크는 다음 `issueops update`/install 때 `PruneStaleSkillLinks`가 정리하고 새 링크를 만든다. 구현 중에는 실제 HOME에 설치하지 않는다.
- 추적 사본: 구현 진입·종료 전이가 `.issueops/issues/<n>/architecture.md`를 새로 쓴다. 봉인 원본은 0600 불변이다.
- golden: `response_contracts.golden.json`(CLI 목록, record 필드, 스킬 목록·설명, 스냅숏 키 `issueops_review_design` → `issueops_review_architecture`), `usage.golden.txt`, `internal/architecture/testdata/ddd_responsibility_inventory.json`(`-update-ddd-inventory`), pioneer holdout manifest의 경로·`evidence_sha256`·`case_sha256`(내용이 바뀐 파일만 `shasum -a 256`으로 재계산). `mcp_tools.golden.json`은 바뀌지 않아야 한다.
- 롤백: PR revert. 새 바이너리가 쓴 `architecture_review` record는 옛 바이너리에서도 `invalid state`가 되므로 revert 뒤에는 그 사이클을 새로 시작한다.
- LLM 프롬프트 본문 변경: `skills/architecture-review/SKILL.md`의 리뷰 렌즈와 `issueops-review`의 리뷰어 프롬프트 구성(새 `--target architecture`)이 바뀐다. 실패 기준: architecture 대상 리뷰 출력에 판정(`pass|revise|stop`)이 없거나, 필수 결함에 위반한 계약(문서 경로:줄 또는 깨지는 입력) 칸이 비어 있으면 실패다. 확인 산출물: 이 사이클 검증 단계에서 새 스킬 본문으로 아키텍처 문서 하나를 실제 서브에이전트에 검토시킨 출력.

## 아키텍처 결정
- 순서(`classifyPreparation`, 실행 전): (1) staged `architecture` 없음 → `plan.architecture`, missing `architecture_artifact`. (2) 리뷰 없음·미승인 → `plan.architecture`, missing `architecture_review`(또는 readiness가 낸 `architecture_*` 키 중 첫 키). (3) 승인 digest ≠ 현재 staged digest → `plan.architecture`, missing `architecture_review_stale`. (4) staged `plan` 없음 → `plan.write`. (5) DA 게이트 → `plan.review`. (6) `plan.handoff`. 계획이 먼저 스테이징돼 있어도 (1)~(3)이 우선한다.
- `issueops architecture review` 기록: staged(실행 전) 또는 봉인(실행 뒤) architecture 문서가 없으면 `architecture review requires a staged architecture artifact`로 거부한다. 필수 절(`## 요구사항과 규모`, `## 데이터와 API 계약`, `## 구조와 흐름`, `## 병목과 장애`, `## 절충점과 기각한 대안`)이 없거나 본문이 비면 `architecture is missing required sections: <제목, ...>`로 거부한다. 통과하면 digest를 `reviewed_architecture_digest`에 저장하고, 이전 기록이 있으면 `history`에 append한다. 승인 증거 문구 판정(`HasArchitectureReviewEvidence`)은 "architecture"+("review"|"audit"|"evaluat") 또는 "아키텍처"+("검수"|"검토")다.
- readiness: `ArchitectureReviewMissing(record)`(record-only, 승인·필드 검사)는 base implementation readiness와 link-plan readiness가 쓴다. `architecture_review_stale`은 관측을 받는 `ImplementationReadinessMissing`(DA stale과 같은 자리)과 next 입력에서만 낸다.
- delegated child: `internal/domain/issueops/delegation.go:90-94`가 부모의 리뷰를 복사해 `Approved = true`로 두는 동작은 유지하고, 복사본의 `reviewer_context`를 부모 승계 표시값 `delegated-parent-review`로 둔다. 이 값은 지금 `internal/domain/issueopsreview/devils_advocate_readiness.go:11`의 `ParentReviewPattern`에 있는데, `internal/domain/issueopsreview`가 `internal/domain/issueops`를 import하므로(`internal/domain/issueopsreview/gates.go:8`) 반대 방향 import는 순환이다. contract로 옮길 수도 없다: `.issueops/conventions/go-and-packages.md:99-102` "domain은 **같은 capability의** contract(`internal/domain/<cap>` → `internal/contract/<cap>`)와 순수 domain helper만 import할 수 있으며" — `internal/contract/issueopsreview`는 `internal/domain/issueops`·`internal/domain/issueopsnext`와 다른 capability다. 그래서 상수를 두 사용 패키지(`internal/domain/issueopsreview`, `internal/domain/issueopsnext`)가 이미 import하는 `internal/domain/issueops`에 `ParentReviewPattern`으로 옮기고, 기존 사용처가 `issueopsdomain.ParentReviewPattern`을 쓴다. 저장되는 값은 그대로다. 이 값이면 순서의 (1) 스테이징 요구와 (3) stale 판정, readiness의 `architecture_review_stale`을 건너뛴다. 자식에는 자기 아키텍처 문서가 없기 때문이다.
- regress: DA stop의 `regress`는 지금처럼 승인을 지운다(`ClearDesignApproval` → `ClearArchitectureApproval`). 메시지는 `architecture-review regression only applies ...`, 결정 제목 `architecture-review devil's-advocate stop`, stale 노트 `stale: architecture-review regression (...)`.
- 리뷰 절차(스킬): `issueops-review`에 `--target architecture`를 더한다. 대상은 staged architecture 전문, 함께 주는 자료는 이슈 본문·intent 성공 기준·관련 문서. 리뷰어는 빈 컨텍스트 서브에이전트이고 `architecture-review` 스킬의 아키텍처 렌즈를 적용한다. `pass`일 때만 `issueops architecture review --approved --reviewer-context subagent`를 기록한다. `revise`면 아키텍처를 고쳐 재스테이징하고 다시 리뷰한다(같은 5라운드 상한). `stop`이면 승인을 기록하지 않고 이슈 본문의 사실 변경을 `feedback add --classification contract_change`로 남긴 뒤 이슈를 동기화한다. `--target plan`의 함께 주는 자료는 "design review 본문"에서 "승인된 아키텍처 문서 전문"으로 바꾼다.

## Work Objectives
### Core Objective
`issueops next`가 계획 단계에서 아키텍처를 계획보다 먼저 요구하고, 승인이 아키텍처 문서에 묶이며, 범위 안의 design 이름이 모두 architecture로 바뀐다.
### Deliverables
위 TL;DR 목록.
### Definition of Done
G1~G10이 모두 met이고, smoke 증거(아래 T7)가 있다.
### Must Have
- 단계 순서·stale·필수 절 거부 동작, 이름 변경, 스킬 절차, golden, ADR.
### Must NOT Have
- 옛 이름 별칭·호환 읽기, durable phase 추가, 과거 기록 문서 수정, 실제 HOME 설치, source checkout `bin/issueops` 재빌드.

## Verification Strategy
- Test decision: tests-after(동작 변경마다 focused 테스트를 같은 TODO에서 추가), framework `go test`.
- QA: smoke는 worktree 바이너리 + 임시 state/저장소로 에이전트가 실행한다. 사람 확인 없음.
- Evidence: `.issueops/evidence/task-{N}-{slug}.{ext}`는 쓰지 않고 게이트 원장 EVIDENCE와 검증 단계 보고에 남긴다.

## 게이트
- G1: 실행 전 사이클에서 아키텍처가 계획보다 먼저 요구된다 | CHECK: go test ./internal/domain/issueopsnext -run TestClassifyPlanStagesArchitectureFirst -count=1 -v | EXPECT: --- PASS: TestClassifyPlanStagesArchitectureFirst
- G2: 필수 절이 빠지거나 빈 아키텍처는 리뷰 기록이 거부되고 오류에 제목이 나온다 | CHECK: go test ./internal/application/issueopsreview -run TestRecordArchitectureReviewRejectsMissingSections -count=1 -v | EXPECT: --- PASS: TestRecordArchitectureReviewRejectsMissingSections
- G3: 승인 뒤 아키텍처를 바꾸면 stale로 plan.architecture에 머문다 | CHECK: go test ./internal/domain/issueopsnext -run TestClassifyArchitectureReviewStaleStaysInArchitectureStage -count=1 -v | EXPECT: --- PASS: TestClassifyArchitectureReviewStaleStaysInArchitectureStage
- G4: 범위 안 옛 이름(내용·경로)이 0건이다 | CHECK: python3 -c "import subprocess,re;P=['plan.design','StagePlanDesign','design_review','DesignReview','proposed_design','proposed-design','proposeddesign','design_open_questions','design_approval','design_verification','issueops_review_design','design review','design-review','cleanDesignValues','runIssueOpsDesign','intentdesign','issueops design',chr(34)+'design'+chr(34)];X=('.issueops/adr/','.issueops/issues/','.issueops/plans/','.issueops/research/','.issueops/verified-execution/','.issueops/archive/','.issueops/agent-improvements-2026-10-02/','docs/superpowers/','.issueops/ADR.md:','.issueops/documentation/owner-design-review.md','internal/domain/projectdoc/route.go:','internal/application/projectbootstrap/plan_test.go:');o=subprocess.run(['git','grep','-n','-i','-F']+sum([['-e',p] for p in P],[]),capture_output=True,text=True).stdout.splitlines()+[f+':' for f in subprocess.run(['git','ls-files'],capture_output=True,text=True).stdout.splitlines() if 'design' in f.lower() and 'database-schema-architecture' not in f and 'diagram-design' not in f];h=[l for l in o if not l.startswith(X) and not re.match(r'[.]issueops/cautions/20[0-9][0-9]-',l) and not re.match(r'[.]issueops/documentation/quality-[^:]*[.]json:',l) and not re.search(r'design-review [FHBM][0-9]+',l) and not re.search(r'design-review [0-9]+차',l) and 'owner-design-review' not in l and 'Parnas' not in l];print(len(h))" | EXPECT: 0
- G5: CLI 경로에서 start부터 handoff까지 아키텍처가 계획보다 먼저 진행된다 | CHECK: go test ./cmd/issueops/issueopscli -run TestIssueOpsNextWalksArchitectureBeforePlan -count=1 -v | EXPECT: --- PASS: TestIssueOpsNextWalksArchitectureBeforePlan
- G6: gofmt와 vet가 깨끗하다 | CHECK: python3 -c "import subprocess,sys;f=subprocess.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();g=subprocess.run(['gofmt','-l']+f,capture_output=True,text=True).stdout.strip();v=subprocess.run(['go','vet','./...']).returncode;print('fmt-vet-ok' if not g and v==0 else 'fmt-vet-fail');sys.exit(0 if not g and v==0 else 1)" | EXPECT: fmt-vet-ok
- G7: 전체 테스트(golden·계약 포함)가 통과한다 | CHECK: python3 -c "import subprocess,sys;r=subprocess.run(['go','test','./...','-count=1']);print('test-ok' if r.returncode==0 else 'test-fail');sys.exit(r.returncode)" | EXPECT: test-ok
- G8: race 검사가 통과한다 | CHECK: python3 -c "import subprocess,sys;r=subprocess.run(['go','test','-race','./...','-count=1']);print('race-ok' if r.returncode==0 else 'race-fail');sys.exit(r.returncode)" | EXPECT: race-ok
- G9: 부모 리뷰를 승계한 위임 자식은 아키텍처 문서 없이도 plan.architecture에 갇히지 않는다 | CHECK: go test ./internal/domain/issueopsnext -run TestClassifyDelegatedChildSkipsArchitectureStage -count=1 -v | EXPECT: --- PASS: TestClassifyDelegatedChildSkipsArchitectureStage
- G10: 위임 자식을 만들면 승계한 아키텍처 리뷰가 부모 승계 표시를 갖는다 | CHECK: go test ./internal/domain/issueops -run TestBuildDelegatedProfileMarksParentArchitectureReview -count=1 -v | EXPECT: --- PASS: TestBuildDelegatedProfileMarksParentArchitectureReview

G7·G8은 `gates check --env HOME,PATH,LANG --timeout-seconds 900`으로 실행한다.

## Execution Strategy
### Parallel Execution Waves
- 한 세션이 T1→T7을 순서대로 수행한다. 이름 변경과 순서 변경이 같은 심볼을 건드려 병렬 분할하면 충돌한다.
### Dependency Matrix
| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | T2 | — |
| T2 | T1 | T3 | — |
| T3 | T2 | T4 | — |
| T4 | T3 | T5 | — |
| T5 | T4 | T6 | — |
| T6 | T5 | T7 | — |
| T7 | T6 | — | — |

## TODOs
- [ ] 1. Go 이름 변경(동작 불변)
  - What to do: 계약·domain·application·port·adapter·cmd의 design 식별자를 architecture로 바꾼다. 파일 이동: `internal/contract/issueopsreview/design.go`→`architecture.go`, `internal/domain/issueopsreview/design*.go`→`architecture*.go`, `internal/application/issueopsreview/design_review*.go`→`architecture_review*.go`, `internal/application/issueopscycle/design_readiness*.go`→`architecture_readiness*.go`, `internal/adapter/issueops/intentdesign/`→`intentarchitecture/`(패키지명 `intentarchitecture`, 파일 `intent_design_*`→`intent_architecture_*`), `internal/adapter/issueops/issueops_intent_design_test.go`→`issueops_intent_architecture_test.go`, `cmd/issueops/issueopscli/issueops_intent_design.go`→`issueops_intent_architecture.go`, `internal/domain/commandparse/issueops_design_review_test.go`→`issueops_architecture_review_test.go`. 타입·함수·상수: `DesignReview`→`ArchitectureReview`, `DesignReviewRequest`→`ArchitectureReviewRequest`, `ProposedDesign`→`ProposedArchitecture`, `DesignReviewMissing`→`ArchitectureReviewMissing`, `PrepareDesignReview`/`FinalizeDesignReview`/`HasDesignReviewEvidence`/`cleanDesignValues`/`ErrMissingDesignReviewEvidence`/`RecordDesignReview`/`RecordIssueOpsDesignReviewWithActor`/`DesignReviewStore`/`designReviewCommand`/`runIssueOpsDesign`/`printIssueOpsDesignReviewUsage`/`IssueOpsDesignReviewEvidenceExample`(값 `architecture review checked alternatives and risks`)/`ClearDesignApproval`/`StagePlanDesign`→각각 Architecture 이름, `PlanningRecorder.Design`→`Architecture`, `parent_design_review_unapproved`→`parent_architecture_review_unapproved`. JSON 태그 `design_review`→`architecture_review`(`internal/contract/issueops/types.go:335`, `internal/contract/issueopslease/record.go:23`, `internal/contract/issueopsnext` 투영), `proposed_design`→`proposed_architecture`. 게이트 키 `design_*`→`architecture_*`, `owner_command.go`의 접두사 `design_`→`architecture_`. CLI 그룹 토큰 `design`→`architecture`(`commandparse/issueops.go:35,53,234-237`, `issueops.go:30`, `issueops_catalog.go:39-40,119`), 플래그 `--proposed-design`→`--proposed-architecture`, 오류 `unknown issueops architecture subcommand`, `cannot link plan before approved architecture review`(`internal/domain/issueops/workspace_link.go:32-33`). 스킬명을 가리키는 주석(`types.go:380`, `cli_result_types.go:4`, `issueops_subcommands.go:463`)은 바꾸고, 과거 리뷰 라운드를 인용한 주석은 그대로 둔다. regress 문자열 세 개를 위 아키텍처 결정대로 바꾼다. 테스트 픽스처 문자열의 "design review" 문구도 "architecture review"로 바꾼다.
  - Must NOT do: 동작 변경, 별칭, `projectdoc/route.go`의 라우팅 토큰 `design` 수정.
  - Recommended Agent: deep; Reason: 계약 다수 파일
  - Parallelization: NO; Wave 1; Blocks T2; Blocked By —
  - References: 위 `## 하위 호환성과 side effect`, LSP rename(`xd://lsp`)으로 참조를 옮긴다.
  - Acceptance Criteria: `go build ./...`와 `go vet ./...`가 통과한다. 스킬 계약 테스트와 golden은 T6을 마친 뒤 T5의 마지막 재생성에서 맞춘다.
  - QA Scenarios: happy — `go run ./cmd/issueops architecture review --help`가 exit 0으로 `--proposed-architecture`를 출력. failure — `go run ./cmd/issueops design review --help`가 `unknown issueops subcommand` 계열 오류로 실패.
  - Commit: NO(T1~T6을 한 커밋으로 묶어 golden·스킬 계약 테스트 red를 남기지 않는다)
- [ ] 2. artifact `architecture`와 필수 절 검사
  - What to do: `issueopsartifact.NormalizeName`에 `architecture` 추가(오류 문구 `plan|spec|verified-execution-loop|architecture`), `artifact stage` 도움말 갱신. `internal/domain/issueops/plan_sections.go` 옆에 `RequiredArchitectureSections`와 `MissingArchitectureSections`(제목 없음 또는 본문 공백). `tracked_materials.go`에 `trackedArchitecture = "architecture.md"`와 봉인 `architecture` 읽기를 `spec`과 같은 방식으로 추가.
  - Must NOT do: plan 필수 절 변경.
  - Recommended Agent: quick; Reason: 기존 패턴 복제
  - Parallelization: NO; Wave 2; Blocks T3; Blocked By T1
  - References: `plan_sections.go`, `plan_sections_test.go`, `tracked_materials.go:77-100`
  - Acceptance Criteria: `MissingArchitectureSections` 테스트가 제목 없음·빈 본문·`### ` 하위 제목·본문 속 언급을 구분한다. `artifact stage --name architecture`가 실행 전 사이클에서 성공한다.
  - QA Scenarios: happy — 다섯 절이 있고 한 절이 "해당 없음: 근거"인 문서 → missing 없음. failure — `## 병목과 장애` 본문이 비어 있으면 그 제목이 반환된다.
  - Commit: NO
- [ ] 3. 아키텍처 리뷰 기록: digest·출처·이력
  - What to do: `ArchitectureReview`에 `ReviewerContext`(`reviewer_context`), `ReviewedArchitectureDigest`(`reviewed_architecture_digest`), `History []ArchitectureReviewRound`(`history`) 추가. `ArchitectureDigestSource` 포트와 `ArchitectureDigestResolver`(실행 전 staged, 실행 뒤 봉인 파일)를 `plan_binding.go` 패턴으로 추가하고 composition root에서 배선. `RecordArchitectureReview`가 문서 부재·필수 절 누락을 위 문구로 거부하고 digest·history를 기록. CLI `--reviewer-context`(필수, `subagent|inline`만 허용). `ImplementationReadinessMissing`에 `architecture_review_stale`(digest 불일치·오류) 추가, delegated child 부모 승계는 예외. `ParentReviewPattern` 상수를 `internal/domain/issueops`(`delegation.go` 옆)로 옮기고(위 아키텍처 결정의 의존 방향 근거) 사용처 `internal/domain/issueopsreview/devils_advocate_readiness.go`, `inherited_review.go`, 테스트 `devils_advocate_readiness_test.go:30,52`, `gaps_test.go:25`, 그리고 새 분류 예외(`internal/domain/issueopsnext`)가 `issueopsdomain.ParentReviewPattern`을 쓴다. `BuildDelegatedProfile`(`internal/domain/issueops/delegation.go:34`, 복사 지점 90-94)이 복사한 `ArchitectureReview`의 `ReviewerContext`를 그 상수로 덮어쓴다(`Approved = true`, `ReviewedAt = now`는 유지).
  - Must NOT do: record-only `PlanReadinessMissing`/`GrillReadinessMissing`에 관측 추가.
  - Recommended Agent: deep; Reason: 포트·배선·readiness
  - Parallelization: NO; Wave 3; Blocks T4; Blocked By T2
  - References: `internal/application/issueopsreview/plan_binding.go`, `record.go:29`, `internal/domain/issueopsreview/devils_advocate_readiness.go`, `internal/adapter/issueops/review_plan_source.go`, `internal/adapter/issueops/issueops_readiness.go:14`
  - Acceptance Criteria: G2, G10 met. G2의 테스트는 같은 함수 안에서 스테이징 없는 기록이 `architecture review requires a staged architecture artifact`로, `--reviewer-context` 없는 기록과 `--reviewer-context delegated-parent-review` 기록이 거부되는 것도 단언한다. G10의 테스트는 부모 `reviewer_context = subagent` 입력에서 자식 복사본의 `ReviewerContext == "delegated-parent-review"`와 `Approved == true`를 단언한다. 같은 리뷰를 두 번 기록하면 `history` 길이 1. 실행 뒤 봉인 파일과 digest가 같으면 stale 없음. `go test ./internal/architecture -run TestProductionGraphHasNoForbiddenAdapterEdges -count=1`이 통과한다.
  - QA Scenarios: happy — 다섯 절 문서 스테이징 후 기록 → `reviewed_architecture_digest`가 문서 sha256과 같다. failure — 스테이징 없이 기록 → `architecture review requires a staged architecture artifact`.
  - Commit: NO
- [ ] 4. 단계 순서와 next 출력
  - What to do: `StagePlanArchitecture = "plan.architecture"` 추가, `StagePlanDesign` 삭제. `Input`에 `StagedArchitecture bool`, `ArchitectureReviewStale bool` 추가. `service.go`가 staged map 한 번으로 `StagedPlan`, `StagedArchitecture`, stale을 계산(포트 `StagedArtifacts`의 반환을 map으로 바꾸고 `issueops_next_wiring.go:58` 배선 수정). `classifyPreparation`을 위 아키텍처 결정 순서로 바꾼다. `owner_command.go`에 `architecture_artifact`와 `architecture_review_stale`. `issueopsstatus/projector.go:68` plan 목록 갱신.
  - Must NOT do: 실행 뒤(4단계 이후) 분류 순서 변경.
  - Recommended Agent: deep; Reason: 상태머신
  - Parallelization: NO; Wave 4; Blocks T5; Blocked By T3
  - References: `classify.go:183-219`, `classify_test.go:183-191`, `selection_test.go:254-257`
  - Acceptance Criteria: G1, G3, G9 met. 기존 classify 표 테스트를 새 순서로 고친다(아키텍처 없이 plan만 staged → `plan.architecture`). `TestClassifyDelegatedChildSkipsArchitectureStage`는 staged architecture 없음 + `reviewer_context = delegated-parent-review` 승인 + staged plan일 때 stage가 `plan.architecture`가 아님(`plan.review`)을, 같은 입력에서 `reviewer_context`만 `subagent`로 바꾸면 `plan.architecture`임을 단언한다.
  - QA Scenarios: happy — staged architecture+승인+staged plan → `plan.review`. failure — staged plan만 있음 → `plan.architecture`, missing `architecture_artifact`.
  - Commit: NO
- [ ] 5. CLI 통합 테스트·golden·inventory·pioneer 자료
  - What to do: `cmd/issueops/issueopscli`에 `TestIssueOpsNextWalksArchitectureBeforePlan`을 추가한다. 기존 `test_helpers_test.go` 픽스처로 사이클을 plan phase까지 준비한 뒤 다음을 순서대로 단언한다: (a) `artifact stage --name plan`만 한 상태의 `next` stage가 `plan.architecture`, missing `architecture_artifact`. (b) 다섯 절 architecture를 스테이징하고 `architecture review --approved --reviewer-context subagent`를 기록한 뒤 `next`가 `plan.review`. (c) 내용을 바꾼 architecture를 다시 스테이징한 뒤 `next`가 `plan.architecture`, missing `architecture_review_stale`. `cmd/issueops/issueopsapp/response_contract_cli_snapshot_test.go`는 스냅숏 키를 `issueops_review_architecture`로, 인자를 `"architecture", "review"`와 `--proposed-architecture`로, 115행의 문자열 `IssueOps needs explicit design review`를 `IssueOps needs explicit architecture review`로, `design review checked contract drift risk`를 `architecture review checked contract drift risk`로, `Gate implementation on approved design`을 `Gate implementation on approved architecture`로 바꾸고, 163행 regress 사유의 `design-review stop`을 `architecture-review stop`으로 바꾼다. 다른 테스트 파일의 같은 문구도 `git grep -n -i -F 'design review' -- cmd internal`으로 찾아 함께 바꾼다. golden 재생성: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`, `go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -update-ddd-inventory -count=1`. `git mv testdata/pioneer-holdouts/design-review testdata/pioneer-holdouts/architecture-review`, `evidence-records/design-review.json`→`architecture-review.json`, `testdata/issueops/fixtures/pioneer-design-review.json`→`pioneer-architecture-review.json`, 내용의 스킬명 갱신, manifest 경로와 바뀐 파일의 sha256 재계산. pioneer catalog·benchmark·quality 테스트 키 갱신. 스킬 계약 테스트(`internal/adapter/skillcontract/skill_contract_test.go:94`, `internal/adapter/issueops/issueops_skill_contract_test.go:27-29`)가 스킬 본문을 읽으므로 T6을 끝낸 뒤 golden을 마지막으로 재생성하고, 재생성한 golden에서 `git grep -c -i -F 'design review' -- cmd/issueops/testdata`가 0인지 확인한다.
  - Must NOT do: golden diff에 이름·순서 외 변화 수용, `has_openai_yaml: false` 수용.
  - Recommended Agent: deep; Reason: 계약 golden
  - Parallelization: NO; Wave 5; Blocks T6; Blocked By T4
  - References: `.issueops/testing/unit-and-contract.md:64-68`, `internal/adapter/outbound/quality/pioneer.go:277-303`
  - Acceptance Criteria: G5, G7 met. `issueops contract check --json`(worktree 바이너리)이 ok.
  - QA Scenarios: happy — golden diff에 `plan.architecture`, `architecture_review`, `architecture-review`가 나타난다. failure — `mcp_tools.golden.json` diff가 생기면 원인을 찾아 되돌린다.
  - Commit: NO(T6과 한 커밋으로 묶는다. 스킬 계약 테스트와 golden이 스킬 이름·본문을 읽어 따로 커밋하면 중간 커밋이 red가 된다)
- [ ] 6. 스킬·현행 문서
  - What to do: `git mv skills/design-review skills/architecture-review`; frontmatter `name: architecture-review`, 제목·`display_name` "Architecture Review", `default_prompt`의 `$architecture-review`, 아키텍처 문서 렌즈 절 추가(다섯 절 각각이 요구사항·데이터/API·구조/흐름·병목/장애·절충점을 결정 가능한 수준으로 담는지, Blocking threshold 그대로). `skills/issueops-plan/SKILL.md`: description의 stage 목록, 절 순서(문서 확인 → 아키텍처 작성·스테이징 → 아키텍처 검토 → 계획 작성·스테이징 → 게이트 원장 → 계획 검토 루프 → 인계), `issueops architecture review` 명령과 증거 문구 규칙, 검증 절 `architecture_review.approved`. `skills/issueops-review/SKILL.md`: `--target architecture` 행·기록 명령·링크 경로. `skills/issueops/SKILL.md:190` 단계 표. `skills/implementation-planning/SKILL.md:264-266`. `skills/issueops/references/cleanup-state.md`, `orchestration.md`, `execution.md:307`, `skills/issueops-complete/SKILL.md:151`, `skills/requirements-analysis/SKILL.md:260`. 현행 문서: `.issueops/AGENT_WORKFLOW.md`(75·87행), `.issueops/cautions/issueops-lifecycle.md`(82행 부근), `.issueops/architecture/host-integration.md:50`, `.issueops/OPERATIONS.md:125`("durable design" → "durable architecture"), `.issueops/CONVENTIONS.md` 이슈 산출물 레이아웃 표에 `architecture.md`(추적 사본)와 artifact 목록, `README.en.md:522`. 한국어 문서는 `fluent-korean` 지침으로 다듬는다.
  - Must NOT do: 날짜 기록 문서, `.issueops/ADR.md`의 기존 행, `owner-design-review.md` 수정.
  - Recommended Agent: quick; Reason: 문서
  - Parallelization: NO; Wave 6; Blocks T7; Blocked By T5
  - References: 위 G4 목록
  - Acceptance Criteria: G4 met. `python3 scripts/validate-skill.py skills/architecture-review`, `python3 scripts/validate-skill.py skills/issueops-plan`, `python3 scripts/verify-skill-shell.py skills/issueops-plan` 통과. 문서 편집 뒤 golden 재생성.
  - QA Scenarios: happy — `skills/issueops-review/SKILL.md`의 `../architecture-review/SKILL.md#blocking-threshold` 링크 대상 제목이 존재. failure — G4가 0이 아니면 남은 줄을 고친다.
  - Commit: YES; Message: `feat(issueops): require an approved architecture before the plan`; Files: T1~T6 전체(Go, testdata, skills/, .issueops 현행 문서, README.en.md, 재생성 golden)
- [ ] 7. ADR과 smoke
  - What to do: MCP `project_docs_append(kind=adr)`로 새 ADR(제목: plan 단계에서 아키텍처를 계획보다 먼저 승인하고 design 단계를 architecture로 바꾼다; 2026-06-18 결정의 순서 부분을 대체; 기각 대안: 새 durable phase, 순서만 변경, `spec` 재사용, 호환 읽기)과 `.issueops/ADR.md` 색인 행. smoke: worktree에서 `go build -o bin/issueops ./cmd/issueops` 뒤 임시 `ISSUEOPS_STATE_DIR`와 임시 Git 저장소(`git init`, 커밋 하나)에서 `start` → `intent record` → `domain-review record` → `decision add` → `plan-prep record` → `phase --to grill` → `link-issue --issue-url https://github.com/example/repo/issues/1` → `branch prepare ... --link-verified` → `phase --to plan` → `artifact stage --name plan` → `next`(`plan.architecture`, `architecture_artifact`) → `artifact stage --name architecture`(다섯 절) → `architecture review ... --approved --reviewer-context inline` → `next`(`plan.review`) → architecture 재스테이징 → `next`(`plan.architecture`, `architecture_review_stale`). 각 단계의 stage.key와 missing을 기록한다. 원격을 부르는 명령은 쓰지 않는다.
  - Must NOT do: 실제 state·HOME 사용, source checkout 바이너리 교체.
  - Recommended Agent: quick
  - Parallelization: NO; Wave 7; Blocked By T6
  - References: `.issueops/AGENT_WORKFLOW.md` Finish 절
  - Acceptance Criteria: smoke 출력이 위 순서와 같다. G1~G10 met. ADR·색인 편집 뒤 `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1`이 실패하면 `-update`로 재생성해 같은 커밋에 넣는다. 새 ADR 색인 행에는 옛 명령·키 이름을 현재 계약처럼 쓰지 않는다.
  - QA Scenarios: happy — 위 순서. failure — 임시 state에 옛 바이너리로 만든 record를 두고 새 바이너리 `status`가 `invalid state`를 내는 것을 한 번 관측해 하위 호환 서술을 확인한다.
  - Commit: YES; Message: `docs(adr): record the architecture-first plan stage`; Files: `.issueops/adr/<새 파일>`, `.issueops/ADR.md`, 재생성 golden

## Final Verification Wave
- F1 Plan Compliance Audit: TODO 1~7 수행 여부
- F2 Code Quality Review: 별칭·죽은 코드·불필요 추상 없음
- F3 Real Manual QA: T7 smoke 증거
- F4 Scope Fidelity Check: 과거 기록 미수정, 범위 밖 design 미수정

## Commit Strategy
두 커밋(T1~T6 묶음, T7). 각 커밋은 golden 재생성을 포함해 그 커밋만으로 테스트가 통과한다. Conventional Commit + Lore body(`.issueops/COMMIT_POLICY.md`).

## Success Criteria
이슈 #570 완료 기준 6개가 G1~G10과 T7 smoke로 확인된다.
