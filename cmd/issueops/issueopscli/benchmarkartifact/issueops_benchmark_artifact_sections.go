package benchmarkartifact

import (
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
)

func prDraft(fixture issueopscontract.IssueOpsBenchmarkFixture, issueNumber, branchName, worktreePath, guideline, expectedIssue, expectedPR string) string {
	return strings.Join([]string{
		"## 요약",
		"",
		"issue #" + issueNumber + "의 IssueOps 품질 요구사항을 한국어 이슈, 계획, TDD, subagent, worktree, cleanup 루프로 충족하기 위한 PR/MR 초안입니다. 품질 기준이 모호하면 실제 PR/MR 오픈과 구현은 사용자 확인 전까지 막습니다.",
		"Issue: https://example.com/acme/issueops/issues/" + issueNumber,
		"",
		"## 변경 내용",
		"",
		"- 예정 변경: 문제 파악과 domain grill 이후 issue contract를 확정한다.",
		"- 예정 변경: issue 기반 브랜치 `" + branchName + "`와 격리 worktree `" + worktreePath + "`에서만 구현한다.",
		"- 예정 변경: worker prompt에 pwd, branch, HEAD, worktree 검증과 bounded review 제약을 넣는다.",
		"- 예정 변경: 작업 완료 후 clean 상태와 worktree cleanup/remove 선택지를 기록한다.",
		"- 현재 상태: 아직 구현하지 않았고, clarification과 branch/worktree gate가 통과된 뒤에만 TDD/구현/PR 오픈으로 진행한다.",
		"",
		"## 확인한 것",
		"",
		"- `./bin/issueops benchmark run --fixtures testdata/issueops/fixtures --judge file --judge-file judge-map.json --json`로 fixture `" + fixture.ID + "`(" + strings.TrimSpace(fixture.Title) + ")를 채점한다. 목표는 average_score 100, minimum_score 100, critical_failure_count 0이다.",
		"- `go test ./... -count=1`과 `git status --short --branch`로 테스트와 작업 공간 상태를 확인한다.",
		"- 결론만 보고하고 번호 선택지를 주지 않는 응답은 실패 예시로 기록한다.",
		"- Expected issue evidence:",
		expectedIssue,
		"- Expected PR/MR evidence:",
		expectedPR,
		"",
		"## 리뷰 포인트",
		"",
		"- remote artifact template validation, 가독성 검사, label/assignee evidence, target branch가 맞는지 봐 주세요.",
		"- `PR/MR 머지 완료했습니다`처럼 cleanup 선택지 없이 끝나는 보고는 불완전합니다. cleanup status가 남는지 봐 주세요.",
		"",
		"## 위험과 되돌리기",
		"",
		"- LLM judge 점수는 변동될 수 있어 deterministic scorer와 함께 확인한다.",
		"- 되돌리기: renderer/template 변경을 되돌리고 기존 dry-run body path로 복귀한다.",
		"",
		"Guideline: " + guideline + "\n",
	}, "\n")
}

func phaseChoices() string {
	return strings.Join([]string{
		"선택지:",
		"1. Proceed: 다음 IssueOps phase로 진행한다. (추천)",
		"2. Revise: 현재 phase의 issue/plan/task contract를 수정한다.",
		"3. Jump: issue, plan, implementation, ai-slop-clean, feedback, PR phase 중 필요한 단계로 이동한다.",
		"4. Pause: 사용자 결정 전까지 진행을 멈춘다.",
	}, "\n")
}

func worktreeCleanup() string {
	return strings.Join([]string{
		"clean worktree confirmed after merge; cleanup/remove choices are offered and present.",
		"선택지:",
		"1. Cleanup: merged worktree and local branch를 삭제한다. (추천)",
		"2. Keep: worktree를 보존하고 나중에 확인한다.",
		"3. Inspect: stale IssueOps worktree 전체를 점검한 뒤 삭제 후보를 제시한다.",
	}, "\n")
}

func domainContractEvidence() string {
	return strings.Join([]string{
		"Invariant: preserve the user-visible behavior described by the issue.",
		"Exact mechanism: compare the documented mechanism with source file:line evidence before implementation.",
		"Equivalent behavior: if the exact mechanism is absent, record whether another verified path enforces the same invariant.",
		"Source: current files, docs, logs, or command output must be cited before claiming completion.",
	}, "\n")
}

func apiDocGateEvidence() string {
	return strings.Join([]string{
		"Changed endpoint list is recorded, or the plan states that no endpoint contract changed.",
		"Public error responses are checked against service/usecase/error-mapping behavior.",
		"Static check: api_doc_static_check or the target repo's equivalent API doc command.",
		"Review: api_doc_review for OpenAPI/Swagger/API doc parity when endpoint contracts change.",
	}, "\n")
}

func liveEvidenceMatrix() string {
	return strings.Join([]string{
		"Environment matrix covers dev, stg, prod, or the target repo's equivalent runtime surfaces.",
		"Repo config evidence is compared with runtime evidence before assigning root cause.",
		"Runtime evidence records live config, logs, deployed code, or external service probes where available.",
		"Remediation order is recorded before edits when multiple fixes are needed.",
	}, "\n")
}

func reviewFeedbackEvidence() string {
	return strings.Join([]string{
		"Classification: valid_review, stale_review, contract_change, defect, question, noise, rollout_evidence_missing, or environment_debt from Kodus, Gemini Code Assist, review-agent, human review, QA, or CI.",
		"Verification: file/line, command output, diff evidence, or live evidence decides validity.",
		"Thread reply: original review thread gets a verdict and evidence.",
		"Resolution: unresolved, fixed, resolveReviewThread/resolved=true, obsolete, or split to follow-up is re-checked.",
	}, "\n")
}

func completionHygiene() string {
	return strings.Join([]string{
		"Draft issue completion record includes final diff, evidence, labels, children, PR URL, and unresolved follow-ups.",
		"Final diff is reviewed from the actual worktree.",
		"Target branch and source branch are verified before remote artifact updates.",
		"Remote artifact issue/PR/MR body is refreshed against the final implementation.",
		"Single commit policy is checked, or multiple commits are explicitly justified.",
		"Cleanup status is recorded with worktree and branch checks.",
	}, "\n")
}
