# 리뷰 실행 시점의 설정 소비 통일

Lifecycle: io-814b092d660e
Issue: https://github.com/m16khb-org/issueops/issues/522
Branch: 522-runtime-review-profile
Base: main / 768546a219b082f5f7ad7f78f85664747eca9b74
Source: $SOURCE_ROOT
사용자 원문: “3 항목을 각각 $issueops 를 통해 병렬로 인계하여 진행해줘”

## 목적과 범위

담당 세션이 구현 검증 리뷰를 띄우기 직전에 exact lifecycle의 `issueops next --id ... --json` 결과에서 `.review.model`, `.review.effort`, `.review.tier`, `.review.lenses`를 읽고 `issueops-review`를 실행하게 한다. 준비 시점 reviewer 메타데이터는 준비 당시 기본값이라는 의미만 유지한다. runtime 정책을 owner 템플릿에 복제하지 않는다. 생성되는 implementation-review 기록 명령 역시 prepare 기본값을 고정하지 않고 실제 리뷰에 넘긴 모델과 effort를 입력받는다.

승인된 종료점은 독립 작업 공간에서 구현, 검증, commit/push, Draft PR 발행과 execution complete다. merge와 cleanup은 포함하지 않는다. 품질 scanner, latest status 선택은 다른 병렬 사이클의 소유이며 수정하지 않는다. source tracked 파일은 수정하지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제3장: host 중립 정책은 domain/application에 유지하며 adapter는 소비만 한다.
- `.issueops/ARCHITECTURE.md` 의존 방향: 이번 변경으로 import 예외나 레이어 역참조를 추가하지 않는다.
- `.issueops/CONVENTIONS.md:87-94`: runtime next.review와 prepare reviewer effort의 불일치 TODO를 실제 코드와 함께 제거한다.
- `.issueops/CAUTIONS.md` Universal summary: canonical worktree와 active native holder만 수정하고 lifecycle 명령이 state를 소유한다.
- `.issueops/ADR.md` 및 `adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md`: 독립 리뷰와 모델 감사 필드를 보존한다. 역사적 모델 명칭은 현재 기본값 근거로 사용하지 않는다.
- `.issueops/TESTING.md` 최소 완료 기준과 `testing/self-verification.md`: final change set의 단일 전체 self-verify 결과를 남기고 서로 다른 run의 부분 통과를 합치지 않는다.
- `skills/prompt-engineering/SKILL.md`: 기존 버전 관리 prompt artifact를 수정하고 실제 렌더링 및 bounded prompt 평가로 기준을 검증한다.

## 재사용하는 기존 구현

- `internal/domain/agentmodel/defaults.go:32-58`의 PlannerDefaults와 ReviewEffortForTier는 그대로 사용한다. Claude Opus 5.5/high, Codex Astra/xhigh와 docs-only medium 선택을 변경하지 않는다.
- `internal/application/issueopsnext/service.go:124-125` 및 `review_tier_test.go`의 runtime tier 선택과 기존 테스트를 재사용한다.
- `internal/application/issueopsowner/policy.go:15-20`은 prepare 기본값 메타데이터 소유다. 새 저장 필드나 runtime 계산을 여기에 추가하지 않는다.
- `internal/adapter/issueops/testdata/execution_owner_prompt.txt:167-172`의 리뷰 실행 지시를 수정한다. 새 command 객체를 만들지 않고 이미 지원하는 exact-ID `issueops next`를 사용한다.
- `internal/adapter/issueops/execution_owner_packet_test.go:335-353`은 `.issueops/prompt-engineering/prompts/issueops-v1-owner-execution-v1.md`의 PROMPT fence와 embed template의 byte parity를 검증한다. 양쪽 원본을 함께 갱신한다.
- `skills/issueops-review/SKILL.md:150-162`의 3라운드 정책을 재사용한다. 준비 기본값, 현재 diff tier, 실제 라운드 상승의 관계를 명료화하고 상승 규칙을 새 구현에 복제하지 않는다.

## 구현 순서

1. canonical worktree에서 next/status/whoami, HEAD와 plan digest를 확인하고 자기 native actor로 active holder를 확보한다. plan과 compatibility review를 기록하고 gates ledger를 만든다.
2. 실제 `executionOwnerPromptFixture` 결과에 exact-ID runtime review 조회와 review skill 위임이 나타나야 한다는 focused failing test를 먼저 추가한다. prepare-time effort를 무조건 실행 effort로 쓰는 이전 지시가 남으면 실패하도록 한다.
3. `internal/domain/issueops/owner_context.go:216-223`의 ImplementationReview command에서 prepare 모델/effort 고정을 제거하고 기존 VERDICT와 같은 입력 방식으로 `--reviewer-model <REVIEWER_MODEL> --reviewer-effort <REVIEWER_EFFORT>`를 둔다. host와 ID/generation/actor fence는 그대로 유지한다. owner 지침은 두 placeholder를 실제 review 실행에 넘긴 값(라운드 상승 적용 후 값)으로 shell-quote해 채우도록 한다. catalog validator가 placeholder를 처리하는 기존 방식을 재사용하고 새 API는 만들지 않는다. `internal/adapter/issueops/issueops_implementation_review_test.go:145`의 고정값 기대 테스트를 runtime 입력과 유효 catalog 기대값으로 교체한다. owner template과 버전 관리 prompt artifact를 최소 수정한다. 상단 reviewer metadata에는 준비 당시 기본값임을 명시하고 검증 시 runtime 결과가 기준임을 지시한다. `next.review`가 비거나 조회 실패하면 고정 값으로 추정하지 않고 review skill의 중단 규칙을 따른다. 모델 이름/effort 매핑 표를 template에 복제하지 않는다.
4. review skill 절차와 CONVENTIONS TODO를 맞춘다. 사용자가 reviewer를 명시한 경우의 기존 예외와 owner-model/owner-effort override를 혼동하지 않는다. 기본 tier 선택과 세 번째 리뷰에서의 명시적 상승을 분리해 기록한다. 신규 override CLI/schema를 만들지 않는다.
5. 아래 G1~G4와 prompt 평가를 실행하고 ai-slop-clean, project docs review, 독립 구현 리뷰를 완료한다. 이후 승인된 commit/push, Draft PR, execution complete까지 진행한다. 다른 두 사이클을 기다릴 필요가 없다.

## 성능 영향

리뷰 실행 전 이미 읽어야 하는 `next`의 read-only 출력 소비를 명시한다. 새로운 polling, 네트워크 조회, cache, durable state, 별도 tier 계산을 추가하지 않는다. 기존 next의 changed-path 관측 횟수 테스트를 유지한다. 최적화 속도 향상은 주장하지 않는다.

## 하위 호환성과 side effect

CLI JSON/MCP schema/record schema/기본 모델/기본 effort는 변경하지 않는다. 기존 prepare 자료의 prompt와 digest를 자동 재생성하거나 active 사이클을 마이그레이션하지 않는다. 신규 prepare부터 새로운 지침이 적용된다. rollback은 이 변경 커밋을 되돌리는 것으로 끝나며 기존 record는 그대로 읽힌다. project docs 수정은 project-docs-update의 read/CAS 절차를 쓴다.

Prompt 실패 기준: 렌더링된 owner가 검증 단계에서 prepare 고정 effort를 직접 실행하거나, 다른 ID의 next를 조회하거나, 최신 결과 조회 실패에도 임의 값으로 진행하거나, 3라운드 상승을 기본값 변경처럼 기록하면 실패다. 최소 5개 입력을 평가한다: (1) Codex docs-only, (2) Claude default, (3) contract 또는 schema-auth 변경, (4) review.model 누락/조회 실패, (5) Claude 세 번째 라운드. 실제 production rendering을 제공한 fresh-context read-only 평가에서 선택 모델·effort·출처·라운드 이유와 최종 implementation-review 기록 명령을 bounded JSON으로 받는다. 실제 실행값과 기록 명령의 reviewer 인자가 다르면 실패다. 평가 프롬프트는 private reasoning 공개를 요구하지 않고 가상의 도구를 요구하지 않는다. 문자열 assertion은 rendering wiring 검증일 뿐 semantic 결과로 대체하지 않는다. 기존 template과 수정 template의 입력별 결과를 기록한다. prompt evaluation은 네트워크/파일 mutation을 허용하지 않는다.

기존 봉인 자료에 대한 side effect 확인은 생성 fixture를 보존한 뒤 신규 renderer 호출로 기존 파일·manifest digest가 변하지 않았음을 확인하는 기존 owner packet test를 재사용하거나 최소 확장한다. 실제 운영 중 record를 수정해 실험하지 않는다.

## 검증 항목

- G1: 실제 owner 렌더링과 봉인 보존 | CHECK: go test ./internal/adapter/issueops -run 'TestExecutionOwner|TestOwnerArtifacts|TestPrepareExecutionOwner|Test.*ImplementationReview' -count=1 | EXPECT: ok
- G2: 정책 및 runtime tier | CHECK: go test ./internal/domain/agentmodel ./internal/application/issueopsnext ./internal/application/issueopsowner -count=1 | EXPECT: ok
- G3: review skill 계약 | CHECK: python3 scripts/validate-skill.py skills/issueops-review | EXPECT: Skill is valid!
- G4: 최종 단일 self-verify | CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json | EXPECT: 각 검증 단계 성공 및 최저 점수 95 이상. 실제 CLI 출력에 맞는 ledger assertion을 작성한다.

G3의 실제 validator 출력 `Skill is valid!`를 준비 단계에서 확인했다. shell &&/pipe를 gates CHECK에 직접 넣지 않는다. Go 변경이면 final self-verify에 race/vet/build 필수 단계가 포함됨을 결과에서 확인하고, 빠져 있으면 저장소 검증 규칙대로 보완한다. semantic prompt 평가 5건은 별도 evidence로 원장에 남긴다.

## 인계

direct execution prepare로 생성한 같은 worktree에서 Orca ready를 재확인하고 새 Codex `gpt-6.1-sol/high` 세션에 인계한다. writer와 자손 0, plan/head/material digest를 기록한 뒤 release한다. 수신 세션은 direct released 상태의 next_command 복구 체인을 따라 자기 actor로 인수하며 다시 자동 인계하지 않는다. 전달 수락 확인 후 준비 세션은 구현 결과를 기다리지 않고 종료한다.

Repo grounding: 위 소스와 문서, 실제 issue #522 및 main base를 조회했다.
Decision-complete plan: 기존 runtime 정책을 소비하는 prompt/생성 기록 명령/skill/docs 최소 변경이다.
Assumptions/defaults: main을 독립 base로 사용하고 기존 모델 기본값은 보존한다.
Unresolved questions: none blocking.
Acceptance criteria: G1~G4와 prompt 평가 5건, 기존 sealed 자료 보존, Draft PR 발행.
