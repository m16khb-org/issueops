// Package issueopsnext classifies which stage of the ten-stage IssueOps cycle a
// record is in. It is pure: no filesystem, git, process, or provider access —
// every observation arrives through Input, so the same record always yields the
// same decision and the rules are table-testable.
package issueopsnext

import (
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// Readiness 타입은 주입된 게이트 판정이다. adapter의 IssueOpsReadiness에서 이
// 분류기가 쓰는 두 필드만 남긴 것이다.
type Readiness struct {
	Ready   bool
	Missing []string
	// Warnings는 차단하지 않는 관측이다. readiness가 이미 내던 경고를 버리지
	// 않고 `next`까지 전달한다(base drift가 이 경로로 보인다).
	Warnings []string
}

// Input 타입은 분류에 필요한 모든 관측을 담는다. 분류기는 I/O를 하지 않으므로
// 파일시스템, git, 프로세스 관측은 호출자가 미리 채워 넣어야 한다.
type Input struct {
	Record issueopsnextcontract.Record
	// Completion은 phase별 완료 판정이다(adapter IssueOpsPhaseCompletion).
	Completion func(phase issueopsnextcontract.Phase) Readiness
	// Local은 fetch 없는 PR readiness다. phase가 ai-slop-clean·feedback일 때만
	// 채운다. nil이면 5~8단계 규칙이 발화하지 않는다.
	Local *Readiness
	// StagedPlan은 `artifact stage --name plan`이 올라와 있는지다.
	StagedPlan bool
	// StagedArchitecture는 `artifact stage --name architecture`가 올라와 있는지다.
	StagedArchitecture bool
	// ArchitectureReviewStale은 승인한 아키텍처 문서가 스테이징된 문서와 달라졌다는 관측이다.
	ArchitectureReviewStale bool
	// DevilsAdvocatePlanStale는 계획 리뷰가 묶인 digest와 스테이징된 계획이 달라졌다는
	// 관측이다. link-plan 전에는 implement readiness가 이것을 보지 못한다.
	DevilsAdvocatePlanStale bool
	// ClaimActorFlags는 호출 세션의 claim 벡터(whoami claim_actor_flags)를 주어진
	// cwd로 만든다. host 영수증을 관측할 수 없으면 ""다. finish가 필요할 때만 부른다.
	ClaimActorFlags func(cwd string) string
	// RootConflictID는 이 사이클의 canonical worktree root를 이미 점유한 다른
	// 사이클의 id다. 없으면 빈 문자열이다.
	RootConflictID string
	// WriterlessRecovery는 writer 없는 lease의 회복 명령이다(adapter가 렌더).
	// 비어 있으면 replace --preview로 떨어진다.
	WriterlessRecovery string
	ActorHost          string
	ActorSessionID     string
	// HolderLive는 holder 프로세스 관측 결과다. holder가 없거나 관측하지
	// 않았으면 nil이다.
	HolderLive      *bool
	WorktreePresent bool
	WorktreeHead    string
	// WorktreeBranch is the branch checked out in the canonical worktree
	// ("" when detached or not observed).
	WorktreeBranch string
	// GitOperation is an unfinished merge, rebase, cherry-pick or revert in the
	// canonical worktree ("" when none).
	GitOperation string
	SourceRoot   string
	// ChildGateMissing holds the PR-gate keys of delegated children that are
	// not integrated yet (child_incomplete:<id>, child_unvalidated:<id>, ...),
	// observed only for a parent in implement.
	ChildGateMissing []string
}

// Decision 타입은 사이클의 현재 단계와 그 단계를 진행시킬 다음 명령, 막힌
// 게이트(Missing), 비차단 경고를 담는 분류 결과다.
type Decision struct {
	Stage           issueopsnextcontract.Stage
	Lease           issueopsnextcontract.Lease
	Missing         []string
	NextCommand     string
	NextCommandKind string
	Exits           issueopsnextcontract.Exits
	Warnings        []string
}

// 1단계 게이트의 고정 순서다. missing은 알파벳 정렬로 돌아오므로 첫 항목을
// 그대로 쓰면 2단계 항목인 branch가 먼저 나온다.
var stageOneOrder = []string{
	"intent_contract", "raw_request", "interpreted_intent", "success_criteria",
	"domain_review", "split_decision",
	"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey",
	"issue_url",
}

var cleanCompletionKeys = []string{
	"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint",
	"cleanup_evidence", "verification_evidence",
}

var cleanLocalKeys = []string{"ai_slop_clean_stale", "ai_slop_clean_fingerprint", "current_fingerprint"}

var docsLocalKeys = []string{"project_docs_review", "project_docs_review_stale"}

var verifyLocalKeys = []string{
	"schema_evidence", "schema_evidence_stale",
	"implementation_review", "implementation_review_stale",
	"feedback_classification", "feedback_resolution", "contract_feedback_issue_update",
}

// commitPushLocalKeys는 8단계로 넘어가도 되는 잔여 missing이다. 이 둘은 커밋과
// 푸시가 해소하므로 검증 단계로 되돌릴 이유가 없다.
var commitPushLocalKeys = []string{"worktree_clean", "upstream"}

// Classify 함수는 규칙을 위에서 아래로 본다. 먼저 맞은 규칙이 결과를 결정한다.
func Classify(in Input) Decision {
	decision := Decision{Lease: leaseView(in)}
	if strings.TrimSpace(in.Record.ID) == "" {
		decision.Stage = stage(issueopsnextcontract.StageNone, 0)
		decision.NextCommand = "issueops start --repo " + placeholder(in.SourceRoot, "<source_root>") + " --json"
		return finish(in, decision)
	}
	decision.Exits = exits(in)
	decision.Warnings = warnings(in)

	index := phaseIndex(in.Record.Phase)

	if decided, ok := classifyCleanupResume(in, decision, index); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyMergeInProgress(in, decision); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyBlockedExecution(in, decision, index); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyLease(in, decision, index); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyPreparation(in, decision); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyImplementation(in, decision); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyPublication(in, decision, index); ok {
		return finish(in, decided)
	}
	return finish(in, unknownDecision(in, decision, index))
}

// unknownDecision은 어느 규칙도 맞지 않을 때의 status 안내다.
func unknownDecision(in Input, decision Decision, index int) Decision {
	decision.Stage = stage(issueopsnextcontract.StageUnknown, index)
	decision.NextCommand = "issueops status --id " + in.Record.ID + " --json"
	if in.Local != nil && len(in.Local.Missing) > 0 {
		decision.Missing = append([]string{}, in.Local.Missing...)
		decision.Warnings = append(decision.Warnings, "unmatched readiness gates: "+strings.Join(in.Local.Missing, ", "))
	}
	return decision
}

func stage(key string, index int) issueopsnextcontract.Stage {
	return issueopsnextcontract.Stage{Key: key, Index: index}
}

// finish fills $RECORD_ACTOR_FLAGS and $ACTOR_FLAGS when next knows the
// caller's vectors at the cycle's canonical cwd ($ACTOR_FLAGS also needs an
// observable host receipt), and then sets next_command_kind from the
// placeholders that remain. One owner keeps rules from hand-writing the kind.
func finish(in Input, decision Decision) Decision {
	if strings.TrimSpace(decision.NextCommand) == "" {
		return decision
	}
	if cwd := actorCWD(in); cwd != "" {
		decision.NextCommand = strings.ReplaceAll(decision.NextCommand, "$RECORD_ACTOR_FLAGS", recordActorFlags(in))
		if in.ClaimActorFlags != nil && strings.Contains(decision.NextCommand, "$ACTOR_FLAGS") {
			if claim := in.ClaimActorFlags(cwd); claim != "" {
				decision.NextCommand = strings.ReplaceAll(decision.NextCommand, "$ACTOR_FLAGS", claim)
			}
		}
	}
	decision.NextCommand = fillBranchPrepareIssue(decision.NextCommand, in.Record.IssueURL)
	// Every issueops command answers JSON with --json; several owner commands
	// were rendered without it, and an agent running link-plan as printed got
	// a one-line text answer it could not check (two QA runs).
	decision.NextCommand = withJSON(decision.NextCommand)
	decision.Exits.PauseCommand = withJSON(decision.Exits.PauseCommand)
	decision.Exits.AbandonCommand = withJSON(decision.Exits.AbandonCommand)
	decision.Exits.TakeoverCommand = withJSON(decision.Exits.TakeoverCommand)
	if strings.Contains(decision.NextCommand, "<") || strings.Contains(decision.NextCommand, "ACTOR_FLAGS") {
		decision.NextCommandKind = issueopsnextcontract.NextCommandKindTemplate
		return decision
	}
	decision.NextCommandKind = issueopsnextcontract.NextCommandKindExact
	return decision
}
