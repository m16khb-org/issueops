# Research: IssueOps 사이클 스킬 규칙 목록

## TL;DR

- 사이클 스킬 11개의 `SKILL.md` 본문에서 규칙 150개를 뽑았다. 같은 뜻의 문장은 한 행으로 합쳤다.
- 강제 방식: `code` 41, `gate` 25, `validator` 0, `sentence` 84. 절반 이상이 문장으로만 있다.
- `sentence`가 많은 유형은 `범위 이탈`(25)과 `검증 누락`(20)이다. 편집 위치, 문서 열람, RED 테스트 선행, 리뷰어 격리, raw provider·git 우회 금지는 코드가 확인하지 않는다.
- `validator`가 0인 이유: `scripts/validate-skill.py`는 frontmatter와 본문 크기만, `scripts/verify-skill-shell.py`는 예시 셸 블록의 문법·파괴적 명령 표기만 검사한다. 에이전트 행동 규칙은 검사하지 않는다.
- 승격은 하지 않았다. 승격 여부는 사용자가 정한다(`.issueops/AGENT_WORKFLOW.md`의 Finish 절).

## Method

- 기준: 커밋 `7a0fb342b`, 2026-11-11. 줄 번호는 이 커밋 기준이라 코드가 바뀌면 어긋난다.
- 대상: `skills/issueops`, `issueops-create-issue`, `issueops-prepare`, `issueops-plan`, `issueops-implement`, `issueops-slop-clean`, `issueops-docs`, `issueops-verify`, `issueops-create-pr`, `issueops-complete`, `issueops-cleanup`의 `SKILL.md` 본문. frontmatter와 `references/`는 뺐다. 읽기 전용 에이전트 셋이 스킬을 나눠 읽고 행을 냈다. 합칠 때 스킬 사이의 같은 규칙을 한 행으로 묶고 분류가 엇갈린 행을 다시 판정했다.
- 강제 방식의 정의:
  - `code`: Go 코드나 스킬 스크립트가 위반을 거부한다. 근거는 실제로 있는 Go 테스트 이름(`TestXxx`)이나 거부 코드의 `파일:라인`이다.
  - `gate`: `issueops next`나 readiness의 missing 키가 다음 단계를 막는다. 키 문자열은 규칙 요지 칸에, 정의 위치는 근거 칸에 적었다.
  - `validator`: `scripts/validate-skill.py`나 `scripts/verify-skill-shell.py`가 스킬 문서를 검사한다.
  - `sentence`: 위 근거를 찾지 못했다. 근거 칸은 `-`다.
- SKILL.md 문구를 고정하는 테스트는 강제로 세지 않았다. 일부만 막는 코드가 있으면 막는 부분을 요지에 적고, 막지 못하는 부분이 규칙의 핵심이면 `sentence`로 두었다.
- 검증: 모든 `code`·`gate` 행의 근거가 있는지 고정 스크립트로 전수 확인했다(테스트는 `func TestXxx(`, 경로는 파일과 줄 수). gate 행은 키 문자열이 인용한 줄 근처에 있는지도 확인했다.
- `sentence` 판정의 부정 결과는 대부분 grep으로 확인했다(observed). 에이전트의 질문·보고 습관처럼 코드로 막을 대상이 아닌 규칙 약 20개는 검색 없이 판정했다(inferred).

## Findings

### 증거 없는 완료

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 승인된 종료점 전에는 완료로 끊지 않는다 | skills/issueops-implement/SKILL.md:8-9,196 | sentence | - |
| 실행하지 않은 검증을 통과로 적지 않고, 정리·문서 변경 뒤 다시 실행한다. 옛 검증 기록을 새 fingerprint 결과로 옮겨 적지 않는다 | skills/issueops-implement/SKILL.md:116,185-187, skills/issueops-slop-clean/SKILL.md:97-99,122-131,176-177, skills/issueops-docs/SKILL.md:124-126, skills/issueops-verify/SKILL.md:76-79,95-97,105-106, skills/issueops-complete/SKILL.md:103-104,184-185,217 | sentence | - |
| report에 `## 적용한 문서 규칙`, `## 문서 → 구현 대조`, 명령별 검증 행, Not Run 사유를 적는다 | skills/issueops-implement/SKILL.md:106-107, skills/issueops-slop-clean/SKILL.md:139-141, skills/issueops-docs/SKILL.md:64-68, skills/issueops-verify/SKILL.md:48-49,107-110 | sentence | - |
| 검증 보고서는 canonical worktree 안의 정규 파일 `.issueops/issues/<n>/verified-execution-report.md`다 | skills/issueops-implement/SKILL.md:188-189, skills/issueops-complete/SKILL.md:50,83-86,212 | code | internal/adapter/outbound/issueopscompletion/environment.go:77-86, internal/adapter/issueops/implementation/evidence.go:162-166, TestEnvironmentVerifyReport |
| 보고서의 초안 표시를 지우고 PR URL·최종 검증을 넣은 뒤 커밋한다 | skills/issueops-complete/SKILL.md:83-91,96-100 | sentence | - |
| 완료 보고 다섯 블록의 값은 출처 명령에서만 읽고 확인하지 못한 줄은 `미확인`으로 둔다 | skills/issueops-complete/SKILL.md:132-136,170-188,218-220 | sentence | - |
| `--final-head`는 보고서 커밋 뒤 읽은 전체 SHA이고 canonical HEAD와 같다 | skills/issueops-complete/SKILL.md:49,71-73,186-187,211 | code | internal/domain/issueopscompletion/input.go:52-55, TestCompletionHeadRequiresFullMatchingHash |
| complete는 `--confirm`과 검증 항목 1개 이상을 요구한다 | skills/issueops-complete/SKILL.md:52,60,103,214 | code | internal/domain/issueopscompletion/input.go:17, internal/domain/issueopscompletion/input.go:88-93, TestCompletionEvidenceRejectsUnconfirmedOrEmptyEvidence |
| 봉인된 artifact 없이 complete하지 않고 URL·target branch·kind를 봉인값과 맞춘다 | skills/issueops-create-pr/SKILL.md:162-171,176-177, skills/issueops-complete/SKILL.md:43-45,213 | code | internal/domain/issueopscompletion/artifact.go:40-57, internal/domain/issueopsremote/artifact.go:106, TestValidateArtifactRejectsNonCanonicalOrMismatchedEvidence, TestCompleteValidatesArtifactBeforeEnvironmentEffects |
| 원격 생성 직후 provider readback으로 URL·상태·draft·label·assignee·본문·head를 대조한다 | skills/issueops-create-issue/SKILL.md:202-205, skills/issueops-create-pr/SKILL.md:12-13,121-123,160-163,186-188 | code | internal/application/issueopsremote/issue_create.go:189-197, internal/adapter/provider/github/provider.go:361-373, internal/adapter/provider/gitlab/provider.go:304, TestRunRemoteCreateIssueConfirmFailsWhenLiveVerificationFails, TestGitLabCreatePullRequestRejectsEveryReadbackDrift |
| close-issue는 ok·closed·다시 읽은 closed 상태일 때만 성공이다 | skills/issueops-cleanup/SKILL.md:189-196,278 | code | internal/adapter/provider/github/provider.go:584, internal/adapter/provider/gitlab/provider_issue.go:98, TestRemoteCompletionCloseWithoutClosureLeavesRecordAlone |
| cleanup apply 뒤 결과 필드와 worktree·branch·이슈를 다시 확인하고 부분 실패를 완료로 보고하지 않는다 | skills/issueops-cleanup/SKILL.md:263-285 | sentence | - |
| done은 `execution complete`만 기록한다(pr phase, 검증된 artifact, 반납된 lease) | skills/issueops/SKILL.md:101-103 | code | internal/application/issueopscycle/phase_entry.go:60-77, TestValidatePhaseEntryDoneRequiresPRPhaseThenArtifactThenReleasedCompletion |
| 테스트 성공이 원격 갱신·reply·merge readiness·cleanup 완료를 대신하지 않는다 | skills/issueops/SKILL.md:136-137,195-196 | sentence | - |
| 원문은 `--raw-request`에 그대로 넣고, plan-prep은 waive 대신 근거를 적고, 걸린 ADR·caution을 열어 판정한다 | skills/issueops-create-issue/SKILL.md:40-55,185 | sentence | - |
| 문제·성공 기준·비목표·열린 결정이 확인되기 전에는 이슈를 만들지 않는다 | skills/issueops-create-issue/SKILL.md:72-75 | sentence | - |
| intent·domain-review·split·plan-prep·issue 기록을 채워야 grill·plan에 들어간다(키 `raw_request`, `domain_review`, `split_decision`, `plan_prep_decisions`, `issue_url`) | skills/issueops-create-issue/SKILL.md:82-84,95-114 | gate | internal/domain/issueops/intent_readiness.go:14-24, internal/domain/issueops/readiness_evidence.go:21-26, internal/domain/issueops/plan_prep_readiness.go:20-24, internal/domain/issueops/early_phase_readiness.go:15, TestIssueOpsGrillReadinessRequiresArtifacts |
| base는 정확한 SHA로 봉인하고 링크는 읽어 본 뒤에만 verified로 적는다(키 `branch_prepare`, `branch_link_verified`. 플래그 값은 자가 신고) | skills/issueops-prepare/SKILL.md:28-29,99-104,160-161 | gate | internal/domain/issueopsnext/classify.go:389-391, TestClassifyUnverifiedBranchLinkStaysInPrepare |
| 계획의 기존 동작 단언은 `파일:라인`으로 인용하고 미확인은 가정으로 적는다 | skills/issueops-plan/SKILL.md:98-100 | sentence | - |
| 계획 검토가 이슈 본문 사실이 틀렸다고 보면 `contract_change` feedback으로 기록한다 | skills/issueops-plan/SKILL.md:145 | sentence | - |
| child verdict는 accept·reject·drop이고 accept는 evidence, 나머지는 10자 이상 reason이 필요하다 | skills/issueops-implement/SKILL.md:175-177 | code | internal/domain/issueops/child_verdict.go:17-25, internal/domain/issueops/child_verdict.go:50-51, TestChildVerdictRefusesInvalidEvidenceAndNonTerminalAcceptance |
| child가 모두 accepted·dropped가 되기 전에는 구현을 끝내지 않는다(키 `child_incomplete:<id>`, `child_unvalidated:<id>`) | skills/issueops-implement/SKILL.md:185-187 | gate | internal/domain/issueops/child_gate.go:41-47, internal/domain/issueopsnext/classify.go:470-474, TestChildPRGateMissingPreservesValidationMatrix |
| 봉인 뒤 파일이 바뀌면 재정리·재검증·재봉인한다(키 `ai_slop_clean_stale`) | skills/issueops-slop-clean/SKILL.md:26-28,156-164, skills/issueops-docs/SKILL.md:30-32,112-126,164 | gate | internal/domain/issueops/ai_slop_fingerprint_readiness.go:24-26, internal/domain/issueopsnext/classify.go:100, TestAISlopCleanFingerprintReadinessMissingPreservesObservedCases |
| 봉인 기록이 있어야 clean을 통과한다(키 `ai_slop_clean_at`, `ai_slop_clean_head`, `ai_slop_clean_fingerprint`, `cleanup_evidence`, `verification_evidence`) | skills/issueops-slop-clean/SKILL.md:185-188 | gate | internal/domain/issueops/ai_slop_completion.go:15-27, TestAISlopCleanCompletionMissingPreservesEvidenceKeys |
| `--category`에는 실제로 제거한 이름만 쓴다 | skills/issueops-slop-clean/SKILL.md:150-154 | sentence | - |
| 근거 없는 "모두·보장·검증됨" 주장은 낮추거나 검증을 더한다 | skills/issueops-slop-clean/SKILL.md:73,81-83 | sentence | - |
| no-change 판정은 존재하는 project doc을 `--reviewed-doc`으로 하나 이상 적는다 | skills/issueops-docs/SKILL.md:136-150,167-168 | code | internal/domain/issueopsreview/record_validation.go:43-45, internal/domain/issueopsreview/project_docs_paths.go:28-46, TestRecordProjectDocsReviewNoChangeWithoutReviewedDocsFollowsTheWorktree, TestProjectDocumentPathPolicy |
| updated 판정의 `--doc`은 실제 변경 집합 안에 있어야 한다 | skills/issueops-docs/SKILL.md:142-143,162-163 | code | internal/domain/issueopsreview/project_docs_paths.go:11-17, internal/application/issueopsreview/project_docs_review.go:103-120, TestProjectDocsUseCaseRejectsUnchangedDocument |
| provider가 읽어 확인한 merge 증거가 있을 때만 cleanup한다(키 `remote_artifact_merged`) | skills/issueops/SKILL.md:124,187, skills/issueops-cleanup/SKILL.md:40,43-44,62-66,108-109 | gate | internal/domain/issueops/cleanup_finish.go:89-91, internal/application/issueopscleanup/finish_evidence.go:27-48, TestCleanupFinishPreviewRequiresIndependentEvidence, TestFinishEvidenceRefusesUnknownMergeBeforeIssueReadback |

### 범위 이탈

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 단계 스킬은 `next --id`의 `stage.key`로 자기 단계를 확인한다. 순서를 건너뛴 phase 전이는 readiness가 거부한다(키 예 `plan_path`) | skills/issueops/SKILL.md:71-74, skills/issueops-create-issue/SKILL.md:8-9,23-31, skills/issueops-prepare/SKILL.md:8-10,16-24, skills/issueops-plan/SKILL.md:8-9,18-27, skills/issueops-implement/SKILL.md:22-36, skills/issueops-slop-clean/SKILL.md:18-24, skills/issueops-docs/SKILL.md:18-23, skills/issueops-verify/SKILL.md:19-24 | gate | internal/application/issueopscycle/phase_entry.go:48-55, internal/domain/issueopsnext/classify.go:428-500 |
| 지정된 ID는 `--id`로 유지하고 ambiguous여도 다른 사이클을 고르지 않는다 | skills/issueops/SKILL.md:25-27,34 | sentence | - |
| 요청 범위와 종료점을 넘지 않는다(초안만이면 초안에서, 계획까지면 prepare 없이) | skills/issueops/SKILL.md:30,41-43,49-50, skills/issueops-create-issue/SKILL.md:14,117-119, skills/issueops-plan/SKILL.md:149-152 | sentence | - |
| 인계 전 승인 범위를 status와 대조하고 보류·phase·claim을 승인으로 보지 않는다. 런처 설치·launch 재시도를 하지 않는다 | skills/issueops-implement/SKILL.md:47-52 | sentence | - |
| canonical worktree에서만 편집하고 source checkout을 고치지 않는다. 편집 전후 pwd·branch·HEAD·status를 실측한다 | skills/issueops/SKILL.md:86-88,141, skills/issueops-implement/SKILL.md:38-45,115,145-146, skills/issueops-slop-clean/SKILL.md:179,189-190 | sentence | - |
| worker 프롬프트에 ID·generation·branch·허용 경로·수락 기준·중단 규칙을 넣는다 | skills/issueops/SKILL.md:88-89 | sentence | - |
| commit·push는 승인된 종료점 안에서만 한다. implement·clean·docs·verify에서 publication·merge·cleanup을 하지 않는다 | skills/issueops/SKILL.md:149-150, skills/issueops-implement/SKILL.md:118-119,148,195-196, skills/issueops-slop-clean/SKILL.md:168,181, skills/issueops-docs/SKILL.md:156, skills/issueops-verify/SKILL.md:207 | sentence | - |
| base 앞섬은 `base_advanced` 경고 때만 `sync-base --preview`로 보고 스스로 apply하지 않는다 | skills/issueops-implement/SKILL.md:61-64, skills/issueops-verify/SKILL.md:165-177 | sentence | - |
| implement 진입은 link-plan, compatibility review, phase 전이 순서다(키 `plan_path`, `compatibility_review`, `compatibility_approval`) | skills/issueops-implement/SKILL.md:66-86 | gate | internal/application/issueopscycle/phase_entry.go:50-51, internal/application/issueopscycle/base_implementation_readiness.go:15, internal/application/issueopscycle/implementation_readiness.go:16-25, internal/domain/issueopsreview/compatibility_readiness.go:14-34 |
| compatibility review 전에 계획이 말한 코드를 열어 단언을 확인하고 충돌은 `--blocker`로 적는다 | skills/issueops-implement/SKILL.md:92 | sentence | - |
| 승인된 plan.md·artifact와 추적 사본을 직접 고치지 않는다 | skills/issueops-implement/SKILL.md:117,136 | sentence | - |
| 재사용을 먼저 보고 계획에 없는 추상화·계약 표면을 만들지 않는다. hot path는 전후를 잰다 | skills/issueops-implement/SKILL.md:125-127, skills/issueops-slop-clean/SKILL.md:133-136,191-192 | sentence | - |
| 요구사항이 바뀌면 `contract_change` feedback으로 이슈를 갱신한 뒤 반영한다(키 `contract_feedback_issue_update`) | skills/issueops-implement/SKILL.md:134-137 | gate | internal/application/issueopscycle/pr_readiness.go:53-55, internal/domain/issueopsreview/feedback_completion.go:34-36, TestFeedbackCompletionMissingMapsPersistedFeedback |
| child 위임은 parent가 implement phase이고 리뷰가 승인됐을 때만 시작한다 | skills/issueops-implement/SKILL.md:171-173 | code | internal/domain/issueops/delegation.go:15-35, TestCovBranchChildStartPreconditions, TestMissingPreconditionsAcceptsReviewedParent |
| 계획이 sub-agent pattern·tradeoff를 기록한 경우에만 위임하고 parent가 child worktree를 만들지 않는다 | skills/issueops-implement/SKILL.md:172-173,177 | sentence | - |
| docs·verify 단계는 코드를 고치지 않는다. 고치면 stale 키로 앞 단계로 돌아간다(키 `implementation_review_stale`, `project_docs_review_stale`, `schema_evidence_stale`) | skills/issueops-docs/SKILL.md:8,169, skills/issueops-verify/SKILL.md:9,26-36,55-56,85-87,110,205-209 | gate | internal/domain/issueopsreview/gates.go:25-60, internal/domain/issueopsnext/classify.go:100-108, internal/domain/issueopsremote/issueops_remote_project.go:225-232 |
| CLEANUP_BOUNDARY 밖은 고치지 않고 Out-of-scope findings에만 적는다 | skills/issueops-slop-clean/SKILL.md:8-9,44-48,175 | sentence | - |
| 정리는 한 pass씩, 증명된 미사용만 지우고 실패하면 되돌린다 | skills/issueops-slop-clean/SKILL.md:65,75-80,84-89,180 | sentence | - |
| 문서와 구현이 어긋나면 구현을 고치고 규칙을 고쳐 덮지 않는다 | skills/issueops-docs/SKILL.md:64-72,165 | sentence | - |
| ADR는 append-only다. 뒤집는 결정은 새 항목으로 적는다 | skills/issueops-docs/SKILL.md:100-101,166 | sentence | - |
| 구현 결과에 맞춰 운영 문서를 갱신하고 같은 실수는 Recurring Cautions 절을 따른다 | skills/issueops-docs/SKILL.md:34-38,40-53,74-88 | sentence | - |
| 기본은 분할하지 않고 조건이 맞을 때만 child로 나눈다. 한 이슈에 사이클 둘을 걸지 않는다 | skills/issueops-create-issue/SKILL.md:147-166 | sentence | - |
| worktree는 `execution prepare`만 만든다. prepare·plan에서 `git worktree add`·link-worktree를 하지 않는다 | skills/issueops-prepare/SKILL.md:8-10,30-32,138-139, skills/issueops-plan/SKILL.md:35,171,179 | sentence | - |
| prepare는 기존 브랜치 재사용·삭제, source checkout의 stash·commit, force, `gh issue develop`를 하지 않는다 | skills/issueops-prepare/SKILL.md:36-41,135,156-157 | sentence | - |
| `start`는 source checkout 기준이다. linked worktree 경로는 source root로 정규화된다 | skills/issueops-create-issue/SKILL.md:33,35-38 | code | internal/adapter/issueops/cycle_record_store.go:76, internal/domain/issueops/start.go:12, TestCycleIdentitiesKeepTheirGitObserver |
| 아키텍처·계획은 source checkout 밖 임시 파일에 쓰고 `artifact stage`로 올린다 | skills/issueops-plan/SKILL.md:31-35,92,187 | sentence | - |
| 아키텍처 승인 전에는 계획을 쓰지 않고 스테이징 없이 인계하지 않는다(키 `architecture_artifact`, `architecture_review`, `plan_artifact`) | skills/issueops-plan/SKILL.md:24-27,89-92,179 | gate | internal/domain/issueopsnext/classify.go:283, internal/domain/issueopsnext/classify.go:308-310, TestClassifyPlanStagesArchitectureFirst |
| link-plan은 plan 단계에서 하지 않는다(worktree와 승인된 아키텍처가 필요) | skills/issueops-plan/SKILL.md:108-109 | code | internal/domain/issueops/workspace_link.go:35-46, internal/application/issueopsbranch/workspace_link.go:51, TestPlanLinkValidationsExplainEachRefusal |
| 의존성은 worktree 안에서 문서화된 명령으로 준비하고 생성물·심볼릭 링크를 커밋하지 않는다 | skills/issueops-plan/SKILL.md:173-175 | sentence | - |
| PR head·base는 현재 branch와 봉인 base이고 moving ref를 추측하지 않는다 | skills/issueops-create-pr/SKILL.md:71,81-82,115 | code | internal/domain/issueopsremote/create_preparation.go:97, internal/domain/issueopsremote/create_preparation.go:151-155, TestPrepareCreateRequestBindsBranchAndCanonicalMetadata |
| complete는 pr phase(`pr.complete`)에서만 한다. "구현 완료"는 phase 전이다 | skills/issueops-implement/SKILL.md:198-199, skills/issueops-complete/SKILL.md:12-14,42,210 | code | internal/domain/issueopscompletion/input.go:40-45, TestCompleteRefusalsPreserveEffectOrder |
| complete는 done과 lease 반납만 한다. merge·정리·코드 수정은 별도 승인이나 새 execution이다 | skills/issueops/SKILL.md:202-204, skills/issueops-complete/SKILL.md:8-10,197-204,216 | sentence | - |
| cleanup은 done 단계이고 lease가 반납된 사이클에서만 한다(키 `phase_done`, `lease_released`) | skills/issueops-cleanup/SKILL.md:22-23,40-41 | gate | internal/domain/issueops/cleanup_finish.go:83-88, TestCleanupFinishPreviewRequiresIndependentEvidence |
| 연결된 child task는 close 증거가 있어야 한다(키 `child_tasks_closed`) | skills/issueops-cleanup/SKILL.md:42,111-115 | gate | internal/domain/issueops/cleanup_finish.go:61-63, TestCleanupFinishPreviewRequiresIndependentEvidence |
| cleanup 대상 worktree의 HEAD가 로컬 branch OID와 같아야 한다 | skills/issueops-cleanup/SKILL.md:43,97-100,211 | sentence | - |
| 이슈 닫기·worktree·branch 삭제를 모두 적은 확인을 최신 사용자 메시지에서 받는다 | skills/issueops-cleanup/SKILL.md:8-12,117-129,131-158 | sentence | - |
| typed cleanup 밖의 raw git 삭제(worktree remove, branch -D, update-ref, push --delete)와 수동 프로세스 종료를 하지 않는다 | skills/issueops-cleanup/SKILL.md:48-49,168,251-259, skills/issueops-create-pr/SKILL.md:132-133 | sentence | - |

### 정체·중단

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 필수 입력·명령·provider를 확인할 수 없거나 게이트가 모호하면 쓰지 않고 보고한다. 대상 ID가 0개나 복수일 때만 묻는다 | skills/issueops/SKILL.md:12-15,177-180, skills/issueops-create-issue/SKILL.md:223-224, skills/issueops-implement/SKILL.md:13,26-27,203-204, skills/issueops-cleanup/SKILL.md:14-18,35-36,127-129 | sentence | - |
| 단계·세션·base 선택과 이미 허용된 write를 메뉴로 묻지 않는다 | skills/issueops/SKILL.md:29,41-43,198-201, skills/issueops-prepare/SKILL.md:33-35,43-62, skills/issueops-plan/SKILL.md:171 | sentence | - |
| 질문은 해소할 수 없는 blocking 사안만, 한 번에 하나 묻는다 | skills/issueops/SKILL.md:31-32,173-176,189-191, skills/issueops-create-issue/SKILL.md:59-71,76-80 | sentence | - |
| 같은 blocker가 두 번 복구 뒤에도 남으면 보고하고 멈춘다. polling과 모호한 원격 mutation을 반복하지 않는다 | skills/issueops/SKILL.md:190-191 | sentence | - |
| 같은 focused test가 두 번 GREEN에 실패하면 debugging으로 넘기고 세 번째는 멈춘다 | skills/issueops-implement/SKILL.md:132 | sentence | - |
| 브랜치 충돌·fetch 실패·부분 완료는 멈추고 원문을 보고한다 | skills/issueops-prepare/SKILL.md:149-161 | sentence | - |
| 계획 리뷰 기록 제약: pass는 finding 1개 이상, stop은 이슈 반영 뒤 regress, revise는 5회 상한 | skills/issueops-plan/SKILL.md:129-143 | code | internal/domain/issueopsreview/devils_advocate.go:49, internal/domain/issueopsreview/devils_advocate.go:95, internal/domain/issueopsreview/regress.go:22-37, TestApplyReviewEnforcesReviseRoundCapAndPreservesHistory, TestRegressionPreconditionsPreserveFailureOrder |
| pending이거나 이미 성공한 prepare·create-pr을 반복하지 않고 reconcile로 간다 | skills/issueops-implement/SKILL.md:151-163, skills/issueops-create-pr/SKILL.md:116,179-180, skills/issueops-complete/SKILL.md:127-128 | code | internal/application/issueopspreparation/prepare.go:141-144, internal/domain/issueopsremote/create_preparation.go:32-34, internal/domain/issueopsremote/create_preparation.go:49-54, TestPrepareExistingExecutionReportsRecoveryCommands, TestCreateAuthorityPreservesGateOrder |
| preview에 `issue_closed` 경고가 있으면 confirm하지 않고 묻는다 | skills/issueops-create-pr/SKILL.md:157-158 | sentence | - |
| 같은 증거로 다시 complete하면 성공이고 다른 증거는 거부된다 | skills/issueops-complete/SKILL.md:121-126 | code | internal/application/issueopscompletion/complete.go:97-103, TestCompleteReopenedGenerationKeepsExactlyOnceContract, TestCompleteIdenticalRetrySkipsEnvironment |
| complete가 거부되면 `--confirm` 없이 상태를 보고하고 `next_command`를 따른다. 완료 보고는 내지 않는다 | skills/issueops-complete/SKILL.md:29-30,60-61,126,189-190,222-225 | sentence | - |
| close dry-run이 ok이고 missing이 `completion_reflected`·`issue_closed`뿐일 때만 reflect·close를 쓴다 | skills/issueops-cleanup/SKILL.md:102-109 | sentence | - |

### 상태·권한 경쟁

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| durable mutation은 활성 lease holder가 canonical cwd에서만 한다 | skills/issueops/SKILL.md:37,80-84,182,202-203, skills/issueops-create-issue/SKILL.md:16-19, skills/issueops-implement/SKILL.md:139-148, skills/issueops-create-pr/SKILL.md:67-73,173, skills/issueops-complete/SKILL.md:46-47,118-119,215 | code | internal/domain/issueopsauthorization/authorization.go:15-30, internal/domain/issueopsauthorization/authorization.go:82-85, internal/domain/issueopscompletion/decision.go:73-77, TestAuthorizeRequiresExactLeaseHolderProcessAndWorkspace, TestExecutionMutationRequiresCurrentLeaseHolderInCanonicalWorktree, TestValidateActiveDeniesEachBrokenPrecondition |
| create-pr의 expected-generation은 현재 lease와 같아야 한다 | skills/issueops-create-pr/SKILL.md:81 | code | internal/domain/issueopsremote/create_preparation.go:38-44, TestValidateCreateAuthorityConfirmRequiresMatchingNonZeroGeneration |
| actor는 살아 있고 PID 재사용에 안전한 process receipt를 가져야 한다 | skills/issueops-complete/SKILL.md:48 | code | internal/domain/issueops/native_actor.go:36, internal/domain/issueops/native_actor.go:76, TestValidateNativeActorRequiresReuseSafeProcessReceipt, TestValidateObservedNativeProcessRequiresLiveMatchingIdentity |
| actor 플래그는 whoami·`next_command` 값을 그대로 쓰고 손으로 조립하지 않는다. claim은 한 번 실행한다 | skills/issueops/SKILL.md:75-78, skills/issueops-create-issue/SKILL.md:16-17, skills/issueops-implement/SKILL.md:29-31,141-143, skills/issueops-create-pr/SKILL.md:173-176, skills/issueops-complete/SKILL.md:118-119, skills/issueops-cleanup/SKILL.md:24-28 | sentence | - |
| "그 세션은 껐다"는 말은 quiescence 증거가 아니다 | skills/issueops/SKILL.md:83 | code | internal/domain/issueops/replacement_owner.go:63, internal/application/issueopsreplacement/inventory.go:39, TestValidateReplacementQuiescenceFinalizesOnlyDeadHolders |
| 인수는 replace preview가 준 generation·inventory·quiescence로 claim까지 이어 간다 | skills/issueops/SKILL.md:64-67,116, skills/issueops-plan/SKILL.md:167-170, skills/issueops-implement/SKILL.md:147,153-164 | code | internal/domain/issueopslease/claim.go:61-76, internal/application/issueopsreplacement/service.go:195-197, internal/application/issueopsreplacement/service.go:213-216, TestClaimDomainContract, TestReplacementRefusalsNeverPersist |
| `start` 응답이 불분명하면 같은 명령을 다시 실행하지 않고 `issueops list`로 확인한다 | skills/issueops-create-issue/SKILL.md:36-39 | sentence | - |
| 세션 인계는 환경별 순서를 따르고 인계 뒤 이 세션은 구현을 이어가지 않는다 | skills/issueops/SKILL.md:39-50,60-67, skills/issueops-plan/SKILL.md:147-171,179 | sentence | - |
| delegated child의 base는 부모 브랜치이고 `--parent-worktree`는 부모 worktree와 같다 | skills/issueops-prepare/SKILL.md:122-127 | code | internal/domain/issueops/branch_prepare.go:63, internal/domain/issueops/execution_workspace_layout.go:63, TestValidatePreparationUmbrellaAllowsMatchingOrUnbranchedUmbrella |
| 문서 수정은 project-docs 도구로 하고 revise는 현재 SHA를 맞춘다 | skills/issueops-docs/SKILL.md:92-98 | code | internal/domain/projectdoc/revision.go:57-63, TestPlanRevisionRequiresCurrentDigestAndPreservesDryRun |
| issue_url·branch prepare·execution이 없거나 phase가 pr이 아니면 create-pr을 하지 않는다 | skills/issueops-create-pr/SKILL.md:64-70,193-194 | code | internal/domain/issueops/remote_command_defaults.go:21, internal/domain/issueopsremote/create_preparation.go:36-40, internal/domain/issueopsremote/create_preparation.go:93, TestCreateAuthorityPreservesGateOrder |
| dirty worktree면 cleanup이 막힌다(키 `worktree_clean`) | skills/issueops-cleanup/SKILL.md:43-44,97 | gate | internal/domain/issueops/cleanup_finish.go:140-141, internal/application/issueopscleanup/finish_preview.go:91-92 |
| cleanup은 source repo의 다른 세션에서 하고 worktree 안이나 source checkout 자체에서는 하지 않는다(키 `cwd_outside_worktree`, `requester_occupies_worktree`, `worktree_is_source_checkout`) | skills/issueops-cleanup/SKILL.md:37-39,45-49 | gate | internal/domain/issueops/cleanup_finish.go:135-137, internal/domain/issueops/cleanup_workspace.go:114, internal/application/issueopscleanup/workspace.go:24, TestFinishPreviewFlagsRequesterCWDInsideWorktreeOnlyWhenCWDIsGiven |
| 대기 중인 external intent가 있으면 cleanup하지 않는다 | skills/issueops-cleanup/SKILL.md:43-44 | sentence | - |
| 원격 source branch가 없거나 남기기로 했을 때만 진행한다(키 `remote_branch_absent`) | skills/issueops-cleanup/SKILL.md:50-58 | gate | internal/domain/issueops/cleanup_finish.go:152-154, TestCleanupFinishPreviewRequiresIndependentEvidence |
| 준비된 base와 provider가 본 merged base가 같아야 한다(키 `base_branch_drifted`) | skills/issueops-cleanup/SKILL.md:59-60 | gate | internal/domain/issueops/cleanup_finish.go:110, internal/domain/issueops/cleanup_finish.go:170-175, TestClassifyMergedBaseRetarget |
| destructive cleanup은 preview의 fingerprint로만 apply하고 대상이 바뀌었거나 missing이 있으면 apply하지 않는다 | skills/issueops/SKILL.md:151-152, skills/issueops-cleanup/SKILL.md:102-109,198-211,225-249 | code | internal/domain/issueops/cleanup_finish_commands.go:45-54, internal/application/issueopscleanup/finish_executor.go:115-125, internal/application/issueopscleanup/abandon_executor.go:52-76, TestFinishExecutorRejectsDriftBeforeDestructiveEffects, TestAbandonExecutorApplyRequiresConfirmAndCurrentFingerprint |
| 이슈를 닫기 전에 completion을 반영한다(키 `completion_reflected`) | skills/issueops-cleanup/SKILL.md:162-165 | gate | internal/domain/issueops/cleanup_finish.go:92-94, TestCleanupFinishPreviewRequiresIndependentEvidence |

### 원격 쓰기 품질

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| `issueops remote`가 있는 쓰기를 raw `gh`·`glab`으로 우회하지 않는다. 직접 PR 생성은 사용자 허락 뒤에만 한다 | skills/issueops/SKILL.md:83,156-160, skills/issueops-create-issue/SKILL.md:204-205, skills/issueops-prepare/SKILL.md:93-97, skills/issueops-create-pr/SKILL.md:114,126-130 | sentence | - |
| preview를 읽고 같은 명령에 `--confirm`만 더한다. 본문이 바뀌면 새 preview, 모호하면 reconcile | skills/issueops-create-issue/SKILL.md:117-119,200-201,208, skills/issueops-plan/SKILL.md:156-158, skills/issueops-create-pr/SKILL.md:140-142,160-161, skills/issueops-cleanup/SKILL.md:166-169 | sentence | - |
| 이슈가 이미 연결됐거나 create 결과가 모호하면 새로 만들지 않고 reconcile한다(키 `issue_create_unsettled`) | skills/issueops-create-issue/SKILL.md:120-123,203-205 | code | internal/domain/issueops/issue_create_lifecycle.go:16-26, internal/domain/issueops/issue_reconcile.go:24-30, TestIssueCreateRefusesWhenIssueAlreadyLinked, TestIssueReconcileRequiresUniqueCompleteSearch |
| confirm 전 readability critical이 0이어야 한다(한국어 비율, 필수 절, 64자리 hex) | skills/issueops-create-issue/SKILL.md:171-180,195-196,200-201,216-218, skills/issueops-create-pr/SKILL.md:29-36,102,119-124,140-142,186-187 | code | internal/application/issueopsremote/template_body.go:87-88, internal/domain/artifactreadability/readability.go:151-171, internal/domain/artifactreadability/readability.go:190, TestTemplateResolveEmptyIssuePreviewReportsFindingsButConfirmRefuses, TestCheckSHA256HexIsCriticalWithLine, TestCheckKoreanRatioThresholds |
| 제목·본문에 secret과 `## Plan` 절을 넣지 않는다 | skills/issueops-create-issue/SKILL.md:182-185, skills/issueops-create-pr/SKILL.md:117,188 | code | internal/domain/policy/remote_create_inputs.go:13-39, internal/domain/issueopsremote/create_preparation.go:114-126, internal/domain/artifacttemplate/template.go:172, TestRunRemoteCreateIssueRejectsSecretLikeFields, TestValidateRejectsPlanLinkAndGitLabRelatedIssuesSection |
| 본문에 커밋 SHA 전문·로컬 경로·plan 원문·원장·파일 나열·라벨 점수·TBD를 넣지 않는다 | skills/issueops-create-issue/SKILL.md:182-185,195-196, skills/issueops-create-pr/SKILL.md:102-103,111-112 | sentence | - |
| confirm 전에 본문을 fluent-korean으로 다듬는다 | skills/issueops-create-issue/SKILL.md:182-185, skills/issueops-create-pr/SKILL.md:99-103,187 | sentence | - |
| PR 요약 마지막 줄에 `Closes #n`을 둔다 | skills/issueops-create-pr/SKILL.md:31-33,113 | sentence | - |
| review tier가 schema-auth·contract면 `위험과 되돌리기` 절을 둔다 | skills/issueops-create-pr/SKILL.md:38-42 | sentence | - |
| confirm 발행에는 label과 assignee가 하나 이상 필요하다 | skills/issueops-create-issue/SKILL.md:189-190,207-211, skills/issueops-create-pr/SKILL.md:67-70,186 | code | internal/domain/issueopsremote/create_metadata.go:46-54, TestValidateConfirmCreateMetadata, TestPrepareCreateRequestBindsBranchAndCanonicalMetadata |
| PR/MR assignee에 `@me` 같은 placeholder를 쓰지 않는다 | skills/issueops-create-pr/SKILL.md:67-70,186 | code | internal/domain/issueopsremote/create_preparation.go:105-110, TestPrepareCreateRequestBindsBranchAndCanonicalMetadata |
| 이슈 assignee는 인증된 실제 사용자이고 `@me`·이메일 추측을 쓰지 않는다 | skills/issueops-create-issue/SKILL.md:196-199 | sentence | - |
| 라벨 후보를 점수화하고 선택·거절은 본문이 아니라 decision으로 남긴다 | skills/issueops-create-issue/SKILL.md:103-107,189-194, skills/issueops-create-pr/SKILL.md:82-84 | sentence | - |
| PR 제목은 저장소의 커밋 제목 형식을 따른다 | skills/issueops-create-pr/SKILL.md:154-156 | sentence | - |
| completion 반영 본문은 전체 SHA·로컬 경로·2000자 초과 없이 쓴다 | skills/issueops-cleanup/SKILL.md:170-176,185 | code | internal/application/issueopsremote/remote_completion.go:86-91, internal/domain/artifactreadability/readability.go:193-196, TestRemoteCompletionReflectRefusesBeforeRemoteWrite, TestCheckLocalPathAndCommitSHAAreCriticalOnlyForCompletion |
| 반영 본문은 결과 1~2문장과 흐름 3~6줄로 쓰고 secret을 지운다 | skills/issueops-cleanup/SKILL.md:170-176 | sentence | - |
| reseed로 PR 본문의 사실이 바뀌면 complete 전에 sync-pr로 갱신한다 | skills/issueops-complete/SKILL.md:91-93 | sentence | - |

### 검증 누락

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 작업 전 관련 `.issueops` 문서를 실제로 열고 적용 여부를 기록한다 | skills/issueops/SKILL.md:142-143, skills/issueops-create-issue/SKILL.md:56-57, skills/issueops-prepare/SKILL.md:64-71,136-138, skills/issueops-plan/SKILL.md:37-53, skills/issueops-implement/SKILL.md:94-104, skills/issueops-slop-clean/SKILL.md:32-35, skills/issueops-docs/SKILL.md:55-59, skills/issueops-verify/SKILL.md:38-49, skills/issueops-create-pr/SKILL.md:87-90, skills/issueops-complete/SKILL.md:65-67,162-163,182, skills/issueops-cleanup/SKILL.md:68-75,139-144 | sentence | - |
| behavior change는 실패하는 테스트(RED)에서 시작한다 | skills/issueops/SKILL.md:139-140, skills/issueops-implement/SKILL.md:111-114 | sentence | - |
| API·DTO·OpenAPI 변경은 OPEN_API_SPEC gate와 api-doc check를 적용한다 | skills/issueops/SKILL.md:144, skills/issueops-implement/SKILL.md:120, skills/issueops-slop-clean/SKILL.md:94-96, skills/issueops-verify/SKILL.md:101-104 | sentence | - |
| pr 진입 전 정리·문서·스키마·feedback·push 증거가 최신이어야 한다(키 `ai_slop_clean_stale`, `project_docs_review`, `schema_evidence`, `upstream`, `target_branch_match`) | skills/issueops/SKILL.md:145-148,183-186, skills/issueops-create-pr/SKILL.md:64-65,74-79 | gate | internal/application/issueopscycle/phase_entry.go:54-55, internal/domain/issueops/ai_slop_fingerprint_readiness.go:25, internal/domain/issueopsreview/gates.go:34-60, internal/domain/issueops/pr_git_readiness.go:64-74, internal/domain/issueops/readiness_evidence.go:34, TestValidatePhaseEntryUsesPrefetchedPRReadiness |
| open question이나 stale 승인이 남으면 진행하지 않는다(키 `architecture_open_questions`, `architecture_review_stale`, `devils_advocate_review_stale`) | skills/issueops/SKILL.md:181, skills/issueops-plan/SKILL.md:67,140-141 | gate | internal/domain/issueopsreview/architecture_readiness.go:28, internal/domain/issueopsnext/classify.go:312, internal/domain/issueopsreview/devils_advocate_readiness.go:38, TestClassifyArchitectureReviewStaleStaysInArchitectureStage |
| 계획 리뷰 판정이 없거나 풀리지 않으면 인계하지 않는다(키 `devils_advocate_review`) | skills/issueops-plan/SKILL.md:129-132,140-143 | gate | internal/domain/issueopsreview/devils_advocate_readiness.go:34, TestClassifyPlanStagesArchitectureFirst |
| 아키텍처 문서 다섯 절과 승인 기록 요건(대안·위험·open question 0)을 갖춘다 | skills/issueops-plan/SKILL.md:60,79-85 | code | internal/domain/issueopsreview/architecture.go:37-69, internal/domain/issueops/plan_sections.go:51, TestRecordArchitectureReviewRejectsMissingSections, TestMissingArchitectureSectionsRejectsEmptyAndWhitespaceBodies |
| 계획에 재사용·호환성·성능·side effect 네 절을 둔다. link-plan이 제목을 검사한다 | skills/issueops/SKILL.md:91-92, skills/issueops-plan/SKILL.md:94, skills/issueops-implement/SKILL.md:72-74 | code | internal/domain/issueops/plan_sections.go:7-27, internal/application/issueopsbranch/workspace_link.go:70-74, TestMissingPlanSectionsReportsAbsentHeadings |
| 리뷰는 빈 컨텍스트 서브에이전트가 하고 inline은 사용자 지시 때만 쓴다. revise를 waive로 닫지 않는다 | skills/issueops-plan/SKILL.md:71-73,81-83,179, skills/issueops-verify/SKILL.md:133-138 | sentence | - |
| 원장 spec은 `G<n>: … CHECK … EXPECT` 형식이고 CHECK는 단일 argv다 | skills/issueops-plan/SKILL.md:113-117 | code | internal/domain/gates/mutation.go:137-171, TestRenderInitialLedgerRefusesIDlessAndEmptyCheckSpecs |
| 게이트 하나는 완료 조건을 직접 관측하고 `gates lint` problems가 남은 계획은 스테이징하지 않는다 | skills/issueops-plan/SKILL.md:96,119-125 | sentence | - |
| 수용 기준을 `gates init`으로 원장 파일로 만든다 | skills/issueops-implement/SKILL.md:81-83 | sentence | - |
| 원장은 `gates check --file`로 채우고 다른 사이클 원장을 읽지 않는다 | skills/issueops-implement/SKILL.md:121-123, skills/issueops-slop-clean/SKILL.md:125-131 | sentence | - |
| compatibility review에 blocker가 있으면 승인하지 않는다 | skills/issueops-implement/SKILL.md:88 | code | internal/domain/issueopsreview/compatibility.go:32-34, TestValidateCompatibilityReviewEvidenceAndBlockers |
| 인계문이 있으면 HEAD·계획 digest를 현재 worktree와 대조하고 취소 뒤 writer 종료를 확인한다 | skills/issueops-implement/SKILL.md:53-58,179-181 | sentence | - |
| 커밋 훅이 있으면 worktree에서 pre-commit을 지금 돌린다 | skills/issueops-implement/SKILL.md:190-192 | sentence | - |
| 정리 전에 회귀 테스트로 동작을 고정한다 | skills/issueops-slop-clean/SKILL.md:39,59-63 | sentence | - |
| 정리 전후를 code-quality-metrics로 재고 측정 없이 개선을 주장하지 않는다 | skills/issueops-slop-clean/SKILL.md:111-118,178 | sentence | - |
| ai-slop-clean 기록에는 category와 verification이 각각 하나 이상 있다 | skills/issueops-slop-clean/SKILL.md:150-154 | code | internal/domain/issueopsreview/ai_slop_clean.go:8-13, TestRecordAISlopCleanEvidenceRejectsBeforeSecondReadOrWrite |
| 하네스 저장소에서 `.issueops/*.md`를 고치면 response-contract golden을 다시 만든다 | skills/issueops-docs/SKILL.md:103-110 | sentence | - |
| 필수 검증을 같은 fingerprint로 병렬 시작하고 결과를 모은 뒤 판정한다. 배터리가 실패하면 리뷰 pass를 기록하지 않는다 | skills/issueops-verify/SKILL.md:51-60,203-204 | sentence | - |
| intent의 성공 기준을 원장과 대조해 덮지 못한 기준을 `## 의도 대조`에 적는다 | skills/issueops-verify/SKILL.md:64-68,178-179 | sentence | - |
| 원장을 파서 state로 읽고 미충족이 남으면 멈춘다(키 `gates_incomplete:<path>`) | skills/issueops-verify/SKILL.md:70-74,110 | gate | internal/domain/issueops/gate_ledger_readiness.go:21-31, internal/domain/gates/evaluate.go:101-111, internal/application/issueopscycle/gate_service.go:30-40, TestStrictPRReadinessBlocksOnEvidencePendingOnly, TestAdvancePhaseGuardsPRWithGates |
| CHECK 없는(`has_check: false`) 게이트는 4단계로 돌려 게이트로 올린다 | skills/issueops-verify/SKILL.md:73-74,88-89 | sentence | - |
| verify-work는 읽기 전용 allowlist 명령만 받는다 | skills/issueops-verify/SKILL.md:90-91 | code | internal/adapter/policy/policy_run.go:19, internal/domain/verifywork/result.go:55, TestRunReadOnlyCommandRefusesWriteCapableArguments, TestBuildVerifyWorkMarksDeniedCommandFailed |
| 스키마 변경이면 measurement와 source를 짝으로 기록한다(키 `schema_evidence`, `schema_evidence_stale`) | skills/issueops-verify/SKILL.md:112-129 | gate | internal/domain/issueopsreview/gates.go:45-62, internal/domain/issueopsreview/record_validation.go:51-64, TestSchemaEvidenceGateActivatesOnSchemaChange |
| 스키마 측정값은 추정이 아니라 실제 DB 관찰값이다 | skills/issueops-verify/SKILL.md:117-118,128 | sentence | - |
| 리뷰어에게 diff·계획·리뷰 finding·렌즈를 준다 | skills/issueops-verify/SKILL.md:140-161 | sentence | - |
| 구현 리뷰는 execution 사이클 전부에 필요하고 pass만 통과한다(키 `implementation_review`) | skills/issueops-verify/SKILL.md:142-144 | gate | internal/domain/issueopsreview/gates.go:14-27, internal/application/issueopscycle/pr_readiness.go:47-49, TestImplementationReviewMissingAppliesWhenExecutionExists |
| 구현 diff가 계획 시점 compatibility 판정과 다르면 판정을 갱신한다 | skills/issueops-verify/SKILL.md:178-188 | sentence | - |
| missing이 커밋·푸시 키뿐이면 커밋·푸시 뒤 pr로, 그 밖이면 앞 단계로 간다(키 `worktree_clean`, `upstream`) | skills/issueops-verify/SKILL.md:190-199 | gate | internal/domain/issueopsnext/classify.go:112, internal/domain/issueops/pr_git_readiness.go:63-91, internal/application/issueopscycle/phase_entry.go:54-55 |
| pr 진입 뒤 보고서 밖 경로가 바뀌었으면 complete를 거부한다 | skills/issueops-complete/SKILL.md:51,75-82 | code | internal/domain/issueopscompletion/input.go:71, internal/application/issueopscompletion/complete.go:126-144, TestCompleteRefusesHeadThatChangedCodeAfterPREntry |
| 보고서를 쓰기 전 `next`의 `_stale`을 보고 있으면 complete하지 않는다 | skills/issueops-complete/SKILL.md:75-82 | sentence | - |
| `확인한 것` 절에 결과만 적은 줄(`pass`)을 두지 않는다 | skills/issueops-create-pr/SKILL.md:35,108-110 | sentence | - |

### 컨텍스트·출력 비용

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 필요한 협업 스킬만 읽고 이슈·PR 생성 스킬을 함께 읽지 않는다. 측정 없이 성능 개선을 주장하지 않는다 | skills/issueops/SKILL.md:54-58, skills/issueops-create-issue/SKILL.md:219-221, skills/issueops-create-pr/SKILL.md:189-191 | sentence | - |

### 기타

| 규칙 요지 | 출처 | 강제 방식 | 근거 |
|---|---|---|---|
| 사용자 보고는 STE100 규칙(결론 먼저, 위험은 `경고`)을 따른다 | skills/issueops/SKILL.md:94-95 | sentence | - |
| 브랜치 이름은 `<issue-number>-<slug>`이고 이슈 번호와 맞는다 | skills/issueops-prepare/SKILL.md:89-91 | code | internal/domain/issueops/branch.go:11-24, TestValidateBranchPreservesAcceptedAndRejectedNames, TestValidatePreparationIssueNumber |

## 요약

| 실패 유형 | code | gate | validator | sentence | 합계 |
|---|---|---|---|---|---|
| 증거 없는 완료 | 10 | 6 | 0 | 13 | 29 |
| 범위 이탈 | 5 | 7 | 0 | 25 | 37 |
| 정체·중단 | 3 | 0 | 0 | 9 | 12 |
| 상태·권한 경쟁 | 9 | 5 | 0 | 4 | 18 |
| 원격 쓰기 품질 | 6 | 0 | 0 | 11 | 17 |
| 검증 누락 | 7 | 7 | 0 | 20 | 34 |
| 컨텍스트·출력 비용 | 0 | 0 | 0 | 1 | 1 |
| 기타 | 1 | 0 | 0 | 1 | 2 |
| 합계 | 41 | 25 | 0 | 84 | 150 |

## sentence 규칙 중 `.issueops/cautions`에 기록이 있는 것

- `.issueops/cautions/issueops-lifecycle.md`: actor 플래그 손조립, worktree 밖 편집, 메뉴 질문, 리뷰어 격리, raw provider 우회, preview·confirm 순서, prepare 금지 조작, raw git 삭제, golden 재생성
- `.issueops/cautions/issueops-stages.md`: 인계 전 승인 범위 대조, 세션 인계 순서, 메뉴 질문, worktree 생성. `.issueops/cautions/issueops-execution.md`: prepare 금지 조작
- `.issueops/cautions/2026-08-21-issueops-whoami-record-flags-and-branch-examples.md`(actor 플래그), `.issueops/cautions/2026-10-19-herdr-claim.md`(인계와 launch 재시도), `.issueops/cautions/2026-09-20-manual-handoff-state-root-and-next-generation-claim.md`(인계문 digest)
- `.issueops/cautions/2026-09-24-sync-base-stale.md`, `.issueops/cautions/2026-08-04-released-sync-base-conflict-write-lease.md`: sync-base를 스스로 apply하지 않기, 승인된 계획 수정 금지
- `.issueops/cautions/2026-10-08-tracked-intent-copies-quote-the-raw-request-and-local-self-v.md`: 원문 `--raw-request`, 추적 사본 수정 금지
- `.issueops/cautions/2026-09-08-review-checks-enter-the-ledger-at-stage-4-entry-never-during.md`, `.issueops/cautions/2026-08-26-gates-root-ledger-worktree-conflicts.md`: `gates init`과 `gates check --file`
- `.issueops/cautions/audit-and-process.md`(RED 선행, 검증 기록), `.issueops/cautions/2026-08-21-api-doc-route-block-assembly-and-evidence-bundling.md`(api-doc gate)
- `.issueops/cautions/2026-08-28-devils-advocate-record-was-unbound.md`, `.issueops/cautions/2026-09-08-issueops-regress-is-refused-while-the-devil-s-advocate-verdi.md`: 리뷰어 격리, revise waive
- `.issueops/cautions/2026-10-14-completion-does-not-check-the-head-was-verified.md`, `.issueops/cautions/2026-09-02-fingerprint-sealing-gates-recorded-after-edit.md`: 보고서 전 `_stale` 확인
- `.issueops/cautions/2026-08-27-cleanup-stops-worktree-processes.md`: 수동 프로세스 종료

## 본문과 코드가 어긋나는 곳 (고치지 않음)

- `skills/issueops-prepare/SKILL.md:96-97`은 치환이 든 명령을 lifecycle guard가 거부한다고 쓴다. PreToolUse hook은 폐기됐다(`TestRetiredHookSubcommandsAreRejected`). 같은 파일 `:40-41`은 `gh issue develop`를 금지하지만 `internal/adapter/issueops/branchinstructions/steps.go:130-133`은 base SHA가 비면 그 명령을 안내한다.
- `skills/issueops-create-issue/SKILL.md:84`는 항목이 비면 grill 진입이 거부된다고 쓴다. grill 진입은 intent 필드만 본다(`internal/application/issueopscycle/phase_entry.go:39-40`).
- `skills/issueops-create-issue/SKILL.md:185`의 plan-prep waive 금지, `skills/issueops-plan/SKILL.md:179`의 revise waive 금지와 달리 코드는 rationale이 있는 waive를 받는다.
- `skills/issueops-verify/SKILL.md:190-191`은 커밋·푸시 키를 넷으로 든다. `next`의 `commitPushLocalKeys`는 둘이다(`internal/domain/issueopsnext/classify.go:112`).
- `.issueops/cautions/issueops-stages.md:93-95`는 fingerprint에 검증 보고서가 들어간다고 쓴다. 코드는 보고서를 뺀다(`internal/adapter/issueops/implementation/evidence.go:162-166`).
