# 계획 검토 기록

## 1차: 수정 요청

- D1: reflect-completion은 provider가 확인한 머지 증거를 요구하므로(issueops_completion_remote.go:45) 머지 전 단계(P3, issueops-complete)에서 실행되지 않고, --body-file 필수화가 issueops-cleanup 등 문서의 기존 호출을 깨며, cleanup remote-branch의 재렌더가 진행 결과를 지운다.
- D2: --confirm에 --template을 강제하면 owner prompt가 렌더하는 create-pr 명령(execution_owner_context.go:392)이 거부된다. create-pr·create-child는 유일 템플릿을 기본값으로 쓰고 create-issue만 필수로 둔다.
- D3: 추적 .gitignore:36이 .issueops/issues/*/artifact/를 무시하므로 artifact 무시 해제는 효과가 없고 plan·spec 보존 수단만 없앤다. CONVENTIONS 표대로 .issueops/issues/<n>/에 추적 사본을 만든다.
- D4: child 위임은 사용자 범위 밖인 중간 머지 2회를 임계 경로에 두고, child 이슈 연결·child plan·parent push 단계가 빠졌다.
- D5: reflect-devils-advocate에 critical 거부를 붙이면 짧은 한국어 판정이 한글 20자 하한에 걸려 stop 반영과 regress가 막힌다.
- D6: 게이트가 sync·create-child·create-pr·reflect-completion 거부와 구현 이슈 5절을 덮지 못하고, 120초 기본 제한 때문에 전체 테스트·race·self-verify가 한 run에 met이 되지 않는다.
- D7: port DTO 변경의 비목표 해석과 --keep-remote-branch 면제가 기대던 감사 줄의 대체 경로가 기록되지 않았다.

## 2차: 수정 요청

- R2-1: 추적 사본을 link-plan에만 묶었는데 prepare가 direct(issueops_artifact_stage.go:213)와 Orca(execution_prepare_bridge.go:136) 모두에서 plan_path를 미리 채워 link-plan이 생략되므로, 완료 구간에서 plan·spec을 뺀 뒤 cleanup finish에서 구현 자료가 사라진다.
- R2-2: 진행 결과 원고의 커밋 SHA 전문·로컬 경로가 warning이라 성공 기준 4가 사람이 쓴 내용에 보장되지 않고, 중단 판정 지적도 검사 없이 렌더되며 G14·G16은 깨끗한 입력만 본다.
- R2-3: 구조를 PR 하나로 바꿨는데 intent 해석은 여전히 child 2개를 말하고, Gap Analysis 12가 이 본문 사실 변경을 형식 전환으로 잘못 분류했다.
- R2-4: G20에 게이트 작성 지침을 소유한 gates-ledger가 없고, G10은 거부가 BeginIssueCreateIntent 전에 일어나는지 확인하지 않는다.

## 3차: 통과

- 3라운드: 리뷰 모델 fable(기본 claude-opus-5와 다른 모델), 2라운드 지적에 대한 delta 리뷰
- 전이 안 사본 쓰기의 span·fingerprint 순서·child·Orca 위치·contract_change 게이트 순서를 공격했으나, 기존 전이가 이미 span 안에서 git을 실행하고 clean 단계의 ai-slop-clean record가 사본을 포함해 다시 봉인하며 contract_feedback_issue_update가 7단계 next에서 먼저 드러나므로 모두 통과했다.
- 구현 메모 A(차단 아님): PlanPath가 봉인 artifact 디렉터리 밖(이미 추적되는 경로)이면 plan.md 사본을 생략해 추적 중인 plan.md를 덮어쓰지 않는다(T12 2항).
- 구현 메모 B(차단 아님): local_path·commit_sha_full critical 상향과 result_too_long은 Kind가 completion일 때만 적용하고 템플릿 부재와 같은 뜻으로 쓰지 않는다(T2).
- 구현 메모 C(차단 아님): IssueOpsRecord에 Warnings 필드가 없으므로 전이 응답의 사본 warning은 AdvanceIssueOpsPhaseWithActor 반환형이나 CLI 래퍼로 전달하고 record 필드는 추가하지 않는다(호출자: issueops_subcommands.go, loopgate, gatesgate).
