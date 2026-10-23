package issueopsnext

import "strings"

// OwnerCommand는 readiness가 돌려준 missing 키 하나를 그 상태를 소유한 명령으로
// 옮긴다. 이 함수가 그 대응의 유일한 소유자다 — 같은 표를 스킬 문서에도 두면
// 한쪽만 낡는다.
//
// 반환 문자열의 `<...>`는 사람이 채울 자리표시자이며, 그 존재가 곧
// next_command_kind=template이다. 알 수 없는 키는 추측하지 않고 status로 보낸다.
func OwnerCommand(id, missingKey string) string {
	key := strings.TrimSpace(missingKey)
	for _, lookup := range ownerCommandLookups {
		if command, ok := lookup(id, key); ok {
			return command
		}
	}
	return statusCommand(id)
}

// ownerCommandLookups는 위에서 아래로 본다. 접두사 군이 정확 일치 키보다 먼저다.
var ownerCommandLookups = []func(id, key string) (string, bool){
	prefixOwnerCommand,
	intakeOwnerCommand,
	planningOwnerCommand,
	executionOwnerCommand,
	reviewOwnerCommand,
	publicationOwnerCommand,
}

// prefixOwnerCommand는 접두사로 묶이는 missing 키 군의 명령이다.
func prefixOwnerCommand(id, key string) (string, bool) {
	switch {
	case strings.HasPrefix(key, "plan_prep_"):
		return "issueops plan-prep record --id " + id +
			" --decisions-evidence <TEXT> --related-score-ref <TEXT> --web-research-evidence <TEXT> --codebase-survey-evidence <TEXT>", true
	case strings.HasPrefix(key, "architecture_") && key != "architecture_artifact":
		return architectureReviewCommand(id), true
	case strings.HasPrefix(key, "compatibility_"):
		return compatibilityReviewCommand(id), true
	case strings.HasPrefix(key, "ai_slop_clean_"):
		return aiSlopCleanCommand(id), true
	case strings.HasPrefix(key, "gates_incomplete:"):
		return "issueops gates check --cwd <worktree> --workspace-root <worktree> --json", true
	}
	return "", false
}

// intakeOwnerCommand는 1단계(의도·도메인·분할·이슈·브랜치) 키의 명령이다.
func intakeOwnerCommand(id, key string) (string, bool) {
	switch key {
	case "intent_contract", "raw_request", "interpreted_intent", "success_criteria":
		return "issueops intent record --id " + id +
			" --raw-request <TEXT> --interpreted-intent <TEXT> --success-criteria <TEXT>", true
	case "domain_review":
		return "issueops domain-review record --id " + id + " --model-fit <TEXT>", true
	case "split_decision":
		return "issueops decision add --id " + id +
			" --kind scope --title <TEXT> --body <TEXT>", true
	case "issue_url":
		return "issueops link-issue --id " + id + " --issue-url <URL>", true
	case "branch", "branch_prepare":
		return "issueops branch prepare --id " + id +
			" --provider <github|gitlab> --issue-url <URL> --branch <NAME> --base-branch <REF> --base-sha <SHA> --link-verified", true
	}
	return "", false
}

// planningOwnerCommand는 2~3단계(아키텍처·호환성·반론·계획) 키의 명령이다.
func planningOwnerCommand(id, key string) (string, bool) {
	switch key {
	case "architecture_artifact":
		return "issueops artifact stage --id " + id + " --name architecture --file <PATH>", true
	case "architecture_review":
		return architectureReviewCommand(id), true
	case "compatibility_review":
		return compatibilityReviewCommand(id), true
	case "devils_advocate_review", "devils_advocate_review_stale":
		return "issueops devils-advocate review --id " + id +
			" --reviewer-context subagent --verdict <pass|revise|stop> --finding <TEXT>", true
	case "plan_artifact":
		return "issueops artifact stage --id " + id + " --name plan --file <PATH>", true
	case "plan_path", "plan_exists", "plan_in_worktree":
		return "issueops link-plan --id " + id + " --plan-path <PATH>", true
	}
	return "", false
}

// executionOwnerCommand는 실행 준비·상태·정리 증거 키의 명령이다.
func executionOwnerCommand(id, key string) (string, bool) {
	switch key {
	case "worktree_path", "worktree_exists":
		return "issueops execution prepare --id " + id + " $ACTOR_FLAGS", true
	case "execution", "execution_valid", "execution_worktree_match", "execution_write_lease":
		return executionStatusCommand(id), true
	case "implementation_changes":
		return statusCommand(id), true
	case "cleanup_evidence", "verification_evidence", "current_fingerprint":
		return aiSlopCleanCommand(id), true
	}
	return "", false
}

// reviewOwnerCommand는 검토·문서·스키마·피드백 증거 키의 명령이다. 모두
// RECORD_ACTOR_FLAGS를 요구하므로(lease가 있으면 holder 검사) 템플릿에 자리를 둔다.
func reviewOwnerCommand(id, key string) (string, bool) {
	switch key {
	case "implementation_review", "implementation_review_stale":
		return "issueops implementation-review record --id " + id +
			" --verdict <pass|revise|stop> --finding <TEXT> --evidence <TEXT> $RECORD_ACTOR_FLAGS", true
	case "project_docs_review", "project_docs_review_stale":
		return "issueops project-docs-review record --id " + id +
			" --verdict <updated|no-change> --reviewed-doc <PATH> --evidence <TEXT> $RECORD_ACTOR_FLAGS", true
	case "schema_evidence", "schema_evidence_stale":
		return "issueops schema-evidence record --id " + id +
			" --measurement <TEXT> --source <TEXT> $RECORD_ACTOR_FLAGS", true
	case "feedback_classification":
		return "issueops feedback add --id " + id +
			" --source <TEXT> --body <TEXT> --classification <TEXT> $RECORD_ACTOR_FLAGS", true
	case "feedback_resolution":
		return "issueops feedback resolve --id " + id +
			" --index <N> --resolution <valid-defect|question-answered|noise-dismissed> $RECORD_ACTOR_FLAGS", true
	case "contract_feedback_issue_update":
		return "issueops feedback mark-issue-updated --id " + id + " $RECORD_ACTOR_FLAGS", true
	}
	return "", false
}

// publicationOwnerCommand는 원격 산출물·자식·PR 준비 키의 명령이다.
func publicationOwnerCommand(id, key string) (string, bool) {
	switch key {
	case "remote_artifact":
		return "issueops remote verify-artifact --id " + id +
			" --provider <github|gitlab> --kind <pr|mr> --url <URL> --target-branch <BRANCH> --label <LABEL> --assignee <USER> $RECORD_ACTOR_FLAGS", true
	case "child_incomplete", "child_unvalidated", "child_rejected_unresolved":
		return "issueops child status --parent " + id + " --json", true
	case "worktree_clean", "upstream", "upstream_fetch", "upstream_synced", "branch_match":
		return "issueops pr-readiness --id " + id + " --strict --json", true
	}
	return "", false
}

func architectureReviewCommand(id string) string {
	return "issueops architecture review --id " + id +
		" --problem-summary <TEXT> --proposed-architecture <TEXT> --refactor-plan <TEXT>" +
		" --alternative <TEXT> --risk <TEXT> --verification <TEXT> --reviewer-context subagent --approved"
}

func compatibilityReviewCommand(id string) string {
	return "issueops compatibility review --id " + id +
		" --backward-compatibility <TEXT> --side-effect <TEXT> --rollback-plan <TEXT> --verification <TEXT> --approved"
}

func aiSlopCleanCommand(id string) string {
	return "issueops ai-slop-clean record --id " + id +
		" --category <TEXT> --verification <TEXT>"
}

func executionStatusCommand(id string) string {
	return "issueops execution status --id " + id + " --json"
}

func statusCommand(id string) string {
	return "issueops status --id " + id + " --json"
}
