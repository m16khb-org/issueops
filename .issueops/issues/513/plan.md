# 이슈·PR을 사람이 읽는 문서로 만든다: 구현 계획 (3차)

- lifecycle ID: `io-7426b49dc042`
- 이슈: https://github.com/m16khb-org/issueops/issues/513
- 브랜치: `513-readable-issue-pr-bodies` (base `main` @ `92bbbddabb9bbee9c7a1e050fc6fe061d7d45201`)
- 설계: `docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md` (로컬 main 커밋 `fbf2c553`, 원격 미반영)
- 사용자 요청 범위: 승인된 설계를 구현하는 전체 사이클. 종료점은 draft PR 발행과 `execution complete`다. merge, post-merge 정리(`reflect-completion`, `close-issue`, `cleanup finish`)는 범위 밖이다.
- 구조: PR 하나. 묶음 1(사람이 쓰는 본문)과 묶음 2(하네스가 붙이는 구간과 구현 자료)를 순서대로 구현하고 묶음마다 커밋을 나눈다. child 위임은 하지 않는다(record의 scope 결정 `no split: PR 하나, 작업 묶음별 커밋`).
- 세션 인계: 이 계획 단계 끝에서 `execution prepare --mode direct`로 워크트리를 준비한 뒤, Orca가 ready이므로 같은 워크트리의 새 세션으로 자동 인계한다. 인계는 요청 범위를 넓히지 않는다.

## TL;DR

> **요약**: 이슈·PR 본문을 요약이 먼저 오는 짧은 계약으로 바꾸고, 해시·plan 원문 같은 구현 자료를 본문에서 빼며, 가독성 검사를 `issueops remote` 게시·동기화 명령 안에서 실행한다. 구현 자료는 `.issueops/issues/<n>/`에 추적되는 사본으로 남긴다.
> **산출물**: 새 본문 계약(`artifacttemplate`), 새 가독성 검사 패키지(`artifactreadability`), 게시·동기화 명령의 검사 집행, 사람이 쓴 진행 결과 구간과 한 줄 계획 검토 구간, 원격 본문을 쓰지 않는 정리 감사, 구현 진입·종료 전이가 쓰는 구현 자료 사본, 작성 지침과 스킬·저장소 템플릿·문서 갱신, 새 ADR
> **규모**: Large
> **병렬**: NO (묶음 2가 묶음 1의 검사 함수를 쓴다)
> **Critical Path**: P0 → T1 → T2 → T3 → T4~T8 → T9~T14 → P2 → P3

## Context

### Original Request

사용자는 이슈와 PR을 사람이 이해하기 쉽게 만드는 쪽을 강화하려 한다. explain식 설명과 fluent-korean을 활용하고 잘 만든 이슈를 참고하라고 했다. 설계 중에 두 가지를 분명히 했다. 이슈와 PR은 사람이 읽는 문서이므로 해시 같은 값을 넣지 않는다. 하네스는 사람이 읽기 좋게 정리된 이슈와 PR을 만들고, 구현 자료는 `.issueops/` 아래에 둔다. 사용자는 설계 문서를 승인했고 커밋과 사이클 시작을 요청했다(원문은 record의 `intent.raw_request`).

### Interview Summary

- 설계 방향 B(본문 계약 재설계와 게시 명령 안의 검사)를 사용자가 골랐다.
- 구현 자료는 이슈 본문이 아니라 `.issueops/issues/<n>/`에 둔다(사용자 의도).
- 독자 검토(맥락 없는 subagent)는 권장 절차로만 둔다.
- 계획 리뷰 1라운드 D4 뒤 사용자가 "PR 하나, 묶음별 커밋"을 골랐다.

### Gap Analysis

설계 문서와 다르게 정한 것과 계획 리뷰 1라운드(판정 revise, 지적 D1~D7)에서 확인한 제약이다. 구현은 이 절을 설계 문서보다 우선한다.

1. **진행 결과 반영 시점(D1)**: `reflect-completion`은 provider가 확인한 머지 증거를 요구한다(`internal/adapter/issueops/issueops_completion_remote.go:45`). 그래서 진행 결과 반영은 지금처럼 머지 후 정리 단계(`issueops-cleanup`)에 둔다. `--confirm`이면 `--body-file`을 요구하고, 원고는 정리 단계의 에이전트가 PR 본문과 record를 바탕으로 쓴다. `issueops-complete`와 이 사이클의 P3는 `reflect-completion`을 실행하지 않는다.
2. **완료 구간을 다시 쓰는 경로 제거(D1)**: `cleanup finish`(`issueops_cleanup_finish.go:210-222`)와 `cleanup remote-branch`(`issueops_cleanup_remote_branch.go:95,110-117`)는 감사 줄을 붙이려고 완료 구간 전체를 스냅샷으로 다시 렌더한다. 새 렌더는 사람이 쓴 원고를 모르므로 이 두 경로의 원격 쓰기를 없애고, 감사 문자열은 각 명령의 JSON 응답 `audit`에만 남긴다. record는 finish 마지막에 삭제되므로 record 저장은 의미가 없다.
3. **남긴 원격 브랜치의 추적(D7b)**: `--keep-remote-branch` 면제는 "이슈 본문의 감사 줄이 남긴 브랜치의 유일한 기록"이라는 전제였다(`issueops_cleanup_finish.go:61-62,357-366`). 브랜치 이름은 `branch prepare`가 `<이슈 번호>-<slug>`로 강제하고 provider가 이슈에 연결해 보여 주므로(GitHub linked branch, GitLab `<iid>-` 접두), 남긴 브랜치는 이슈에서 찾을 수 있다. 이 근거를 새 ADR에 적고 면제는 유지한다.
4. **`--template` 요구 범위(D2)**: owner packet이 렌더하는 `remote create-pr` 명령은 `--template`이 없다(`internal/adapter/issueops/execution_owner_context.go:391-394`). create-pr과 create-child는 템플릿이 하나뿐이므로(`internal/domain/artifacttemplate/template.go:146-159`) 생략하면 그 템플릿을 쓰고, `--confirm`에서 `--template`을 요구하는 것은 create-issue뿐이다. owner packet 명령은 바꾸지 않는다.
5. **구현 자료 위치(D3, R2-1)**: 추적 `.gitignore:36`이 `.issueops/issues/*/artifact/`를 무시하고 CONVENTIONS 표(`.issueops/CONVENTIONS.md:100-127`)가 이를 정한다. 봉인 디렉터리와 무시 규칙은 그대로 둔다.
   - 추적 사본(`plan.md`, `intent.md`, 봉인 spec이 있으면 `spec.md`, 반론 검토 기록이 있으면 `plan-review.md`)은 `.issueops/issues/<n>/`에 쓴다.
   - 쓰는 시점은 `phase --to implement`와 `phase --to ai-slop-clean` 전이가 성공한 직후다. prepare가 direct(`internal/adapter/issueops/issueops_artifact_stage.go:213`)와 Orca(`internal/adapter/issueops/execution_prepare_bridge.go:136`) 모두에서 `plan_path`를 봉인 경로로 미리 채워 link-plan이 생략되므로(`skills/issueops-implement/SKILL.md:99`, `internal/adapter/issueops/execution_owner_context.go:367-370`), link-plan은 쓰는 지점이 될 수 없다. 두 전이는 두 모드가 모두 지난다(direct는 `skills/issueops-implement/SKILL.md:114`, Orca는 owner packet `execution_owner_context.go:379`와 `--to ai-slop-clean`).
   - 내용이 같으면 쓰지 않고, 다르면 현재 계획과 record를 기준으로 덮어쓴다(사본은 파생물이다). 종료 전이에서 다시 맞추므로 구현 중 계획이 바뀌어도 사본이 따라간다.
   - 구현 진입 뒤 첫 커밋 전까지는 미추적 사본 때문에 `execution switch-mode`·`cleanup abandon`이 `worktree_clean`에 걸릴 수 있다. 구현 스킬이 사본을 첫 커밋에 포함하게 하고, abandon은 기존 dirty 워크트리 선택지를 쓴다.
   - 이 변경 전에 구현에 들어간 사이클은 사본이 없다. `cleanup finish` preview와 `reflect-completion`은 봉인 plan이 있는데 `.issueops/issues/<n>/plan.md`가 없으면 warning `tracked_materials_missing`을 낸다. 전환 규칙은 새 ADR에 적는다.
   - CONVENTIONS 표에 `intent.md`, `plan-review.md` 행을 추가한다.
6. **반론 검토 반영(D5)**: `reflect-devils-advocate`는 렌더만 바꾸고 critical 거부를 넣지 않는다. 1차 통과 구간은 한글 20자 하한보다 짧고, `regress`는 중단 판정의 반영을 전제로 한다(`internal/adapter/issueops/issueops_regress.go:76-77`).
7. **라벨 판단 위치**: `plan-prep record --related-score-ref`가 점수 요약을 record에 이미 저장한다. create-issue 스킬의 순서를 `remote score → plan-prep record → phase grill → remote create-issue`로 고정하고 본문의 라벨 점수 절만 없앤다.
8. **provider DTO 해석(D7a)**: 비목표 "provider 어댑터 API 변경 없음"은 `IssueProvider` 메서드 집합과 provider CLI 호출 방식을 바꾸지 않는다는 뜻으로 해석한다. 관리 구간 DTO(`port.IssueProviderCompletionSection`, `IssueProviderUpdateIssueBodySectionRequest`)의 필드는 바꾼다. 이 해석을 record decision으로 남긴다.
9. **구조 판정의 단일 소유(리뷰 관문 2)**: 요약 절, 필수 절, 자리 표시 판정은 `artifacttemplate`이 소유한다. `artifactreadability`는 그 결과를 자기 Report에 옮겨 담을 뿐 같은 판정을 다시 구현하지 않는다.
10. **범위 검사 게이트**: diff 경로를 제한하는 Go 게이트는 없다. 사이클별 원장에서만 경로를 제한하므로, 게이트 작성 지침에 `.issueops/issues/<n>/`를 허용 경로로 적는다.
11. **preview 표시 잘림**: `DryRunPreview`(`internal/adapter/provider/providerutil/bounded_command.go:37`)가 4,096바이트에서 UTF-8 글자 중간을 잘라 한글이 깨진다. 실제 요청 본문은 잘리지 않는다. 후속 작업으로 남긴다.
12. **#513 본문과 intent(R2-3)**: 구조를 child 2개에서 PR 하나로 바꾼 것은 본문 사실의 변경이다. 2026-09-24에 intent 해석을 다시 기록했고(`recorded_at` 2026-09-24T01:50:59Z), #513 구현 범위에 대해 `feedback add --classification contract_change`를 기록했다. 그래서 기존 `contract_feedback_issue_update` 게이트가 P3의 `sync-issue`와 `feedback mark-issue-updated`를 강제한다(`internal/adapter/issueops/issueops_feedback.go:103-106`). #513은 새 계약이 생기기 전이라 기존 13절 계약으로 게시됐고, 같은 sync에서 새 형식으로 다시 쓴다.
13. **진행 결과와 중단 지적의 값(R2-2)**: 성공 기준 4는 사람이 쓴 내용에도 적용한다.
   - `reflect-completion` 원고(템플릿 없는 completion 입력)에서는 `local_path`, `commit_sha_full`을 critical로 올리고, 2,000자를 넘으면 critical `result_too_long`을 낸다. plan 원문을 붙이는 것을 막는 상한이다. 이 명령은 머지 뒤에 쓰므로 regress 경로와 관계가 없다.
   - 중단 판정의 지적 목록은 렌더할 때 64·40자리 hex와 로컬 절대 경로를 가린다(`[해시 생략]`, `[로컬 경로 생략]`). 거부하지 않으므로 D5 결정은 유지된다.
14. **create-issue 거부 시점(R2-4b)**: 계약·가독성 critical 거부는 `BeginIssueCreateIntent`(`cmd/issueops/issueopscli/remotecmd/remote_issue.go:107`) 전에 일어나야 한다. intent가 봉인된 뒤에는 같은 요청만 재시도할 수 있어(`internal/adapter/issueops/issue_create_intent.go:34-37`) 고친 본문으로 다시 만들 수 없기 때문이다. `resolveTemplateBody`(62행 호출)가 이미 intent 기록보다 앞에 있으므로 검사를 그 안에 두고, 거부 뒤 record에 `issue_create_intent`가 없음을 테스트로 고정한다.

## 적용되는 결정과 주의사항

| 문서 | 항목 | 이 계획에 미치는 제약 |
|---|---|---|
| `.issueops/CONSTITUTION.md` | 제2장 안전 불변식 | 핵심 정책(가독성 검사)은 host adapter나 스킬이 아니라 Go core에 둔다. |
| `.issueops/CONSTITUTION.md` | 제2장 문제 해결 원칙 | 문서만 고치는 대응으로 끝내지 않는다. 검사를 게시 경로 안으로 옮겨 권고로만 남은 검사라는 근본 원인을 제거한다. |
| `.issueops/CONSTITUTION.md` | 제3장 아키텍처 원칙 | 검사 규칙은 I/O 없는 `internal/domain`에, 원격 본문 읽기와 사본 쓰기는 adapter에, JSON 출력은 CLI 계층에 둔다. |
| `.issueops/CONSTITUTION.md` | 제5장 하네스 검증 불변식 | CLI 계약 변경은 golden과 명령 smoke로 검증한다. |
| `.issueops/CONVENTIONS.md` | 이슈 산출물 레이아웃(100-127행) | `artifact/`는 무시, `plan.md`·`gates.md`·`spec.md`는 추적이 유일한 규정이다. 새 추적 파일(`intent.md`, `plan-review.md`)은 이 표에 먼저 추가한다. |
| `.issueops/CAUTIONS.md` | 게이트 원장 `CHECK:`는 argv 한 줄 | 복합 검사는 `python3 -c` 하나로 감싸고, EXPECT가 있어도 CHECK는 exit 0이어야 met이다. |
| `.issueops/cautions/issueops-lifecycle.md` | §16.1 | "한국어 검사는 `issueops remote` 명령 안에서 실행된다"는 문장에 구현을 맞추고 문장을 정확히 고친다. |
| `.issueops/cautions/issueops-lifecycle.md` | §26 | create-issue의 라이브 검증 경로(`VerifyRemoteArtifactLive`)는 바꾸지 않는다. |
| `.issueops/cautions/issueops-lifecycle.md` | §27 | `.issueops/*.md`를 고친 커밋에 response-contract golden 재생성을 포함한다. 문서만 바꾼 커밋은 diff가 `docs_index`뿐인지 확인한다. |
| `.issueops/cautions/issueops-lifecycle.md` | §28 | critical 검사를 `--confirm`에 걸면 create/sync fixture가 넓게 깨진다. 공유 fixture 본문을 새 계약으로 바꾸고 커밋 전 `go test ./...` 전체를 돌린다. |
| `.issueops/cautions/issueops-stages.md` | §3 | 스펙 커밋 cherry-pick은 `execution prepare`가 워크트리를 만든 뒤(구현 단계)에만 한다. |
| `.issueops/cautions/issueops-stages.md` | §6 | 파일 변경은 4·5단계에서 끝낸다. 구현 진입·종료 전이가 쓰는 사본도 fingerprint에 들어가므로 커밋에 포함한다. 종료 전이는 정리 단계의 봉인보다 앞선다. |
| `.issueops/conventions/cli-mcp-and-output.md` | `--json` snake_case, schema 변경은 golden 동반 | 새 JSON 필드(`readability`, `live_readability`, `audit`)는 snake_case로 쓰고 usage·response golden을 함께 갱신한다. |
| `.issueops/TESTING.md` | 최소 완료 기준, 부분 검증 상태 금지 | 최종 검증은 한 run에서 전 게이트를 통과한 증거로 보고한다. 긴 게이트 때문에 `gates check --timeout-seconds 900`으로 실행한다(기본 120초, `internal/contract/gates/types.go:17`). |
| `skills/issueops-complete/SKILL.md` | 174행 "머지하지 않는다" | 이 사이클은 draft PR과 `execution complete`에서 멈춘다. |
| `.issueops/ADR.md` → `adr/2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact.md` | 추적되는 INTENT.md 기각(리뷰어 가시성이 필요해지면 재검토) | 추적 사본(`intent.md` 포함)은 이 결정을 바꾸므로 새 ADR로 기록한다. |
| `.issueops/ADR.md` → `adr/2026-09-08-issueops-project-doc-gates-link-plan-checks-the-four-plan-se.md` | link-plan이 네 필수 절을 검사 | 이 계획의 네 필수 절 제목을 바꾸거나 합치지 않는다. link-plan은 바꾸지 않는다. |
| `.issueops/ADR.md` → `adr/decisions/2026-08-28-issueops-devils-advocate-plan-binding.md` | 판정은 계획 digest에 묶인다 | 계획을 고치면 반론 검토를 다시 받는다. |
| `.issueops/architecture/issueops.md` | 42행 제공 표면, 178행 정리 순서 계약 | completion 구간 내용 설명을 진행 결과 구간으로 바꾸고, 정리 순서(reflect → close-issue → finish)는 유지한다. |

## 재사용하는 기존 구현

| 기존 구현 | 위치 | 재사용 방식 |
|---|---|---|
| 템플릿 렌더·검증 | `internal/domain/artifacttemplate/template.go` (`Render`, `Validate`, `requiredFields`, `fieldSatisfiedByBody`, `normalizeFields`, 별칭 맵 220-242행) | 새 패키지를 만들지 않고 이 패키지의 절 정의를 새 계약으로 바꾼다. 별칭 맵을 확장해 옛 `--field` 키를 새 절로 받는다. 구조 판정(요약 절, 필수 절, 자리 표시)의 유일한 소유자로 둔다. |
| 한글 비율 판정 | `skills/issueops-remote-write/scripts/remote_artifact_gate.py` | 정규식(코드 fence, inline code, URL, 경로 제거)과 임계값(한글 20자, 영어/한글 1.2)을 Go로 그대로 옮긴다. |
| 관리 구간 식별 | `internal/domain/issueopsbodysync/bodysync.go` (`ManagedRegions`) | 검사가 작성 본문만 보도록 관리 구간을 뺄 때 쓴다. |
| 본문 해석 진입점 | `cmd/issueops/issueopscli/remotecmd/remote_child_pr.go:273` (`resolveTemplateBody`) | create-issue·create-child·create-pr이 공유하는 이 함수에 검사 호출을 한 번만 넣는다. |
| sync의 원격 본문 읽기 | `internal/adapter/issueops/issueops_remote_body_sync.go:66` (`ReadArtifactBody`) | 이미 읽은 원격 본문에 검사를 적용해 `live_readability`로 돌려준다. 원격 호출을 추가하지 않는다. |
| 관리 구간 렌더·병합 | `internal/adapter/provider/issuebody/issue_body_section.go` (`RenderCompletionSection` 66행, `RenderDevilsAdvocateSection` 42행, `MergeManagedSection`, `SectionBudget`) | 경계 표지와 병합 규칙은 그대로 두고 렌더 내용만 바꾼다. |
| 완료 구간 데이터 수집 | `internal/adapter/issueops/issueops_completion_remote.go:129` (`gatherCompletionSection`) | plan·spec·manifest 수집을 없애고 PR URL만 모은다. 원고는 요청으로 받는다. |
| 계약 피드백 게이트 | `internal/adapter/issueops/issueops_pr_readiness.go:68` | 결정 변경 반영에 새 Go 게이트를 만들지 않고 이 게이트를 쓴다. |
| phase 전이 | `internal/adapter/issueops/issueops_phase.go:24-101` (`AdvanceIssueOpsPhaseWithActor`, `advanceIssueOpsPhaseLocked`) | 새 명령을 만들지 않고 `implement`·`ai-slop-clean` 전이가 성공한 뒤 추적 사본을 쓴다. 전이 검증 규칙은 그대로다. link-plan은 바꾸지 않는다. |
| 봉인 artifact 경로 | `internal/adapter/issueops/issueops_artifact_stage.go` (`sealedArtifactPath` 40행, `issueArtifactDirFor` 46행) | 사본의 원본 경로와 이슈 폴더 계산에 그대로 쓴다. |
| 원격 본문 표지 | `port.IssueBodyCompletionStartMarker`, `issueCreateMarker` | 형식을 바꾸지 않는다. 정리 단계의 확인(`cmd/issueops/issueopscli/feedbackcleanup/feedback_cleanup.go:253`)이 이 표지에 의존한다. |
| 게이트 시간 제한 | `issueops gates check --timeout-seconds` | 긴 게이트를 위해 새 기능을 만들지 않고 기존 플래그를 쓴다. |

새로 만드는 것은 `internal/domain/artifactreadability` 하나다. 한국어 비율, 해시, 하네스 용어, AI 티 패턴은 템플릿과 무관하게 모든 본문(진행 결과 원고 포함)에 적용되므로 별도 순수 패키지로 둔다.

## 성능 영향

- hot path가 아니다. 게시·동기화 명령 한 번에 본문 하나(수 KB)를 한 번 훑는다.
- 검사는 본문 길이 n에 대해 O(n) 정규식 스캔 몇 번과 문장 집합 구성(O(n))이다. 원격 호출을 추가하지 않는다. sync의 원격 본문 검사도 이미 읽은 문자열을 쓴다.
- 구현 진입·종료 전이 때 작은 파일 최대 네 개를 쓴다. 내용이 같으면 쓰지 않는다.
- 완료 구간이 plan·spec 전문을 싣지 않으므로 원격 쓰기 payload가 작아진다.
- 측정: 50KB 본문 검사가 10ms 안에 끝나는지 단위 테스트(`TestCheckLargeBodyIsFast`)로 확인한다.

## 하위 호환성과 side effect

- **CLI 입력**
  - `remote create-issue`는 `--confirm`일 때 `--template`을 요구한다. `create-child`·`create-pr`은 생략하면 `child_task`·`pull_request`를 쓴다. preview는 모든 명령에서 검사 결과를 보여 준다.
  - `remote reflect-completion`은 `--confirm`일 때 `--body-file`을 요구한다. 머지 증거 요구는 그대로다.
  - 옛 `--field` 키는 별칭으로 계속 받는다. 본문에 렌더하지 않는 키(`worktree_cleanup`, `scope_management`, `change_type`, `automation_evidence`, `feedback_log`)는 받되 warning으로 알린다.
- **CLI JSON 출력**: create·sync·reflect-completion 응답에 `readability` 객체를 추가하고, sync preview에 `live_readability`를 추가한다(additive). `cleanup finish`·`cleanup remote-branch` 응답에서 `audit_reflected`·`audit_error`를 빼고 `audit` 문자열을 넣는다. 이 키를 읽는 스킬과 테스트를 같은 커밋에서 고친다.
- **port 타입**: `IssueProvider` 메서드 집합은 그대로다. `IssueProviderCompletionSection`은 렌더 대상 필드를 줄이고 `ResultBody`를 더하며, `IssueProviderUpdateIssueBodySectionRequest`에 반론 검토 판정·라운드 필드를 더한다. provider fake와 두 provider 구현을 함께 고친다.
- **record 스키마**: 바꾸지 않는다. record 디코더는 모르는 필드를 거부한다(`internal/adapter/outbound/issueopsrecord/codec.go:28`).
- **golden**: `usage.golden.txt`(create-issue·reflect-completion 설명), `response_contracts.golden.json`(`issueops_remote_create_issue/child/pr`, `issueops_remote_render_template`, `issueops_remote_reflect_devils_advocate` 항목과 `.issueops/*.md` 편집에 따른 `docs_index`)을 해당 커밋에서 재생성한다.
- **원격 본문**: 새 계약은 새로 게시하거나 동기화하는 본문에만 적용한다. 이미 게시된 본문은 자동으로 고치지 않는다. 관리 구간 표지 형식은 그대로라 기존 완료 구간도 sync 때 보존된다. 정리 명령은 더 이상 이슈 본문을 쓰지 않는다.
- **추적 사본**: `implement`·`ai-slop-clean` 전이가 `.issueops/issues/<n>/`에 파일을 쓴다. 같은 내용이면 쓰지 않고, 다르면 현재 계획과 record를 기준으로 덮어쓴다(사본은 원본의 파생물이다). 봉인 디렉터리와 무시 규칙은 바뀌지 않는다. 첫 커밋 전까지는 미추적 사본이 `worktree_clean`을 막을 수 있다(Gap Analysis 5).
- **프롬프트 변경**: LLM 프롬프트 템플릿은 바꾸지 않는다. 독자 검토 절차는 스킬 산문이고, 실패 기준은 "답이 `intent.md`의 해석·성공 기준과 다르다" 또는 "모르는 용어가 1개 이상"이다.
- **롤백**: 묶음마다 커밋이 나뉘므로 묶음 2 커밋만 되돌리면 완료 구간과 정리 경로가 옛 동작으로 돌아간다. 묶음 1까지 되돌리면 계약과 검사가 옛 동작으로 돌아간다.

## Work Objectives

### Core Objective

팀원이 이슈·PR 본문의 요약만 읽고 무엇이 왜 바뀌는지, 무엇이 달라지는지, 어떻게 확인했는지 알 수 있게 하고, 하네스가 이 형식을 건너뛸 수 없게 한다.

### Deliverables

- 새 본문 계약과 저장소 템플릿
- `internal/domain/artifactreadability`와 게시·동기화·반영 명령의 집행
- 사람이 쓴 진행 결과 구간, 한 줄 계획 검토 구간, 원격 본문을 쓰지 않는 정리 감사
- 구현 진입·종료 전이의 추적 사본, 사본 누락 warning, CONVENTIONS 갱신
- 새 ADR, 아키텍처·운영·주의사항 문서 정정
- 작성 지침(`skills/issueops-remote-write/references/readable-body.md`)과 관련 스킬 갱신

### Definition of Done

- 아래 게이트 G1~G28이 한 run에서 모두 met이다(`gates check --write --timeout-seconds 900`).
- PR 본문이 새 계약으로 게시되고 critical 0건이다.
- #513 본문이 새 계약 형식으로 동기화됐다.

### Must Have

- critical 4개(한국어 비율, 요약 절, 필수 절, 코드 밖 SHA-256)와 warning 9개(설계 5절). 진행 결과 원고에서는 `local_path`·`commit_sha_full`을 critical로 올리고 `result_too_long`(2,000자)을 더한다(Gap Analysis 13).
- 관리 구간 표지 형식과 정리 단계의 표지 확인 유지
- 봉인 디렉터리와 무시 규칙 유지

### Must NOT Have

- record 스키마 변경, `IssueProvider` 메서드 집합 변경, lease·generation·CAS 절차 변경
- 문장 자연스러움의 기계 판정, 독자 검토의 게시 조건화, `reflect-devils-advocate`의 critical 거부
- 이미 게시된 이슈·PR의 일괄 수정
- 비공개 저장소의 문장과 수치를 작성 지침·테스트 fixture에 넣는 것
- merge, `reflect-completion`, `close-issue`, `cleanup` 실행

## Verification Strategy

> 모든 검증은 에이전트가 실행한다.

- 테스트 방식: TDD(RED→GREEN→SURFACE→CLEAN). Go `testing` 패키지를 쓴다.
- 게이트 원장: `.issueops/issues/513/gates.md`. 구현 진입 때 아래 spec 하나로 `gates init`을 실행한다.
- 실행: 게이트 전체는 `issueops gates check --write --timeout-seconds 900`으로 한 run에 채운다. G24는 원장을 봉인하기 전에 한 번 실행해 환경 문제로 실패하는지 먼저 본다. 환경 문제(예: 로컬 Omo MCP 설정 부재)로 실패하면 그 사실과 출력을 보고하고 사용자 판단 없이 게이트를 지우지 않는다.

### 게이트 (gates-ledger `--gate` 형식)

```text
G1: 스펙 문서가 브랜치에 있다 | CHECK: test -f docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md
G2: Go 전체 테스트 | CHECK: go test ./... -count=1
G3: race 테스트 | CHECK: go test -race ./... -count=1
G4: go vet | CHECK: go vet ./...
G5: gofmt 깨끗함(커밋 전 파일 포함) | CHECK: python3 -c "import subprocess,sys; fs=subprocess.run(['git','ls-files','--cached','--others','--exclude-standard','*.go'],capture_output=True,text=True).stdout.split(); out=subprocess.run(['gofmt','-l']+fs,capture_output=True,text=True).stdout.strip() if fs else ''; print(out or 'clean'); sys.exit(1 if out else 0)" | EXPECT: clean
G6: 계약 golden | CHECK: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
G7: PR 계약 필수 절 4개 | CHECK: go test ./internal/domain/artifacttemplate -run TestPullRequestContractRequiresFourSections -count=1
G8: 구현 이슈 계약 필수 절 5개 | CHECK: go test ./internal/domain/artifacttemplate -run TestImplementationIssueContractRequiresFiveSections -count=1
G9: 가독성 검사 규칙 | CHECK: go test ./internal/domain/artifactreadability -count=1
G10: create-issue confirm이 critical과 템플릿 누락을 intent 기록 전에 거부 | CHECK: go test ./cmd/issueops/issueopscli/remotecmd -run TestRemoteCreateIssueRefusesCriticalReadability -count=1
G11: create-child·create-pr confirm이 critical을 거부하고 기본 템플릿을 쓴다 | CHECK: go test ./cmd/issueops/issueopscli/remotecmd -run TestRemoteCreateChildAndPRRefuseCriticalReadability -count=1
G12: sync-issue·sync-pr confirm이 critical을 거부하고 원격 본문 warning을 보고 | CHECK: go test ./internal/adapter/issueops ./cmd/issueops/issueopscli/remotecmd -run TestRemoteSyncRefusesCriticalAndReportsLiveReadability -count=1
G13: reflect-completion이 원고를 요구하고 SHA·로컬 경로·과다 길이를 거부 | CHECK: go test ./cmd/issueops/issueopscli/remotecmd ./internal/adapter/issueops -run TestReflectCompletionRequiresReadableResult -count=1
G14: 진행 결과 구간에 하네스 값이 없음 | CHECK: go test ./internal/adapter/provider/issuebody -run TestCompletionSectionIsHumanReadable -count=1
G15: 정리 경로가 이슈 본문을 쓰지 않음 | CHECK: go test ./internal/adapter/issueops -run TestCleanupPathsKeepAuditOutOfIssueBody -count=1
G16: 계획 검토 한 줄 요약, 중단 지적 가림, 짧은 중단 판정의 반영·regress | CHECK: go test ./internal/adapter/provider/issuebody ./internal/adapter/issueops -run "TestPlanReviewSectionSummarizesRounds|TestPlanReviewMasksHashesAndPaths|TestShortKoreanStopReflectsAndRegresses" -count=1
G17: implement·ai-slop-clean 전이가 두 모드에서 추적 사본을 쓴다 | CHECK: go test ./internal/adapter/issueops -run TestPhaseTransitionWritesTrackedMaterials -count=1
G18: #513 추적 사본이 있고 무시되지 않는다 | CHECK: python3 -c "import os,subprocess,sys; ps=['.issueops/issues/513/plan.md','.issueops/issues/513/intent.md','.issueops/issues/513/plan-review.md']; bad=[p for p in ps if not os.path.exists(p) or subprocess.run(['git','check-ignore','-q',p]).returncode==0]; print('tracked' if not bad else 'bad: '+','.join(bad)); sys.exit(1 if bad else 0)" | EXPECT: tracked
G19: 저장소 템플릿이 계약과 일치 | CHECK: go test ./internal/domain/artifacttemplate -run TestRepositoryTemplatesMatchContract -count=1
G20: 바뀐 스킬 검증 | CHECK: python3 -c "import subprocess,sys; s=['issueops-remote-write','issueops-create-issue','issueops-create-pr','issueops-sync-issue','issueops-sync-pr','issueops-cleanup','issueops-implement','issueops-review','issueops-plan','issueops','gitlab-usecase','gates-ledger']; bad=[x for x in s if subprocess.run([sys.executable,'scripts/validate-skill.py','skills/'+x],capture_output=True).returncode or subprocess.run([sys.executable,'scripts/verify-skill-shell.py','skills/'+x],capture_output=True).returncode]; print('skills ok' if not bad else 'failed: '+','.join(bad)); sys.exit(1 if bad else 0)" | EXPECT: skills ok
G21: Python 한국어 게이트 제거 | CHECK: python3 -c "import os,sys; p='skills/issueops-remote-write/scripts/remote_artifact_gate.py'; gone=not os.path.exists(p); print('removed' if gone else 'present'); sys.exit(0 if gone else 1)" | EXPECT: removed
G22: 프로젝트 문서 검사기 | CHECK: python3 -c "import os,subprocess,sys; sys.exit(subprocess.run(['uv','run','--directory','skills/project-docs-optimize','python','-m','scripts.check','--root',os.getcwd(),'--mode','check','--json'],capture_output=True).returncode)"
G23: 바이너리 빌드 | CHECK: go build -o bin/issueops ./cmd/issueops
G24: self-verify 기본 게이트 | CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
G25: 진행 결과 반영은 머지 증거를 계속 요구 | CHECK: rg -o "cannot reflect completion without provider-verified merge evidence" internal/adapter/issueops/issueops_completion_remote.go | EXPECT: cannot reflect completion without provider-verified merge evidence
G26: regress는 중단 판정 반영을 계속 요구 | CHECK: rg -o "reflect the devil's-advocate findings to the issue before regressing" internal/adapter/issueops/issueops_regress.go | EXPECT: reflect the devil's-advocate findings to the issue before regressing
G27: 봉인 디렉터리는 계속 무시된다 | CHECK: git check-ignore -q .issueops/issues/513/artifact/plan.md
G28: 사본이 없는 기존 사이클에 warning | CHECK: go test ./internal/adapter/issueops -run TestTrackedMaterialsMissingWarning -count=1
```

## Execution Strategy

### Waves

- Wave 0: P0 스펙 커밋 반입과 원장 생성
- Wave 1(묶음 1, 사람이 쓰는 본문): T1 → T2 → T3 → T4, T5, T6, T7, T8
- Wave 2(묶음 2, 하네스가 붙이는 구간과 구현 자료): T9, T10, T11, T12, T13 → T14
- Wave 3: P2 통합 검증과 단계 전이, P3 게시와 완료

### Dependency Matrix

| Task | Depends On | Blocks |
|---|---|---|
| P0 | execution prepare, phase --to implement | T1 |
| T1 | P0 | T2, T3, T6, T8 |
| T2 | T1 | T3, T9 |
| T3 | T1, T2 | T4, T5, T7 |
| T4~T8 | T3 (T6·T8은 T1) | Wave 2 |
| T9 | T2, 묶음 1 커밋 | T14 |
| T10~T13 | 묶음 1 커밋 | T14 |
| T14 | T9~T13 | P2 |
| P2 | T14 | P3 |
| P3 | P2 | 종료 |

## TODOs

- [ ] P0. 스펙 커밋을 가져오고 원장을 만든다

  **What to do**: 구현 진입(`phase --to implement`) 뒤 워크트리에서 `git cherry-pick fbf2c553`으로 설계 문서 커밋을 가져온다. 이어서 위 게이트 spec 하나로 `issueops gates init --file "$WORKTREE/.issueops/issues/513/gates.md" --scope 513 ...`을 실행하고 G1을 채운다.
  **Must NOT do**: source checkout의 main을 되돌리거나 푸시하지 않는다. 워크트리를 만들기 전에 커밋하지 않는다(stages §3).
  **Recommended Agent**: quick. 이유: cherry-pick과 원장 생성이다.
  **Parallelization**: Can Parallel: NO | Wave 0 | Blocks: T1
  **References**: `docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md`, `skills/gates-ledger/SKILL.md`, `skills/issueops-implement/SKILL.md`
  **Acceptance Criteria**:
  - [ ] G1 met
  - [ ] `git -C "$WORKTREE" log --oneline -1 -- docs/superpowers/specs` subject가 `docs(spec): design readable issue and PR bodies`
  **QA Scenarios**:
  ```
  Scenario: 반입 성공
    Channel: bash
    Steps: git -C "$WORKTREE" cherry-pick fbf2c553 && test -f "$WORKTREE/docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md"
    Expected: exit 0
  Scenario: 잘못된 위치
    Channel: bash
    Steps: test "$(git -C "$WORKTREE" branch --show-current)" = 513-readable-issue-pr-bodies
    Expected: exit 0. 다르면 중지하고 보고한다.
  ```
  **Commit**: YES(cherry-pick) + 원장은 묶음 1 첫 커밋에 포함

- [ ] T1. 본문 계약을 새로 정의한다 (`internal/domain/artifacttemplate`)

  **What to do**:
  1. 종류별 필수·선택 절을 설계 2절 표대로 정의한다. canonical 제목: 요약, 배경, 완료 기준, 범위, 검증, 접근 방법과 대안, 위험, 열린 질문, 하위 Task, 재현 절차, 기대 동작과 실제 동작, 원인, 환경과 로그, 제안, 대안과 선택 이유, 선행 조건과 병합 조건, 변경 내용, 확인한 것, 리뷰 포인트, 위험과 되돌리기, 호환성과 마이그레이션, 남은 일.
  2. 구조 판정을 이 패키지가 소유한다: 첫 `## ` 제목이 `## 요약`이고 비어 있지 않은지(`summary_section_missing`), 필수 절이 모두 있는지(`required_section_missing`), 필수 절이 자리 표시(없음, 해당 없음, N/A, TBD, (없음), `-`)만 담는지(`placeholder_section`).
  3. 필드 별칭: `problem`·`current_evidence`→배경, `non_goals`·`implementation_scope`→범위, `intent`→요약, `reviewer_focus`→리뷰 포인트, `risk_rollback`·`risk`·`risks`·`rollback`→위험과 되돌리기(PR)·위험(이슈), `breaking_changes`·`docs_migration`·`user_impact`→호환성과 마이그레이션. 렌더하지 않는 키 목록을 두고 warning `unrendered_field:<key>`를 낸다.
  4. `render-template`이 필수 절 골격을 순서대로 출력한다. `scoreSummary`와 "관련 이슈/라벨 판단" 절을 없앤다.
  5. 이슈 계약의 `## 검증` 제목은 그대로 쓴다(`internal/adapter/issueops/execution_owner_context.go:587`의 추출기 호환).
  6. create-child·create-pr 종류는 템플릿 생략 시 유일 템플릿을 기본값으로 쓰는 헬퍼를 둔다.
  **Must NOT do**: 새 패키지로 옮기지 않는다. `## plan`·GitLab `## related issues` 금지 규칙을 없애지 않는다.
  **Recommended Agent**: deep. 이유: 계약, 별칭, 테스트를 함께 바꾼다.
  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2, T3, T6, T8 | Blocked By: P0
  **References**: `internal/domain/artifacttemplate/template.go:101,146-159,220-242,322,343`, `internal/domain/artifacttemplate/template_test.go`, 설계 2절
  **Acceptance Criteria**:
  - [ ] G7, G8 met
  - [ ] `go test ./internal/domain/artifacttemplate -count=1` 통과
  **QA Scenarios**:
  ```
  Scenario: 새 PR 계약 통과
    Channel: bash
    Steps: 요약·변경 내용·확인한 것·리뷰 포인트만 있는 PR 본문 fixture로 Validate
    Expected: OK=true, Critical=[]
  Scenario: 옛 13절 본문 거부
    Channel: bash
    Steps: `## 의도`로 시작하는 옛 PR 본문 fixture로 Validate
    Expected: Critical에 summary_section_missing
  ```
  **Commit**: YES | `feat(template): define the reader-first body contract` | Files: artifacttemplate 패키지, golden(render-template 항목)

- [ ] T2. 가독성 검사 패키지를 만든다 (`internal/domain/artifactreadability`)

  **What to do**: `Check(Input{Kind, Template, Title, Body}) Report`를 순수 함수로 만든다. `Report{OK bool, Critical []Finding, Warnings []Finding}`, `Finding{Code, Line, Message}`(JSON snake_case). 관리 구간은 `issueopsbodysync.ManagedRegions`로 뺀 뒤 검사한다. 구조 판정은 `artifacttemplate.Validate` 결과를 옮겨 담는다(단일 소유). 자체 규칙은 critical `korean_ratio`, `sha256_hex`와 warning `summary_too_long`(400자), `harness_term`(설계 5절 초기 목록), `local_path`, `commit_sha_full`, `empty_optional_section`, `slop_pattern`, `result_only_pass`, `duplicate_sentence`, `unrendered_field`다. 템플릿이 없는 입력(진행 결과 원고)은 구조 판정 없이 자체 규칙만 적용한다.
  **Must NOT do**: 파일·네트워크 I/O를 넣지 않는다. 구조 판정을 다시 구현하지 않는다. 문장 자연스러움을 판정하는 규칙을 추가하지 않는다.
  **Recommended Agent**: deep. 이유: 규칙 11개와 fixture 테스트다.
  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T3, T9 | Blocked By: T1
  **References**: `skills/issueops-remote-write/scripts/remote_artifact_gate.py`, `internal/domain/issueopsbodysync/bodysync.go`, `skills/fluent-korean/SKILL.md`, `skills/fluent-korean/references/slop-patterns.md`
  **Acceptance Criteria**:
  - [ ] G9 met(규칙마다 통과·실패 fixture, `TestCheckLargeBodyIsFast`, Python 게이트와 같은 판정을 내는 비교 fixture 3건)
  **QA Scenarios**:
  ```
  Scenario: SHA-256 critical
    Channel: bash
    Steps: 코드 밖에 64자리 hex 한 줄이 있는 본문으로 Check
    Expected: Critical에 sha256_hex, Line은 해당 줄 번호
  Scenario: 코드 블록 안 해시는 허용
    Channel: bash
    Steps: 코드 fence 안에만 64자리 hex가 있는 본문으로 Check
    Expected: sha256_hex 없음
  ```
  **Commit**: YES | `feat(readability): add the issue and PR body readability check` | Files: 새 패키지

- [ ] T3. 게시·동기화 명령에 검사를 붙인다

  **What to do**:
  1. `resolveTemplateBody`가 계약 검증과 `artifactreadability.Check`를 항상 실행하고 결과를 호출자에게 돌려준다.
  2. create-issue: `--confirm`인데 `--template`이 없으면 거부한다. create-child·create-pr: 생략 시 유일 템플릿을 쓴다. 세 명령 모두 preview·confirm 응답 JSON에 `readability`를 붙이고(CLI 계층에서 감싸며 port 타입은 그대로), confirm은 critical이 있으면 거부한다. create-issue의 거부는 `resolveTemplateBody`(`remote_issue.go:62`) 안에서 일어나 `BeginIssueCreateIntent`(`remote_issue.go:107`)보다 앞선다. 테스트는 거부 뒤 record에 `issue_create_intent`가 없음을 단언한다(Gap Analysis 14).
  3. sync-issue·sync-pr: 제안 본문에 같은 검사를 적용하고 critical이면 confirm을 거부한다. `--template`이 없으면 제안 본문 제목으로 추론한다(재현 절차→bug, 제안→proposal, `--url` child→child_task, sync-pr→pull_request, 그 밖→implementation_task). preview는 `issueops_remote_body_sync.go:66`에서 읽은 원격 본문도 검사해 `live_readability`(warning만)로 돌려준다.
  4. usage 문구, `usage.golden.txt`, response golden의 `issueops_remote_create_*` 항목을 같은 커밋에서 재생성한다.
  **Must NOT do**: provider 호출을 추가하지 않는다. record에 템플릿 종류를 저장하지 않는다. owner packet의 create-pr 명령(`execution_owner_context.go:391-394`)을 바꾸지 않는다.
  **Recommended Agent**: deep. 이유: 세 명령 경로와 fixture 파급(§28)이 있다.
  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T4, T5, T7 | Blocked By: T1, T2
  **References**: `cmd/issueops/issueopscli/remotecmd/remote_child_pr.go:273`, `remote_issue.go:19`, `remote_body_sync.go:15-59`, `internal/adapter/issueops/issueops_remote_body_sync.go:33-66`, `cmd/issueops/testdata/usage.golden.txt:87-97`
  **Acceptance Criteria**:
  - [ ] G10, G11, G12 met
  - [ ] `go test ./cmd/issueops/... ./internal/adapter/issueops/... -count=1` 통과
  **QA Scenarios**:
  ```
  Scenario: critical 거부
    Channel: bash
    Steps: 요약 절 없는 body-file로 create-issue --template implementation_task --confirm (fake provider)
    Expected: 에러, provider 호출 0회, 오류에 summary_section_missing
  Scenario: owner packet 명령 호환
    Channel: bash
    Steps: --template 없이 새 계약 본문으로 create-pr --confirm (fake provider)
    Expected: pull_request 템플릿으로 검사 후 성공
  ```
  **Commit**: YES | `feat(remote): enforce the body contract and readability check` | Files: remotecmd, adapter, golden

- [ ] T4. Python 한국어 게이트를 제거하고 remote-write 절차를 바꾼다

  **What to do**: `skills/issueops-remote-write/scripts/remote_artifact_gate.py`를 삭제하고, `skills/issueops-remote-write/SKILL.md`의 절차를 "골격 받기 → 작성 → fluent-korean → 독자 검토(권장) → preview 검사 결과 확인 → confirm → readback"으로 바꾼다. 본문 품질 규칙 7개는 작성 지침으로 옮기고, "provider MCP 도구로도 원격에 쓰지 않는다"와 warning 처리 규칙을 추가한다.
  **Must NOT do**: fluent-korean 호출 의무를 없애지 않는다.
  **Recommended Agent**: quick. 이유: 스킬 문서와 파일 삭제다.
  **Parallelization**: Can Parallel: YES(T5~T8) | Wave 1 | Blocked By: T3
  **References**: `skills/issueops-remote-write/SKILL.md`, 설계 7절
  **Acceptance Criteria**:
  - [ ] G20, G21 met, `rg -n remote_artifact_gate skills` 0건
  **QA Scenarios**:
  ```
  Scenario: 스킬 검증
    Channel: bash
    Steps: python3 scripts/validate-skill.py skills/issueops-remote-write
    Expected: exit 0
  Scenario: 잔여 참조 없음
    Channel: bash
    Steps: rg -n remote_artifact_gate skills
    Expected: 0건
  ```
  **Commit**: YES | `docs(skill): move the Korean gate into the remote commands` | Files: remote-write 스킬

- [ ] T5. 작성 지침을 쓰고 게시 스킬을 정리한다

  **What to do**: `skills/issueops-remote-write/references/readable-body.md`를 설계 7절 내용(explain 구조, fluent-korean, 용어 변환표, 공개 모범 사례, 독자 검토, warning 처리)으로 쓴다. `issueops-create-issue`·`issueops-create-pr`·`issueops-sync-issue`·`issueops-sync-pr`에서 절 목록·13절 표·옛 예시를 지우고 `render-template`과 지침을 가리키게 한다. create-issue의 순서를 `remote score → plan-prep → phase grill → create-issue`로 고친다. `skills/issueops/SKILL.md`의 explain 행을 "사람에게 보고할 때와 이슈·PR을 쓸 때"로 넓히고, `skills/gitlab-usecase/SKILL.md`에 사이클 안 glab MCP 쓰기 금지를 넣는다. 게이트 작성 지침 `skills/gates-ledger/SKILL.md`의 경로 규칙 절에 `.issueops/issues/<n>/`의 추적 사본을 허용 경로로 적는다. `.issueops/operations/guides/issueops-providers.md:71`의 create-issue 예시에 `--template`을 넣는다.
  **Must NOT do**: 비공개 저장소의 문장·수치를 예시로 쓰지 않는다. explain·fluent-korean 스킬은 바꾸지 않는다.
  **Recommended Agent**: deep. 이유: 여러 스킬의 일관성을 맞춘다.
  **Parallelization**: Can Parallel: YES | Wave 1 | Blocked By: T3
  **References**: 설계 7절, `skills/explain/SKILL.md`, `skills/fluent-korean/SKILL.md`, PR #495·#489
  **Acceptance Criteria**:
  - [ ] G20 met
  - [ ] `rg -n "관련 이슈/라벨 판단|워크트리 정리|자동화/AI 개입 근거" skills` 0건
  **QA Scenarios**:
  ```
  Scenario: 지침 링크
    Channel: bash
    Steps: rg -n "references/readable-body.md" skills/issueops-create-issue skills/issueops-create-pr skills/issueops-sync-issue skills/issueops-sync-pr
    Expected: 스킬마다 1건 이상
  Scenario: 비공개 사례 유출 없음
    Channel: bash
    Steps: 작성 지침(skills/issueops-remote-write/references/readable-body.md)에서 사내 GitLab 호스트 이름과 사내 저장소 이름을 rg로 찾는다
    Expected: 0건
  ```
  **Commit**: YES | `docs(skill): add the readable body guide and route publication skills to it` | Files: 스킬 문서

- [ ] T6. 저장소 템플릿을 새 계약에 맞춘다

  **What to do**: `.github/ISSUE_TEMPLATE/*.yml`, `.github/pull_request_template.md`, `.gitlab/issue_templates/*.md`, `.gitlab/merge_request_templates/default.md`를 새 계약의 필수 절 순서로 다시 쓴다. `internal/domain/artifacttemplate/repo_templates_test.go`가 종류별 필수 제목이 대응 템플릿에 모두 있는지 읽어 확인한다(테스트 전용 파일 I/O).
  **Recommended Agent**: quick. 이유: 템플릿 파일과 테스트 하나다.
  **Parallelization**: Can Parallel: YES | Wave 1 | Blocked By: T1
  **References**: `.github/ISSUE_TEMPLATE/implementation_task.yml`, `.gitlab/issue_templates/implementation_task.md`, `.github/pull_request_template.md`
  **Acceptance Criteria**:
  - [ ] G19 met
  **QA Scenarios**:
  ```
  Scenario: 템플릿 일치
    Channel: bash
    Steps: go test ./internal/domain/artifacttemplate -run TestRepositoryTemplatesMatchContract -count=1
    Expected: PASS
  Scenario: 누락 감지
    Channel: bash
    Steps: 테스트에서 필수 제목 하나를 뺀 문자열로 비교 함수 호출
    Expected: 누락 제목 보고
  ```
  **Commit**: YES | `docs(templates): align provider templates with the body contract` | Files: .github, .gitlab, 테스트

- [ ] T7. CAUTIONS §16.1을 구현과 맞추고 golden을 갱신한다

  **What to do**: `.issueops/cautions/issueops-lifecycle.md` §16.1의 한국어 검사 문장을 "가독성 검사(한국어 비율 포함)는 `issueops remote`의 게시·동기화 명령 안에서 실행된다"로 고친다. 같은 커밋에서 response golden을 재생성하고 diff가 `docs_index`뿐인지 확인한다.
  **Recommended Agent**: quick. 이유: 문서 한 절과 golden이다.
  **Parallelization**: Can Parallel: YES | Wave 1 | Blocked By: T3
  **References**: `.issueops/cautions/issueops-lifecycle.md:55-65`, §27
  **Acceptance Criteria**:
  - [ ] G6 met, 문서 250줄 이하
  **QA Scenarios**:
  ```
  Scenario: golden 일치
    Channel: bash
    Steps: go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
    Expected: PASS
  Scenario: 문서 예산
    Channel: bash
    Steps: wc -l .issueops/cautions/issueops-lifecycle.md
    Expected: 250 이하
  ```
  **Commit**: YES | `docs(cautions): state that remote commands run the readability check` | Files: cautions, golden

- [ ] T8. 벤치마크 절 개념을 새 계약에 맞춘다

  **What to do**: `internal/adapter/issueops/benchmark/issueops_benchmark_quality.go`의 `issueOpsIssueSectionConcepts`·`issueOpsPRSectionConcepts`와 `cmd/issueops/issueopscli/benchmarkartifact/issueops_benchmark_artifact_sections.go`의 절 목록을 새 계약으로 바꾸고 fixture 테스트를 고친다.
  **Must NOT do**: 벤치마크 점수 체계를 바꾸지 않는다.
  **Recommended Agent**: quick. 이유: 목록 교체와 fixture 수정이다.
  **Parallelization**: Can Parallel: YES | Wave 1 | Blocked By: T1
  **References**: 위 두 파일, `internal/adapter/issueops/benchmark/issueops_benchmark_fixtures_test.go`
  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapter/issueops/benchmark ./cmd/issueops/issueopscli/benchmarkartifact -count=1` 통과
  **QA Scenarios**:
  ```
  Scenario: 새 계약 초안 통과
    Channel: bash
    Steps: 새 계약 PR·이슈 초안 fixture로 benchmark 판정
    Expected: 절 개념 누락 0
  Scenario: 요약 없는 초안
    Channel: bash
    Steps: 요약 절 없는 초안 fixture
    Expected: 요약 개념 누락 보고
  ```
  **Commit**: YES | `test(benchmark): score sections against the body contract` | Files: 두 파일과 테스트

- [ ] T9. 진행 결과 구간을 만든다 (post-merge 반영)

  **What to do**: `remote reflect-completion`에 `--body-file`을 추가하고 `--confirm`이면 필수로 한다. 원고는 `artifactreadability.Check`(completion 입력)를 통과해야 하며, 이 입력에서는 `local_path`·`commit_sha_full`이 critical이고 2,000자를 넘으면 `result_too_long`이다(Gap Analysis 13). 봉인 plan이 있는데 `.issueops/issues/<n>/plan.md`가 없으면 warning `tracked_materials_missing`을 낸다. 머지 증거 요구(`issueops_completion_remote.go:45`)는 그대로다. `port.IssueProviderCompletionSection`에 `ResultBody`를 더하고, `RenderCompletionSection`은 표지 + `## 진행 결과` + `ResultBody`만 렌더한다(최종 head, manifest, Turing 요약, plan·spec, 빈 소제목 제거). `gatherCompletionSection`은 PR URL만 모은다. 원고가 예산을 넘으면 실패한다. `issueops_cleanup_finish.go:47`의 안내 명령과 문서의 호출(`skills/issueops-cleanup/SKILL.md:186-189`, `skills/issueops/references/execution.md:506-512`, `.issueops/operations/guides/issueops-execution.md:187`, `.issueops/AGENT_WORKFLOW.md:165`)에 `--body-file`을 넣는다. `issueops-cleanup` 스킬에 진행 결과 원고 작성(작성 지침의 진행 결과 절, PR 본문과 record 참고)을 넣는다.
  **Must NOT do**: 표지 형식과 `MergeManagedSection`을 바꾸지 않는다. `issueops-complete`에 반영 단계를 넣지 않는다.
  **Recommended Agent**: deep. 이유: port, 두 provider, CLI, 문서가 함께 바뀐다.
  **Parallelization**: Can Parallel: YES(T10~T13) | Wave 2 | Blocks: T14 | Blocked By: 묶음 1 커밋
  **References**: `internal/adapter/provider/issuebody/issue_body_section.go:66`, `internal/adapter/issueops/issueops_completion_remote.go:44-45,129`, `cmd/issueops/issueopscli/remotecmd/remote.go:237-262`, `internal/port/provider.go:190-212`
  **Acceptance Criteria**:
  - [ ] G13, G14, G25 met
  **QA Scenarios**:
  ```
  Scenario: 진행 결과 렌더
    Channel: bash
    Steps: ResultBody "결과 문장\n- 계획: ..."로 RenderCompletionSection
    Expected: 시작·끝 표지, "## 진행 결과", 원고 포함. 64자리 hex·"/Users/"·"plan 전문" 없음
  Scenario: body-file 누락
    Channel: bash
    Steps: reflect-completion --confirm (body-file 없음)
    Expected: 에러, provider 호출 0회
  Scenario: 하네스 값이 섞인 원고
    Channel: bash
    Steps: 40자리 커밋 SHA 한 줄과 /Users/x/wt 경로 한 줄이 든 원고로 reflect-completion --confirm (fake provider, 머지 검증 통과 fake)
    Expected: 에러, readability.critical에 commit_sha_full·local_path, provider 쓰기 0회
  ```
  **Commit**: YES | `feat(completion): reflect a human-written result after merge` | Files: port, issuebody, provider, remotecmd, 문서·스킬, tests

- [ ] T10. 정리 경로가 이슈 본문을 쓰지 않게 한다

  **What to do**: `cleanup finish`(`issueops_cleanup_finish.go:210-222`)와 `cleanup remote-branch`(`issueops_cleanup_remote_branch.go:95,110-117`)에서 `ReflectAudit` 원격 쓰기와 완료 구간 스냅샷을 제거하고, 감사 문자열을 각 응답의 `audit`에 넣는다. `ReflectCleanupAudit`(`issueops_cleanup_finish.go:507`)와 wiring을 제거한다. `audit_reflected`·`audit_error`를 읽는 스킬·테스트를 고친다. `--keep-remote-branch` 면제의 주석(61-62, 357-366행)을 새 근거(이슈의 연결 브랜치로 추적)로 바꾼다.
  **Must NOT do**: 정리 preview 게이트(`CompletionReflected` 표지 확인)를 바꾸지 않는다.
  **Recommended Agent**: deep. 이유: 두 정리 경로와 반영 상태(#128)를 확인해야 한다.
  **Parallelization**: Can Parallel: YES | Wave 2 | Blocked By: 묶음 1 커밋
  **References**: 위 위치, `internal/contract/issueops/cleanup_types.go`
  **Acceptance Criteria**:
  - [ ] G15 met, `issueops list`의 반영 상태가 바뀌지 않음을 테스트로 고정
  **QA Scenarios**:
  ```
  Scenario: 감사는 응답에만
    Channel: bash
    Steps: fake provider로 cleanup finish와 cleanup remote-branch 실행
    Expected: UpdateIssueBodySection 호출 0회, 응답 audit에 감사 문자열
  Scenario: 반영 상태 유지
    Channel: bash
    Steps: reflect-completion 뒤 cleanup remote-branch를 거쳐 list
    Expected: reflected 상태 유지
  ```
  **Commit**: YES | `fix(cleanup): keep cleanup audits out of the issue body` | Files: 정리 경로, 스킬, tests

- [ ] T11. 계획 검토 구간을 한 줄 흐름으로 바꾼다

  **What to do**: `IssueProviderUpdateIssueBodySectionRequest`에 판정과 라운드(순서, 판정, 지적 수)를 더한다. `RenderDevilsAdvocateSection`은 `## 계획 검토`, 흐름 한 줄("계획 검토: 1차 수정 요청(지적 3건) → 계획 수정 → 2차 통과"), 중단 판정이면 지적 목록을 렌더한다. 판정 값은 통과·수정 요청·중단으로 쓴다. 지적 목록을 렌더할 때 64·40자리 hex는 `[해시 생략]`, 로컬 절대 경로는 `[로컬 경로 생략]`으로 가린다. 렌더만 바꾸고 critical 거부는 넣지 않는다(Gap Analysis 6·13).
  **Must NOT do**: `regress`의 `IssueReflectedAt` 전제를 바꾸지 않는다.
  **Recommended Agent**: deep. 이유: port, 두 provider, reflect 경로가 바뀐다.
  **Parallelization**: Can Parallel: YES | Wave 2 | Blocked By: 묶음 1 커밋
  **References**: `internal/adapter/provider/issuebody/issue_body_section.go:42`, `internal/adapter/issueops/issueops_devilsadvocate_reflect.go`, `internal/adapter/issueops/issueops_regress.go:76-77`
  **Acceptance Criteria**:
  - [ ] G16, G26 met
  **QA Scenarios**:
  ```
  Scenario: 두 라운드 요약
    Channel: bash
    Steps: 수정 요청(3건) → 통과 이력으로 렌더
    Expected: "1차 수정 요청(지적 3건)"과 "2차 통과"가 한 줄, 지적 원문 없음
  Scenario: 짧은 한국어 중단
    Channel: bash
    Steps: 1차 중단(지적 1건 "범위가 이슈와 다르다")을 반영한 뒤 regress
    Expected: 반영 성공, regress 성공
  Scenario: 지적 속 해시·경로
    Channel: bash
    Steps: 지적 "plan digest 64자리 hex, /Users/x/plan.md를 확인"으로 중단 판정 렌더
    Expected: 렌더 결과에 64자리 hex와 "/Users/" 없음, "[해시 생략]"·"[로컬 경로 생략]" 포함
  ```
  **Commit**: YES | `feat(review): summarize plan review rounds for readers` | Files: port, issuebody, reflect, tests

- [ ] T12. 구현 진입·종료 전이가 구현 자료의 추적 사본을 쓴다

  **What to do**:
  1. 새 파일 `internal/adapter/issueops/issueops_materials.go`에 사본 작성 함수를 둔다. 입력은 record, 출력은 쓴 파일 목록과 warning이다. 워크트리 루트는 `record.Execution.Workspace.Root`, 이슈 폴더는 `issueArtifactDirFor`와 같은 번호 규칙으로 정한다.
  2. 쓰는 파일: `plan.md`(record `PlanPath`가 가리키는 계획, 없으면 봉인 plan), `intent.md`(봉인 intent가 있으면 그 내용, 없으면 `issueopsintent.Render`), 봉인 spec이 있으면 `spec.md`, 반론 검토 기록이 있으면 `plan-review.md`(판정, 라운드, 지적. 지적의 hex·경로는 T11과 같은 규칙으로 가림). 같은 내용이면 쓰지 않는다. 이슈 번호를 알 수 없으면 쓰지 않고 warning을 낸다.
  3. `advanceIssueOpsPhaseLocked`(`internal/adapter/issueops/issueops_phase.go:73`)가 `implement`와 `ai-slop-clean`으로 전이에 성공한 직후 이 함수를 호출한다. 사본 작성이 실패해도 전이는 되돌리지 않고 응답 warning으로 알린다.
  4. `cleanup finish` preview(T10 경로)와 `reflect-completion`(T9)은 봉인 plan이 있는데 `.issueops/issues/<n>/plan.md`가 없으면 warning `tracked_materials_missing`을 낸다.
  5. `.issueops/CONVENTIONS.md` 레이아웃 표에 `intent.md`·`plan-review.md` 행을 추가하고, `issueops-implement` 스킬에 "구현 진입 뒤 생긴 사본을 첫 커밋에 포함한다"를 적는다.
  **Must NOT do**: 봉인 디렉터리, 무시 규칙, link-plan, 전이 검증 규칙을 바꾸지 않는다.
  **Recommended Agent**: deep. 이유: phase 전이 경로와 멱등성, 두 실행 모드, 문서 규정을 다룬다.
  **Parallelization**: Can Parallel: YES | Wave 2 | Blocked By: 묶음 1 커밋
  **References**: `internal/adapter/issueops/issueops_phase.go:24-101`, `internal/adapter/issueops/issueops_artifact_stage.go:40-52,213`, `internal/adapter/issueops/execution_prepare_bridge.go:136`, `internal/domain/issueopsintent`, `.issueops/CONVENTIONS.md:100-127`
  **Acceptance Criteria**:
  - [ ] G17, G27, G28 met
  **QA Scenarios**:
  ```
  Scenario: direct 모드 사본
    Channel: bash
    Steps: prepare가 PlanPath를 봉인 경로로 채운 direct fixture record(이슈 513, intent·반론 검토 기록 포함)를 implement로 전이
    Expected: .issueops/issues/513/plan.md, intent.md, plan-review.md 생성. ai-slop-clean 전이 뒤 내용 불변
  Scenario: Orca 모드 사본
    Channel: bash
    Steps: execution_prepare_bridge 경로로 PlanPath가 채워진 Orca fixture record를 implement로 전이
    Expected: 같은 세 파일 생성
  Scenario: 사본 누락 경고
    Channel: bash
    Steps: 봉인 plan만 있고 추적 plan.md가 없는 record로 cleanup finish preview
    Expected: warnings에 tracked_materials_missing
  ```
  **Commit**: YES | `feat(issueops): keep tracked copies of implementation materials` | Files: issueops_materials.go, issueops_phase.go, CONVENTIONS, 스킬, golden, tests

- [ ] T13. sync-graph 댓글을 한국어로 바꾼다

  **What to do**: `internal/adapter/issueops/issueops_remote_sync.go:35`의 댓글을 "## 관련 이슈"와 한국어 링크 종류 이름으로 바꾸고 사이클 ID 줄을 지운다.
  **Recommended Agent**: quick. 이유: 문자열과 테스트다.
  **Parallelization**: Can Parallel: YES | Wave 2 | Blocked By: 묶음 1 커밋
  **References**: `internal/adapter/issueops/issueops_remote_sync.go:20-55`
  **Acceptance Criteria**:
  - [ ] 댓글 본문 테스트에 "Related Issue Graph"와 "Cycle:"이 없다.
  **QA Scenarios**:
  ```
  Scenario: 한국어 댓글
    Channel: bash
    Steps: 링크 2개 record로 댓글 본문 생성
    Expected: "## 관련 이슈" 포함, "Cycle:" 없음
  Scenario: 링크 없음
    Channel: bash
    Steps: 링크 0개
    Expected: synced=false, 댓글 없음
  ```
  **Commit**: YES | `fix(remote): write the issue graph comment in Korean` | Files: sync 경로, tests

- [ ] T14. ADR과 문서를 정정한다

  **What to do**: 새 ADR(`.issueops/adr/2026-09-24-issue-and-pr-bodies-are-human-documents.md`)을 쓴다: 본문과 구현 자료의 분리, 추적 사본(2026-09-09 ADR의 tracked intent 기각을 부분 대체), 진행 결과 구간과 post-merge 반영, 정리 감사의 응답 전용화와 남긴 브랜치 추적 근거, provider DTO 해석. `.issueops/ADR.md` 색인에 행을 추가한다. `.issueops/architecture/issueops.md` 42행과 178행의 completion 설명을 진행 결과 구간으로 고친다. `issueops-review`·`issueops-plan` 스킬에 `contract_change` 기록 규칙과 "지적은 독자가 읽는 한국어 문장으로 쓴다"를 추가한다. response golden을 재생성한다.
  **Must NOT do**: 다른 ADR을 고치지 않는다.
  **Recommended Agent**: deep. 이유: 결정 기록과 여러 문서의 일관성을 맞춘다.
  **Parallelization**: Can Parallel: NO | Wave 2 | Blocked By: T9~T13
  **References**: `.issueops/adr/2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact.md`, `.issueops/ADR.md:38-64`, `skills/issueops-review/SKILL.md`, `skills/issueops-plan/SKILL.md`
  **Acceptance Criteria**:
  - [ ] G6, G20, G22 met
  **QA Scenarios**:
  ```
  Scenario: ADR 색인
    Channel: bash
    Steps: rg -n "2026-09-24" .issueops/ADR.md
    Expected: 1건
  Scenario: 옛 계약 문장 제거
    Channel: bash
    Steps: rg -n "artifact 본문" .issueops/architecture/issueops.md
    Expected: 0건
  ```
  **Commit**: YES | `docs(adr): record that issue and PR bodies are human documents` | Files: ADR, architecture, 스킬, golden

- [ ] P2. 통합 검증과 단계 전이

  **What to do**: 구현 종료 전이(`phase --to ai-slop-clean`)는 워크트리에서 빌드한 바이너리(`$WORKTREE/bin/issueops`)로 실행해 #513의 추적 사본을 만든다(이 사이클의 구현 진입 전이는 설치된 옛 바이너리로 실행돼 사본이 없다). 사본은 정리 단계 커밋에 포함한다(G18). G24를 한 번 먼저 실행해 환경 문제 여부를 본 뒤, `issueops gates check --write --timeout-seconds 900`으로 G1~G28을 한 run에 채운다. 이어서 라우터 순서대로 AI slop 정리, 프로젝트 문서 반영, 검증 단계를 진행한다.
  **Recommended Agent**: deep. 이유: 전체 게이트와 단계 전이다.
  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: P3 | Blocked By: T14
  **References**: `skills/issueops/SKILL.md` 단계 표, `.issueops/TESTING.md`, `skills/issueops-clean/SKILL.md`, `skills/issueops-docs/SKILL.md`, `skills/issueops-verify/SKILL.md`
  **Acceptance Criteria**:
  - [ ] 원장 G1~G28 met(단일 run)
  **QA Scenarios**:
  ```
  Scenario: 전체 게이트
    Channel: bash
    Steps: issueops gates check --write --timeout-seconds 900
    Expected: 28/28 met
  Scenario: 시간 초과 감지
    Channel: bash
    Steps: 기본 제한으로 G2를 실행하면 초과할 수 있으므로 항상 --timeout-seconds 900을 붙인다
    Expected: G2·G3·G24가 timeout 없이 판정
  ```
  **Commit**: 사본과 원장 커밋 YES

- [ ] P3. 새 계약으로 게시하고 완료를 기록한다

  **What to do**: 워크트리 바이너리(`$WORKTREE/bin/issueops`)로 순서대로 진행한다. (1) `sync-issue`로 #513 본문을 새 이슈 계약으로 바꾸고 구현 범위를 PR 하나·묶음별 커밋으로 고친다. 2026-09-24에 기록한 `contract_change`를 해소하도록 이어서 `feedback mark-issue-updated`를 기록한다(Gap Analysis 12). (2) `remote create-pr`로 새 PR 계약(요약, 변경 내용, 확인한 것, 리뷰 포인트) 본문을 게시한다. (3) `remote verify-artifact`로 readback한다. (4) 검증 보고서를 커밋·푸시한다. (5) `execution complete`를 기록한다. 게시 본문마다 critical 0건과 warning 처리 내용을 보고한다.
  **Must NOT do**: PR을 머지하거나 `reflect-completion`, `close-issue`, cleanup을 실행하지 않는다.
  **Recommended Agent**: deep. 이유: 원격 쓰기 절차와 readback이 필요하다.
  **Parallelization**: Can Parallel: NO | Wave 3 | Blocked By: P2
  **References**: `skills/issueops-sync-issue/SKILL.md`, `skills/issueops-create-pr/SKILL.md`, `skills/issueops-complete/SKILL.md`, `skills/issueops-remote-write/SKILL.md`
  **Acceptance Criteria**:
  - [ ] PR readback 본문의 첫 절이 `## 요약`이고 64자리 hex 0건
  - [ ] #513 readback 본문의 첫 절이 `## 요약`
  **QA Scenarios**:
  ```
  Scenario: PR 게시
    Channel: bash
    Steps: remote create-pr preview → confirm → verify-artifact
    Expected: readability.critical 0, verify-artifact ok
  Scenario: 모호한 결과
    Channel: bash
    Steps: create-pr 응답이 timeout이면 재실행하지 않고 execution reconcile --preview
    Expected: 후보 정확히 1건 채택 또는 중지
  ```
  **Commit**: 검증 보고서 YES

## Final Verification Wave

- [ ] F1. 계획 준수 감사: 모든 TODO가 계획대로 실행됐는가
- [ ] F2. 코드 품질 검토: AI slop, 죽은 코드, 과도한 추상화가 없는가
- [ ] F3. 실제 QA: 각 시나리오가 증거와 함께 통과했는가
- [ ] F4. 범위 확인: 범위 확장이나 누락된 산출물이 없는가

## Commit Strategy

- 각 TODO의 Commit 항목대로 의도 하나당 커밋 하나를 만든다. 메시지는 `.issueops/COMMIT_POLICY.md`의 Conventional + Lore 형식이다.
- 묶음 1 커밋(T1~T8)을 모두 만든 뒤 묶음 2(T9~T14)를 시작한다. 리뷰와 되돌리기는 이 커밋 경계로 한다.
- `.issueops/*.md`를 고친 커밋에는 golden 재생성을 포함한다.

## Success Criteria

- 게이트 G1~G28이 한 run에서 모두 met이다.
- 새로 게시한 PR과 #513 본문이 새 계약을 따르고 critical 0건이다.
- 후속 작업: `DryRunPreview`의 UTF-8 잘림(Gap Analysis 11), 새 형식 본문 10건의 전후 측정(설계 검증 절), 독자 검토의 게시 조건화 여부 결정.
