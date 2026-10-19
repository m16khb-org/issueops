# #570 검증 보고서

## 결과와 판정 조건

plan 단계 안의 순서를 `plan.architecture`, `plan.write`, `plan.review`, `plan.handoff`로 바꿨다.
아키텍처 문서(artifact `architecture`)를 계획보다 먼저 스테이징하고 독립 리뷰를 받는다. 승인은 그
문서의 sha256, `reviewer_context`, `history`에 묶인다. 같은 변경에서 이 단계와 리뷰를 가리키던
`design` 이름(stage 키, CLI 명령, record 키, 게이트 키, Go 타입, 공용 스킬)을 `architecture`로 바꿨고
별칭이나 옛 키 읽기 경로는 두지 않았다. 사용자 지시로 과거 리뷰 라운드를 인용한 Go 주석 25줄과
날짜 없는 현행 문서(ADR 색인 문구, `owner-architecture-review.md`, `quality-*-2026-10-03.json` 경로)도 함께 바꿨다.

완료 판정은 게이트 원장 G1~G11의 실행 결과, 새 바이너리 smoke, 독립 diff 리뷰, 발행된 draft PR과
`execution complete` receipt를 함께 본다. 최종 HEAD와 PR URL은 해당 사이클의 durable completion에 남는다.

## 의도 대조

| 성공 기준 | 이 변경의 확인 방법과 증거 |
| --- | --- |
| `issueops next`가 `plan.architecture` → `plan.write` → `plan.review` → `plan.handoff` 순서를 돌려준다 | G1 `TestClassifyPlanStagesArchitectureFirst`, G5 `TestIssueOpsNextWalksArchitectureBeforePlan`(CLI 경로), smoke 1~5 |
| 필수 절이 빠진 문서는 리뷰 기록이 거부되고 빠진 절 제목이 오류에 나온다 | G2 `TestRecordArchitectureReviewRejectsMissingSections`, smoke 4(`architecture is missing required sections: ## 데이터와 API 계약, ...`) |
| 승인 뒤 문서를 고치면 stale이 되어 `plan.write`로 넘어가지 않는다 | G3 `TestClassifyArchitectureReviewStaleStaysInArchitectureStage`, G5의 (c) 단계, smoke 6(`architecture_review_stale`) |
| 현행 코드·스킬·문서·golden에 범위 안의 옛 이름이 남지 않는다 | G4(내용·경로 0건), G11(코드 주석과 날짜 없는 현행 문서의 `design review` 표기 0건) |
| 임시 state와 임시 저장소에서 새 바이너리로 계획 단계를 처음부터 실행해 순서를 확인한다 | 아래 smoke 기록 |
| 순서 변경과 이름 변경 결정을 새 ADR로 남긴다 | `.issueops/adr/2026-10-19-approve-the-architecture-before-the-plan-and-rename-the-desi.md`와 `.issueops/ADR.md` 색인 행 |

## 변경 범위

- 계약·domain·application·port·adapter·cmd: `DesignReview*`를 `ArchitectureReview*`로 옮기고(파일 이동 포함),
  `ArchitectureReview`에 `reviewer_context`, `reviewed_architecture_digest`, `history`를 더했다.
  `issueopsnext`는 `StagedArchitecture`, `ArchitectureReviewStale` 입력으로 `classifyPreparation` 순서를 바꿨다.
  `ImplementationReadinessMissing`은 봉인 architecture digest로 `architecture_review_stale`을 낸다.
  `ParentReviewPattern`은 `internal/domain/issueops`로 옮겨 위임 자식의 승계 표시를 `reviewer_context`에도 쓴다.
- artifact 이름 `architecture`, 필수 절 검사(`MissingArchitectureSections`), 추적 사본 `architecture.md`.
- 공용 스킬 `design-review`를 `architecture-review`로 옮기고 아키텍처 문서 렌즈를 더했다.
  `issueops-plan`(아키텍처 작성·스테이징·검토 절과 순서), `issueops-review`(`--target architecture`), `issueops`
  단계 표와 참조 문서, `implementation-planning`을 고쳤다.
- golden(`response_contracts`, `usage`), DDD inventory, pioneer manifest·fixture·holdout 경로와 `evidence_sha256`을 갱신했다.
- 현행 문서: `AGENT_WORKFLOW`, `CONVENTIONS`(이슈 산출물 표), `OPERATIONS`, `architecture/host-integration`,
  `cautions/issueops-lifecycle`, `conventions/go-and-packages`, `README.en.md`, `ADR.md`와 새 ADR.
- `.issueops/issues/570/`: intent, plan, plan-review, gates, 이 보고서를 커밋한다.

계획 `## 재사용하는 기존 구현`과 다르게 한 곳이 둘 있다. `ArchitectureDigestResolver` 타입은 만들지 않고
`ArchitectureReviewStore.Document` 함수 필드로 문서 본문을 주입했다(읽기는 어댑터 `ReviewArchitectureSource`,
digest와 필수 절 판정은 domain). 같은 판정을 한 곳이 소유하고 래퍼가 하나 줄어든다. 그리고 G7·G8의
CHECK는 `subprocess.run(..., capture_output=True)`로 고쳤다. 원래 명령은 `go test` 출력 수백 줄 뒤에 기대 문자열을
찍어서 하네스가 `expect not matched`로 판정했다. 검사하는 명령과 기대 문자열은 같다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
| --- | --- | --- |
| `.issueops/CONSTITUTION.md` | CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다. | usage·response contract golden을 재생성하고 worktree 바이너리로 smoke를 실행했다 |
| `.issueops/ARCHITECTURE.md` | `internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete adapter나 `cmd/...`를 import하지 않는다. | 단계 판정·stale 판정·필수 절 검사는 domain 순수 함수이고 파일 읽기는 어댑터가 주입한다 |
| `.issueops/CONVENTIONS.md` | 이 표가 유일한 규정이다. | 이슈 산출물 레이아웃 표에 `architecture.md`와 봉인 artifact 이름을 더했다 |
| `.issueops/cautions/issueops-lifecycle.md` | 골든 재생성은 원인이 된 변경과 같은 커밋에 넣어 red를 남기지 않는다. | 이름 변경, 스킬, golden을 한 커밋에 넣는다 |
| `.issueops/testing/unit-and-contract.md` | CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다. | `-update`로 재생성한 뒤 diff가 이름·순서·신규 필드뿐인지 확인했고 `mcp_tools.golden.json`은 바뀌지 않았다 |

대조했으나 적용할 항목이 없는 문서: `.issueops/TESTING.md`(필수 배터리는 G6~G8로 실행), `.issueops/OPEN_API_SPEC.md`(HTTP endpoint 변경 없음).

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
| --- | --- | --- | --- |
| `.issueops/ARCHITECTURE.md` | `internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete adapter나 `cmd/...`를 import하지 않는다. | 지킴 | `internal/domain/issueopsreview/architecture.go`, `internal/domain/issueopsnext/classify.go`는 domain과 contract만 import한다. `internal/architecture` 테스트 통과 |
| `.issueops/CONVENTIONS.md` | 이 표가 유일한 규정이다. | 지킴 | `CONVENTIONS.md` 이슈 산출물 표 |
| `.issueops/cautions/2026-09-23-issueopsrecord-field-lease-record.md` | IssueOpsRecord에 최상위 field를 추가하면 lease Record에도 추가한다. | 지킴 | 최상위 키는 이름만 바뀌었고 `internal/contract/issueopslease/record.go`를 같은 커밋에서 바꿨다 |
| `.issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md` | 스킬 디렉터리를 추가하는 커밋은 | 지킴 | `skills/architecture-review/`는 `git mv`로 옮겨 `SKILL.md`와 `agents/openai.yaml`이 함께 있다 |

## 검증 방식과 실행 근거

Success criteria: 위 의도 대조의 여섯 항목을 이진 결과로 확인한다.
Verification mode: 게이트 원장과 새 바이너리 smoke, 독립 diff 리뷰.
Cleanup receipt: smoke는 `mktemp -d`의 state와 저장소에서만 실행했고 실제 HOME에 설치하지 않았다. source checkout의 `bin/issueops`는 바꾸지 않았다.

| 요구한 문서 경로 | 그 문서의 해당 문장 인용 | 명령 | 결과 또는 Not Run 사유 |
| --- | --- | --- | --- |
| 게이트 원장 | G1 | `go test ./internal/domain/issueopsnext -run TestClassifyPlanStagesArchitectureFirst -count=1 -v` | PASS |
| 게이트 원장 | G2 | `go test ./internal/application/issueopsreview -run TestRecordArchitectureReviewRejectsMissingSections -count=1 -v` | PASS |
| 게이트 원장 | G3 | `go test ./internal/domain/issueopsnext -run TestClassifyArchitectureReviewStaleStaysInArchitectureStage -count=1 -v` | PASS |
| 게이트 원장 | G4 | 잔여 이름 스캔(원장 CHECK) | 0건(main에서 604건) |
| 게이트 원장 | G5 | `go test ./cmd/issueops/issueopscli -run TestIssueOpsNextWalksArchitectureBeforePlan -count=1 -v` | PASS |
| 게이트 원장 | G6 | `gofmt -l`과 `go vet ./...` | fmt-vet-ok |
| `.issueops/TESTING.md` | 필수 배터리 | `go test ./... -count=1`(G7) | test-ok |
| `.issueops/TESTING.md` | 필수 배터리 | `go test -race ./... -count=1`(G8) | race-ok |
| 게이트 원장 | G9, G10 | `TestClassifyDelegatedChildSkipsArchitectureStage`, `TestBuildDelegatedProfileMarksParentArchitectureReview` | PASS |
| 게이트 원장 | G11 | `design review` 표기 스캔(코드 주석, ADR 색인, documentation) | 0건(main에서 164건) |
| 스킬 | issueops-plan, issueops-review, architecture-review | `python3 scripts/validate-skill.py`, `scripts/verify-skill-shell.py` | 통과 |

새 바이너리 smoke(`go build -o bin/issueops ./cmd/issueops`, 임시 `ISSUEOPS_STATE_DIR`와 임시 Git 저장소):

| 순서 | 상태 | stage.key | missing |
| --- | --- | --- | --- |
| 1 | start부터 plan phase까지 | `plan.architecture` | `architecture_artifact` |
| 2 | plan을 먼저 스테이징 | `plan.architecture` | `architecture_artifact` |
| 3 | architecture 스테이징 | `plan.architecture` | `architecture_review` |
| 4 | 한 절만 있는 문서로 리뷰 기록 | 거부 | `architecture is missing required sections: ## 데이터와 API 계약, ## 구조와 흐름, ## 병목과 장애, ## 절충점과 기각한 대안` |
| 5 | 다섯 절 문서로 승인(`--reviewer-context inline`) | `plan.review` | `devils_advocate_review` |
| 6 | 승인 뒤 문서를 고쳐 재스테이징 | `plan.architecture` | `architecture_review_stale` |

기록된 `reviewed_architecture_digest`는 문서 파일의 `shasum -a 256`과 같았다(`5218a0a2…`). `issueops design review`는 종료 코드 2로 실패했다.
옛 바이너리(source checkout의 `bin/issueops`)가 `design_review`를 기록한 임시 state를 새 바이너리의 `status`로 읽으면 `invalid state`였다(하위 호환 서술과 같다).

`self-verify`: source checkout의 `scripts/install-native.sh`는 워크트리에서 실행해도 git common dir로 ROOT를 source checkout으로 잡아
그 스킬 링크를 임시 HOME에 만든다(source checkout 파일은 바뀌지 않았다). 그래서 작업 트리 내용을 `.git`이 따로 있는 비 symlink 경로
복사본(`rsync` 뒤 `git init`)에 옮기고, CI와 같은 방식으로 임시 HOME에 `install-native.sh --skip-build --path-mode=skip --mcp-transport=stdio`를 실행한 뒤
`PATH=<CI와 같은 venv>:$PATH LANG=en_US.UTF-8 ISSUEOPS_STATE_DIR=<임시> ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`을 돌렸다.
결과는 `ok: true`, `termination_eligible: true`, 27/27 step 통과다. 앞선 시도의 실패는 환경 문제였다(`bin/issueops` 14분 낡음 → 재빌드, 실제 HOME에 `architecture-review` 링크 없음 →
임시 HOME 설치, `/tmp`가 `/private/tmp` symlink라 installcli 테스트 실패 → symlink 없는 경로).

LLM 프롬프트 본문 변경(`architecture-review` 렌즈, `issueops-review --target architecture`)의 실패 기준은 판정(`pass|revise|stop`)이 없거나
필수 결함의 위반한 계약 칸이 비는 것이다. 확인 산출물: 이 사이클의 설계를 다섯 절 문서로 적어(`--target architecture`) 새 스킬 본문을 읽은 빈 컨텍스트
서브에이전트(`omp -p`, `anthropic/claude-opus-5-5`, effort high)에 검토시켰다. 출력에 판정 `proceed`가 있었고 절마다 근거 한 문장과 다섯 줄 evidence 블록이 있었으며
필수 결함이 없어 위반한 계약 칸이 비는 경우도 없었다. 같은 출력이 막지 않는 메모 세 건을 남겼다(실행 전 읽기 실패 시 키 문구, 파일 경로 누락, archive 문서의 옛 이름 한 건).

구현 diff 리뷰는 같은 모델·effort의 빈 컨텍스트 서브에이전트가 두 라운드를 돌려 모두 `proceed`였다. 1라운드는 막지 않는 메모 일곱 건을 남겼고 그중 셋을 고쳤다:
실행 뒤 사이클의 불필요한 digest 계산(`service.go`), 리뷰 기록 digest와 봉인 파일 digest의 일치를 production 봉인 경로로 확인하는 테스트
(`TestSealedArchitectureMatchesTheDigestBoundAtReview`), `issueops-plan`의 `inline` 설명. 2라운드는 delta 리뷰였다.

## 성능 측정

`issueops next`의 읽기는 늘지 않았다. staged artifact를 읽는 sqlite `Get` 한 번을 이름 목록에서 이름→본문 map으로 바꿨고,
architecture 문서가 스테이징돼 있고 execution이 아직 없을 때만 그 본문의 sha256을 한 번 계산한다(문서 크기에 비례). 첫 구현 리뷰가 실행 뒤 사이클에서도 계산한다고 지적해 `record.Execution == nil` 가드를 더했다. 실행 뒤 readiness는
봉인 파일 읽기 한 번이 더해진다. 별도 시간 측정은 하지 않았고 추가 I/O가 위 두 가지뿐임을 코드로 확인했다.

## Side effect와 정리

- CLI: `issueops design review`와 `--proposed-design`이 사라지고 `issueops architecture review`, `--proposed-architecture`,
  필수 `--reviewer-context subagent|inline`이 생겼다. 별칭은 없다.
- record: 최상위 키 `design_review`가 `architecture_review`로 바뀌었다. 옛 키가 있는 record는 `invalid state`다. 변환 명령은 없다.
  이 사이클의 record에는 옛 키가 남아 있어 lifecycle 명령을 끝까지 옛 바이너리로 실행했다.
- 스킬: `skills/design-review`가 `skills/architecture-review`로 옮겨졌다. 사용자 홈의 옛 링크는 다음 `issueops update`가 정리한다.
- 파일: `.issueops/issues/570/architecture.md`는 이 사이클이 아키텍처를 스테이징하지 않아 만들어지지 않는다. 이후 사이클의 구현 진입·종료 전이가 봉인 architecture에서 이 사본을 쓴다.
- 원격: 이슈 #570 본문을 사용자 지시에 따라 한 번 갱신했다(`contract_change` feedback과 `mark-issue-updated` 기록).
- 롤백: PR revert. 새 바이너리가 쓴 `architecture_review` record는 옛 바이너리에서 `invalid state`이므로 revert 뒤에는 그 사이클을 새로 시작한다.

## UI 판단

해당 없음. 화면 변경이 없다.
