// Package issueopsnext classifies which stage of the ten-stage IssueOps cycle a
// record is in. It is pure: no filesystem, git, process, or provider access —
// every observation arrives through Input, so the same record always yields the
// same decision and the rules are table-testable.
package issueopsnext

import (
	"slices"
	"strconv"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/shelltoken"
)

// Readiness는 주입된 게이트 판정이다. adapter의 IssueOpsReadiness에서 이
// 분류기가 쓰는 두 필드만 남긴 것이다.
type Readiness struct {
	Ready   bool
	Missing []string
	// Warnings는 차단하지 않는 관측이다. readiness가 이미 내던 경고를 버리지
	// 않고 `next`까지 전달한다(base drift가 이 경로로 보인다).
	Warnings []string
}

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

// Classify는 규칙을 위에서 아래로 본다. 먼저 맞은 규칙이 결과를 결정한다.
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

// classifyBlockedExecution은 pending execution과 이미 점유된 canonical root처럼
// lease를 보기 전에 사이클을 멈추는 두 차단 규칙이다.
func classifyBlockedExecution(in Input, decision Decision, index int) (Decision, bool) {
	execution := in.Record.Execution
	if execution != nil && execution.Pending != nil {
		// Reconcile needs the live lease holder. When the holder died inside
		// the write (QA: a killed create-pr), take the lease over first; the
		// new holder reconciles the pending intent it inherits.
		lease := execution.Lease
		if (lease.Status == issueopsnextcontract.LeaseStatusActive && lease.Holder != nil && !holderIsSelf(in) && !live(in.HolderLive)) ||
			lease.Status == issueopsnextcontract.LeaseStatusRevoking {
			decision.Stage = stage(issueopsnextcontract.StageTakeover, index)
			decision.NextCommand = takeoverCommand(in)
			return decision, true
		}
		if lease.Status == issueopsnextcontract.LeaseStatusClaimable {
			// The takeover finalized with the intent carried over: claim
			// first; the claimed lease then shows blocked.pending.
			return decision, false
		}
		decision.Stage = stage(issueopsnextcontract.StageBlockedPending, index)
		decision.NextCommand = "issueops execution reconcile --id " + in.Record.ID + " --preview $ACTOR_FLAGS --json"
		return decision, true
	}
	if execution == nil && issueopsdomain.IssueOpsPhaseRank(in.Record.Phase) >= issueopsdomain.IssueOpsPhaseRank(issueopsnextcontract.PhasePlan) &&
		strings.TrimSpace(in.RootConflictID) != "" {
		decision.Stage = stage(issueopsnextcontract.StageBlockedRoot, 3)
		decision.NextCommand = "issueops list --repo " + placeholder(in.SourceRoot, "<source_root>") + " --json"
		decision.Warnings = append(decision.Warnings, "canonical worktree root is already claimed by cycle "+strings.TrimSpace(in.RootConflictID))
		return decision, true
	}
	return decision, false
}

// classifyLease는 execution lease 상태가 다음 행동을 정하는 규칙이다: 다른
// 살아 있는 holder, 인수, 아직 claim되지 않은 lease.
func classifyLease(in Input, decision Decision, index int) (Decision, bool) {
	execution := in.Record.Execution
	if execution == nil {
		return decision, false
	}
	id := in.Record.ID
	lease := execution.Lease
	holderOther := lease.Holder != nil && !holderIsSelf(in)
	if lease.Status == issueopsnextcontract.LeaseStatusActive && holderOther && live(in.HolderLive) {
		decision.Stage = stage(issueopsnextcontract.StageBlockedHolderLive, index)
		decision.NextCommand = "issueops execution status --id " + id + " --json"
		return decision, true
	}
	if (lease.Status == issueopsnextcontract.LeaseStatusActive && holderOther && !live(in.HolderLive)) ||
		lease.Status == issueopsnextcontract.LeaseStatusRevoking {
		decision.Stage = stage(issueopsnextcontract.StageTakeover, index)
		decision.NextCommand = takeoverCommand(in)
		return decision, true
	}
	if (lease.Status == issueopsnextcontract.LeaseStatusClaimable || lease.Status == issueopsnextcontract.LeaseStatusReleased) &&
		execution.Completion == nil {
		decision.Stage = stage(issueopsnextcontract.StageClaim, index)
		// The chain's first command, not a status read that only returns it.
		decision.NextCommand = issueopsdomain.ReplacementPreviewCommand(id, lease.Generation)
		if lease.Status == issueopsnextcontract.LeaseStatusClaimable {
			decision.NextCommand = issueopsdomain.ReplacementClaimCommand(id, lease.Generation)
			// The actor-free claim works only from the canonical worktree, and
			// QA agents distrusted an "exact" claim without flags. With an
			// observable receipt, pin this session's vector and the worktree.
			if root := strings.TrimSpace(execution.Workspace.Root); root != "" && in.ClaimActorFlags != nil {
				if flags := in.ClaimActorFlags(root); flags != "" {
					decision.NextCommand += " " + flags
				}
			}
		}
		return decision, true
	}
	return decision, false
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

// classifyPreparation은 execution이 아직 없는 1~3단계를 가른다. phase가 아니라
// artifact로 가르는 이유는 link-issue가 phase를 plan으로 자동 전진시키기
// 때문이다 — 이슈만 만든 사이클도 phase는 plan이다.
func classifyPreparation(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Execution != nil {
		return decision, false
	}
	if !planningPhase(record.Phase) {
		return decision, false
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return classifyIssueMissing(in, decision)
	}
	if branchPrepareNeeded(record) {
		return classifyBranchPrepare(in, decision)
	}
	if key := architectureGate(in); key != "" {
		decision.Stage = stage(issueopsnextcontract.StagePlanArchitecture, 3)
		decision.Missing = []string{key}
		decision.NextCommand = OwnerCommand(id, key)
		return decision, true
	}
	if !in.StagedPlan {
		decision.Stage = stage(issueopsnextcontract.StagePlanWrite, 3)
		decision.Missing = []string{"plan_artifact"}
		decision.NextCommand = OwnerCommand(id, "plan_artifact")
		return decision, true
	}
	if devilsAdvocateMissing := devilsAdvocateGate(in); devilsAdvocateMissing != "" {
		decision.Stage = stage(issueopsnextcontract.StagePlanReview, 3)
		decision.Missing = []string{devilsAdvocateMissing}
		decision.NextCommand = devilsAdvocateCommand(in)
		return decision, true
	}
	decision.Stage = stage(issueopsnextcontract.StagePlanHandoff, 3)
	decision.NextCommand = "issueops execution prepare --id " + id + " $ACTOR_FLAGS --json"
	return decision, true
}

// architectureGate는 아키텍처가 계획보다 먼저 통과해야 하는 조건 중 처음 막히는
// 키다. 부모 리뷰를 승계한 위임 자식은 자기 아키텍처 문서를 검토하지 않았으므로
// 문서 스테이징과 stale 판정을 건너뛴다.
func architectureGate(in Input) string {
	review := in.Record.ArchitectureReview
	if review != nil && review.Approved && review.ReviewerContext == issueopsdomain.ParentReviewPattern {
		return ""
	}
	switch {
	case !in.StagedArchitecture:
		return "architecture_artifact"
	case review == nil || !review.Approved:
		return "architecture_review"
	case in.ArchitectureReviewStale:
		return "architecture_review_stale"
	}
	return ""
}

// classifyIssueMissing는 이슈 URL이 없을 때 1단계로 가른다. problem/grill 외 phase는 분류하지 않는다.
func classifyIssueMissing(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	if record.Phase != issueopsnextcontract.PhaseProblem && record.Phase != issueopsnextcontract.PhaseGrill {
		return decision, false
	}
	decision.Stage = stage(issueopsnextcontract.StageIssue, 1)
	decision.Missing = stageOneMissing(in)
	decision.NextCommand = OwnerCommand(record.ID, firstOr(decision.Missing, "intent_contract"))
	// QA (lost create response): after "outcome unknown; do not retry", next
	// still offered create-issue, which could open a duplicate issue. An
	// unsettled create intent goes to the read-only reconcile preview first.
	if issueCreateUnsettled(record.IssueCreateIntent) {
		decision.Missing = append([]string{"issue_create_unsettled"}, decision.Missing...)
		decision.NextCommand = "issueops remote reconcile-issue --id " + record.ID + " --json"
	}
	return decision, true
}

// issueCreateUnsettled reports a create-issue whose provider outcome is not
// recorded as settled (the same statuses operational health flags).
func issueCreateUnsettled(intent *issueopsnextcontract.IssueCreateIntent) bool {
	if intent == nil {
		return false
	}
	switch intent.Status {
	case issueopsnextcontract.IssueCreateIntentPending, issueopsnextcontract.IssueCreateIntentInvokedUnknown,
		issueopsnextcontract.IssueCreateIntentURLObserved, issueopsnextcontract.IssueCreateIntentVerificationFailed,
		issueopsnextcontract.IssueCreateIntentReceiptFailed:
		return true
	}
	return false
}

// branchPrepareNeeded는 이슈 URL이 있는 사이클이 아직 2단계(브랜치 준비)에 머무는지 판정한다.
// provider의 이슈↔브랜치 연결이 확인되지 않았으면 2단계다. link-plan이 그 증거
// (branch_link_verified)를 요구하므로, 여기서 넘기면 4단계 진입에서야 막힌다.
func branchPrepareNeeded(record issueopsnextcontract.Record) bool {
	return record.Phase == issueopsnextcontract.PhaseProblem || record.Phase == issueopsnextcontract.PhaseGrill ||
		record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" ||
		!record.BranchPrepare.LinkVerified
}

func classifyBranchPrepare(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	decision.Stage = stage(issueopsnextcontract.StagePrepare, 2)
	if record.Phase == issueopsnextcontract.PhaseProblem {
		decision.Missing = stageOneMissing(in)
		decision.NextCommand = "issueops phase --id " + record.ID + " --to grill $RECORD_ACTOR_FLAGS --json"
		return decision, true
	}
	if bp := record.BranchPrepare; bp != nil && strings.TrimSpace(bp.BaseSHA) != "" && bp.LinkVerified {
		// Only the phase is behind (a child cycle starts in grill, and link-issue
		// does not advance it): re-preparing would loop on the same evidence.
		decision.NextCommand = "issueops phase --id " + record.ID + " --to plan $RECORD_ACTOR_FLAGS --json"
		if record.Phase == issueopsnextcontract.PhaseGrill && len(record.RegressEvents) > 0 {
			// QA (stop + regress): next showed only `phase --to plan`, as if the
			// stop asked for nothing before re-planning.
			decision.Warnings = append(decision.Warnings, "regressed from a plan stop: re-investigate first; when the requirement changed, record feedback add --classification contract_change, update the issue (sync-issue), feedback mark-issue-updated, and intent record, then run this phase --to plan and re-plan")
		}
		return decision, true
	}
	key := branchEvidenceKey(record)
	decision.Missing = []string{key}
	decision.NextCommand = OwnerCommand(record.ID, key)
	return decision, true
}

// branchEvidenceKey names the missing branch evidence: the prepare record
// itself, or only the provider link check on a prepared branch.
func branchEvidenceKey(record issueopsnextcontract.Record) string {
	if record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.BaseSHA) != "" && !record.BranchPrepare.LinkVerified {
		return "branch_link_verified"
	}
	return "branch_prepare"
}

// classifyImplementation은 홀더가 이 세션인 4~8단계를 가른다.
func classifyImplementation(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Execution != nil && record.Execution.Lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) {
		if next, ok := classifyHolderImplement(in, decision); ok {
			return next, true
		}
	}
	if record.Phase == issueopsnextcontract.PhaseAISlopClean {
		if missing := intersect(in.Completion(issueopsnextcontract.PhaseAISlopClean).Missing, cleanCompletionKeys); len(missing) > 0 {
			decision.Stage = stage(issueopsnextcontract.StageClean, 5)
			decision.Missing = missing
			decision.NextCommand = OwnerCommand(id, missing[0])
			return decision, true
		}
	}
	if !postImplementPhase(record.Phase) || in.Local == nil {
		return decision, false
	}
	return classifyLocalGates(in, decision)
}

// classifyHolderImplement는 활성 lease 홀더가 이 세션일 때 4단계(구현 진입/구현)를 가른다.
func classifyHolderImplement(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	switch record.Phase {
	case issueopsnextcontract.PhasePlan, issueopsnextcontract.PhaseCompatibilityReview:
		decision.Stage = stage(issueopsnextcontract.StageImplementEnter, 4)
		switch {
		case branchPrepareNeeded(record):
			// link-plan refuses without branch evidence; send the holder to
			// the prepare stage that owns it instead of a link-plan that fails.
			key := branchEvidenceKey(record)
			decision.Stage = stage(issueopsnextcontract.StagePrepare, 2)
			decision.Missing = []string{key}
			decision.NextCommand = OwnerCommand(id, key)
		case strings.TrimSpace(record.PlanPath) == "":
			decision.Missing = []string{"plan_path"}
			decision.NextCommand = linkPlanCommand(record)
		case record.CompatibilityReview == nil || !record.CompatibilityReview.Approved:
			decision.Missing = []string{"compatibility_review"}
			decision.NextCommand = OwnerCommand(id, "compatibility_review")
		default:
			decision.NextCommand = "issueops phase --id " + id + " --to implement $RECORD_ACTOR_FLAGS --json"
		}
		return decision, true
	case issueopsnextcontract.PhaseImplement:
		decision.Stage = stage(issueopsnextcontract.StageImplement, 4)
		// The transition needs the implementation itself (changes, focused
		// evidence); naming the gaps keeps an agent that just entered
		// implement from running the exit command before writing any code.
		if in.Completion != nil {
			decision.Missing = append([]string{}, in.Completion(issueopsnextcontract.PhaseImplement).Missing...)
		}
		// QA (child delegation): the parent got `phase --to ai-slop-clean`
		// while its children were not merged or accepted; the PR gate refuses
		// later and the cleanup seals go stale. Route to the children first.
		if len(in.ChildGateMissing) > 0 {
			decision.Missing = append(decision.Missing, in.ChildGateMissing...)
			decision.NextCommand = "issueops child status --parent " + id + " $RECORD_ACTOR_FLAGS --json"
			return decision, true
		}
		decision.NextCommand = "issueops phase --id " + id + " --to ai-slop-clean $RECORD_ACTOR_FLAGS --json"
		return decision, true
	}
	return decision, false
}

// classifyLocalGates는 구현 이후 phase의 로컬 미충족 게이트로 5~8단계를 가른다.
func classifyLocalGates(in Input, decision Decision) (Decision, bool) {
	id := in.Record.ID
	local := in.Local.Missing
	if missing := intersect(local, cleanLocalKeys); len(missing) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageClean, 5)
		decision.Missing = missing
		decision.NextCommand = OwnerCommand(id, missing[0])
		return decision, true
	}
	if missing := intersect(local, docsLocalKeys); len(missing) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageDocs, 6)
		decision.Missing = missing
		decision.NextCommand = OwnerCommand(id, missing[0])
		return decision, true
	}
	verify := intersect(local, verifyLocalKeys)
	verify = append(verify, prefixed(local, "gates_incomplete:")...)
	if len(verify) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageVerify, 7)
		decision.Missing = verify
		decision.NextCommand = OwnerCommand(id, verify[0])
		return decision, true
	}
	if subset(local, commitPushLocalKeys) {
		decision.Stage = stage(issueopsnextcontract.StageCommitPush, 8)
		decision.Missing = append([]string{}, local...)
		decision.NextCommand = "issueops pr-readiness --id " + id + " --strict --json"
		// The step itself is git, which issueops does not run; say which one
		// so the readiness re-check is not mistaken for the action (two QA
		// agents still read next_command as the step when the to-dos were
		// separate lines).
		var todo []string
		if slices.Contains(local, "worktree_clean") {
			todo = append(todo, "commit the changes (atomic-commit-push skill)")
		}
		if slices.Contains(local, "upstream") {
			todo = append(todo, "git push -u origin "+placeholder(in.Record.Branch, "<branch>"))
		}
		if len(todo) > 0 {
			decision.Warnings = append(decision.Warnings, "do first: "+strings.Join(todo, ", then ")+"; next_command only re-checks readiness afterwards")
		}
		if len(local) == 0 {
			// Committed and pushed: the pr transition is the next step (it runs
			// the strict readiness itself), not another readiness read.
			decision.NextCommand = "issueops phase --id " + id + " --to pr $RECORD_ACTOR_FLAGS --json"
		}
		return decision, true
	}
	return decision, false
}

func classifyPublication(in Input, decision Decision, index int) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Phase == issueopsnextcontract.PhasePR && record.Execution != nil {
		if record.RemoteArtifact == nil {
			if back, ok := classifyPRRefresh(in, decision); ok {
				return back, true
			}
			decision.Stage = stage(issueopsnextcontract.StagePRCreate, 9)
			decision.Missing = []string{"remote_artifact"}
			// create-pr requires canonical labels and assignees; the head and
			// base are the sealed branch and base, so they are filled in.
			decision.NextCommand = "issueops remote create-pr --id " + id +
				" --expected-generation " + generationText(in) +
				" --title <TEXT> --head " + placeholder(record.Branch, "<BRANCH>") + " --base " + placeholder(prBase(record), "<BRANCH>") +
				" --body-file <PATH> --label <LABEL> --assignee <USER> $ACTOR_FLAGS"
			return decision, true
		}
		if record.Execution.Completion == nil {
			decision.Stage = stage(issueopsnextcontract.StagePRComplete, 9)
			decision.NextCommand = "issueops execution complete --id " + id +
				" --generation " + generationText(in) +
				" --final-head <SHA> --verification-report <PATH> --remote-artifact-url " + record.RemoteArtifact.URL +
				" --verification <TEXT> $ACTOR_FLAGS --confirm"
			return decision, true
		}
	}
	if record.Phase == issueopsnextcontract.PhaseDone {
		decision.Stage = stage(issueopsnextcontract.StageDone, 10)
		// QA (Go cycle): after the merge and the remote-branch delete, next
		// kept sending the agent to merge-pr, whose preview names the
		// remote-branch step again. Reflecting the completion or closing the
		// issue requires the merge readback, so either receipt means the
		// merge is behind us: the finish preview reads what cleanup is left.
		if completion := record.RemoteCompletion; completion != nil && (completion.ReflectedAt != "" || completion.IssueClosedAt != "") {
			decision.NextCommand = "issueops cleanup finish --id " + id + " --preview --json"
			return decision, true
		}
		// merge-pr recorded the merge readback. cleanup status --merged reads
		// what is left (remote branch, completion, issue close) and names its
		// next_command; a fixed remote-branch preview repeated after the branch
		// was gone (QA).
		if completion := record.RemoteCompletion; completion != nil && completion.MergedAt != "" {
			decision.NextCommand = "issueops cleanup status --id " + id + " --merged --json"
			return decision, true
		}
		// next does not read the provider. merge-pr's preview does: it gives the
		// merge command, or points to cleanup when the PR is already merged.
		decision.NextCommand = "issueops remote merge-pr --id " + id + " --json"
		// Agents read a repeated merge-pr after the merge as "not merged yet".
		// QA: the old wording ("reports already_merged after a merge") read as
		// "already merged" before the merge too; state both answers.
		decision.Warnings = append(decision.Warnings, "next does not read the PR, so it names merge-pr before and after the merge: the preview answers merged:false with the merge command, or already_merged:true with the cleanup command")
		return decision, true
	}
	_ = index
	return decision, false
}

// classifyPRRefresh is the in-pr fix path before the PR exists (a sync-base
// merge, an edit): create-pr runs the strict readiness and completion compares
// against pr_head, so the seals are refreshed in order — `phase --to
// ai-slop-clean` (a refresh that stays in pr), then the stale reviews, then
// `phase --to pr`, which reseals pr_head at the new head.
func classifyPRRefresh(in Input, decision Decision) (Decision, bool) {
	id := in.Record.ID
	if in.Local != nil && len(in.Local.Missing) > 0 {
		if len(intersect(in.Local.Missing, cleanLocalKeys)) > 0 {
			decision.Stage = stage(issueopsnextcontract.StageClean, 5)
			decision.Missing = append([]string{}, in.Local.Missing...)
			decision.NextCommand = "issueops phase --id " + id + " --to ai-slop-clean $RECORD_ACTOR_FLAGS --json"
			return decision, true
		}
		return classifyLocalGates(in, decision)
	}
	prHead, head := strings.TrimSpace(in.Record.PRHead), strings.TrimSpace(in.WorktreeHead)
	if in.Local != nil && prHead != "" && head != "" && prHead != head {
		decision.Stage = stage(issueopsnextcontract.StageCommitPush, 8)
		decision.Missing = []string{"pr_head_stale"}
		decision.NextCommand = "issueops phase --id " + id + " --to pr $RECORD_ACTOR_FLAGS --json"
		return decision, true
	}
	return decision, false
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

// withJSON appends --json to an issueops command that lacks it.
func withJSON(command string) string {
	if strings.HasPrefix(command, "issueops ") && !strings.Contains(command, " --json") {
		return command + " --json"
	}
	return command
}

// actorCWD is the cwd whoami would be run from to get the caller's vectors
// for this cycle, or "" when next is not sure the caller acts on it: the
// source checkout before a lease (the caller is the preparer), the worktree
// while the caller holds the lease itself. Passing the shell variables was
// the step every QA agent failed first (zsh does not split them).
func actorCWD(in Input) string {
	if strings.TrimSpace(in.ActorHost) == "" || strings.TrimSpace(in.ActorSessionID) == "" {
		return ""
	}
	var cwd string
	switch execution := in.Record.Execution; {
	case execution == nil:
		cwd = in.Record.Repo
	case execution.Lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) &&
		strings.TrimSpace(execution.Lease.Holder.AgentID) == "":
		cwd = execution.Workspace.Root
	case releasedMergeResolution(in):
		// sync-base finalize runs in the canonical worktree (cwd_canonical).
		cwd = execution.Workspace.Root
	}
	return strings.TrimSpace(cwd)
}

// recordActorFlags is the record vector (--host --session-id --cwd) whoami
// prints at actorCWD, or "" when that cwd is unknown.
func recordActorFlags(in Input) string {
	cwd := actorCWD(in)
	if cwd == "" {
		return ""
	}
	return "--host " + shellWord(strings.TrimSpace(in.ActorHost)) + " --session-id " + shellWord(strings.TrimSpace(in.ActorSessionID)) + " --cwd " + shellWord(cwd)
}

// shellWord quotes value only when the shell would otherwise split or expand it.
func shellWord(value string) string {
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._/:@+=,-", r)) {
			return shelltoken.QuoteWord(value)
		}
	}
	return value
}

func placeholder(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

// phaseIndex는 phase를 사용자 10단계 번호로 옮긴다. 규칙이 고정 index를 주지
// 않을 때(차단·인수·미분류) 쓴다.
func phaseIndex(phase issueopsnextcontract.Phase) int {
	switch phase {
	case issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill:
		return 1
	case issueopsnextcontract.PhasePlan:
		return 3
	case issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement:
		return 4
	case issueopsnextcontract.PhaseAISlopClean:
		return 5
	case issueopsnextcontract.PhaseFeedback:
		return 7
	case issueopsnextcontract.PhasePR:
		return 9
	case issueopsnextcontract.PhaseDone:
		return 10
	default:
		return 0
	}
}

// planningPhase는 execution이 아직 없어도 정상인 phase다. ai-slop-clean 이후에
// execution이 없으면 그것은 준비 단계가 아니라 이상 상태이므로 unknown이다.
func planningPhase(phase issueopsnextcontract.Phase) bool {
	switch phase {
	case issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill, issueopsnextcontract.PhasePlan,
		issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement:
		return true
	default:
		return false
	}
}

func postImplementPhase(phase issueopsnextcontract.Phase) bool {
	return phase == issueopsnextcontract.PhaseAISlopClean || phase == issueopsnextcontract.PhaseFeedback
}

// holderIsSelf is the caller's own session holding the lease through a live
// process. A resumed session (same host and session id, new process, e.g.
// `claude --resume`) is not the holder: every mutation is refused because the
// recorded receipt is not in its ancestry, so next must route it to the
// takeover chain like any dead holder instead of to holder commands.
func holderIsSelf(in Input) bool {
	if in.Record.Execution == nil || in.Record.Execution.Lease.Holder == nil {
		return false
	}
	holder := in.Record.Execution.Lease.Holder
	return strings.EqualFold(strings.TrimSpace(holder.Host), strings.TrimSpace(in.ActorHost)) &&
		strings.TrimSpace(holder.SessionID) == strings.TrimSpace(in.ActorSessionID) &&
		strings.TrimSpace(in.ActorSessionID) != "" &&
		(in.HolderLive == nil || *in.HolderLive)
}

// live는 관측하지 못한 홀더를 살아 있는 것으로 본다. 확인하지 않은 세션의
// lease를 빼앗으라고 권하지 않기 위해서다.
func live(observed *bool) bool {
	return observed == nil || *observed
}

func leaseView(in Input) issueopsnextcontract.Lease {
	if in.Record.Execution == nil {
		return issueopsnextcontract.Lease{}
	}
	lease := in.Record.Execution.Lease
	return issueopsnextcontract.Lease{
		Status:       string(lease.Status),
		Generation:   lease.Generation,
		HolderIsSelf: holderIsSelf(in),
		HolderLive:   in.HolderLive,
	}
}

func exits(in Input) issueopsnextcontract.Exits {
	id := in.Record.ID
	out := issueopsnextcontract.Exits{
		AbandonCommand: "issueops cleanup abandon --id " + id + " --reason <TEXT> --preview",
	}
	if in.Record.Execution == nil {
		return out
	}
	lease := in.Record.Execution.Lease
	generation := generationText(in)
	if lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) {
		out.PauseCommand = "issueops execution release --id " + id +
			" --generation " + generation + " $ACTOR_FLAGS"
	}
	if (lease.Status == issueopsnextcontract.LeaseStatusActive && lease.Holder != nil && !holderIsSelf(in) && !live(in.HolderLive)) ||
		lease.Status == issueopsnextcontract.LeaseStatusRevoking {
		out.TakeoverCommand = "issueops execution replace --id " + id +
			" --expected-generation " + generation + " --preview"
	}
	return out
}

func warnings(in Input) []string {
	var out []string
	// readiness가 이미 관측한 비차단 경고는 어떤 stage로 분류되든 그대로 전달한다.
	// 특정 분기에 두면 먼저 맞은 규칙이 early return하며 경고가 사라진다.
	if in.Local != nil && len(in.Local.Warnings) > 0 {
		out = append(out, in.Local.Warnings...)
	}
	// QA (killed create-child): next kept naming the plan stage while a
	// child create stayed pending, and create-child refuses until it settles.
	for _, op := range in.Record.ChildCreateOperations {
		if issueCreateUnsettled(&op.IssueOpsIssueCreateIntent) {
			out = append(out, "child create "+op.OperationID+" is unsettled ("+op.Status+"); before any create-child run: issueops remote reconcile-child --id "+in.Record.ID+" --operation-id "+op.OperationID+" $RECORD_ACTOR_FLAGS --json")
		}
	}
	execution := in.Record.Execution
	if execution == nil {
		return out
	}
	// A cleanup attempt removes the worktree on purpose; the restore/takeover
	// advice below would contradict cleanup.resume.
	if root := strings.TrimSpace(execution.Workspace.Root); root != "" && !in.WorktreePresent && in.Record.CleanupAttempt == nil {
		warning := "canonical worktree is missing at " + root
		if execution.Completion == nil {
			// Probed: a worktree recreated with git worktree add lacks the
			// sealed artifacts prepare wrote, so link-plan and the holder
			// commands fail there and nothing in issueops rebuilds them.
			// Abandon needs a lease without a writer: the holder releases it;
			// anyone else (a resumed session too) has no pause command and
			// gets there through the takeover chain up to finalize.
			warning += "; issueops does not rebuild it (the sealed plan artifacts lived there): restore that directory, or end the cycle with exits.abandon_command after "
			if holderIsSelf(in) {
				warning += "releasing the lease (exits.pause_command, --cwd still that path)"
			} else {
				warning += "the takeover chain reaches a released lease (stop after --finalize; do not claim)"
			}
		}
		out = append(out, warning)
	}
	// Probe: after `git checkout -b other` in the worktree, link-plan and the
	// record commands still succeeded and commits would land on the wrong
	// branch, discovered only at publication.
	if branch, want := strings.TrimSpace(in.WorktreeBranch), strings.TrimSpace(execution.Workspace.Branch); in.WorktreePresent && branch != "" && want != "" && branch != want {
		out = append(out, "canonical worktree is on branch "+branch+", not the cycle branch "+want+": switch back with git -C "+strings.TrimSpace(execution.Workspace.Root)+" switch "+want)
	}
	// Probe: a detached HEAD (git checkout <sha>) read as "" and went
	// unwarned; commits made there belong to no branch. A rebase detaches
	// too and has its own warning below.
	if want := strings.TrimSpace(execution.Workspace.Branch); in.WorktreePresent && strings.TrimSpace(in.WorktreeBranch) == "" && strings.TrimSpace(in.WorktreeHead) != "" && want != "" && in.GitOperation != "rebase" {
		out = append(out, "canonical worktree is on a detached HEAD ("+shortHead(in.WorktreeHead)+"), not the cycle branch "+want+": switch back with git -C "+strings.TrimSpace(execution.Workspace.Root)+" switch "+want+" (note the SHAs of any commits made while detached first)")
	}
	switch op := in.GitOperation; op {
	case "rebase", "cherry-pick", "revert":
		// issueops never starts these; one left open (detached HEAD for a
		// rebase) makes every record and seal read a half-applied tree.
		out = append(out, "a git "+op+" is in progress in the canonical worktree: finish it or run git -C "+strings.TrimSpace(execution.Workspace.Root)+" "+op+" --abort before continuing")
	}
	if head := strings.TrimSpace(in.WorktreeHead); head != "" && execution.Completion != nil &&
		strings.TrimSpace(execution.Completion.FinalHead) != "" &&
		strings.TrimSpace(execution.Completion.FinalHead) != head {
		out = append(out, "worktree head "+head+" differs from the recorded completion head "+strings.TrimSpace(execution.Completion.FinalHead))
	}
	return out
}

func takeoverCommand(in Input) string {
	if command := strings.TrimSpace(in.WriterlessRecovery); command != "" {
		return command
	}
	return "issueops execution replace --id " + in.Record.ID +
		" --expected-generation " + generationText(in) + " --preview"
}

func generationText(in Input) string {
	if in.Record.Execution == nil || in.Record.Execution.Lease.Generation == 0 {
		return "<N>"
	}
	return strconv.FormatUint(in.Record.Execution.Lease.Generation, 10)
}

// stageOneMissing은 1단계 게이트만 고정 순서로 남긴다. branch는 2단계 항목이라
// 여기서 빠진다.
func stageOneMissing(in Input) []string {
	if in.Completion == nil {
		return nil
	}
	present := map[string]bool{}
	for _, phase := range []issueopsnextcontract.Phase{issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill} {
		for _, key := range in.Completion(phase).Missing {
			present[key] = true
		}
	}
	var out []string
	for _, key := range stageOneOrder {
		if present[key] {
			out = append(out, key)
		}
	}
	return out
}

// devilsAdvocateGate는 판정 자체를 다시 계산하지 않는다. 연결된 계획의 digest
// 비교는 adapter의 게이트가 하고, 연결 전 스테이징 계획의 비교는 service가
// DevilsAdvocatePlanStale로 넘긴다. 여기서는 그 결과만 읽는다.
func devilsAdvocateGate(in Input) string {
	if in.Completion == nil {
		return ""
	}
	missing := in.Completion(issueopsnextcontract.PhaseImplement).Missing
	for _, key := range []string{"devils_advocate_review", "devils_advocate_review_stale"} {
		if slices.Contains(missing, key) {
			return key
		}
	}
	if in.DevilsAdvocatePlanStale {
		return "devils_advocate_review_stale"
	}
	return ""
}

func devilsAdvocateCommand(in Input) string {
	review := in.Record.DevilsAdvocateReview
	if review != nil && strings.EqualFold(strings.TrimSpace(review.Verdict), "stop") && !review.Waived {
		return "issueops regress --id " + in.Record.ID + " --reason <TEXT>"
	}
	return OwnerCommand(in.Record.ID, "devils_advocate_review")
}

func intersect(list, keys []string) []string {
	var out []string
	for _, key := range keys {
		if slices.Contains(list, key) {
			out = append(out, key)
		}
	}
	return out
}

func prefixed(list []string, prefix string) []string {
	var out []string
	for _, item := range list {
		if strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

func subset(list, allowed []string) bool {
	for _, item := range list {
		if !slices.Contains(allowed, item) {
			return false
		}
	}
	return true
}

func firstOr(list []string, fallback string) string {
	if len(list) > 0 {
		return list[0]
	}
	return fallback
}

// prBase is the branch the cycle's PR targets: the sealed base branch.
func prBase(record issueopsnextcontract.Record) string {
	if record.BranchPrepare == nil {
		return ""
	}
	return record.BranchPrepare.BaseBranch
}

// fillBranchPrepareIssue fills the branch prepare template with the issue the
// record already links (QA: agents copied the URL and guessed the provider
// by hand). Only shell-safe URLs are spliced; the rest stays a placeholder.
func fillBranchPrepareIssue(command, issueURL string) string {
	if !strings.HasPrefix(command, "issueops branch prepare ") || !shellSafeURL(issueURL) {
		return command
	}
	command = strings.Replace(command, "--issue-url <URL>", "--issue-url "+issueURL, 1)
	switch {
	case strings.HasPrefix(issueURL, "https://github.com/"):
		command = strings.Replace(command, "--provider <github|gitlab>", "--provider github", 1)
	case strings.Contains(issueURL, "/-/issues/") || strings.Contains(issueURL, "/-/work_items/"):
		command = strings.Replace(command, "--provider <github|gitlab>", "--provider gitlab", 1)
	}
	return command
}

func shellSafeURL(raw string) bool {
	if !strings.HasPrefix(raw, "https://") {
		return false
	}
	for _, r := range raw {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(":/._-", r)) {
			return false
		}
	}
	return true
}

// classifyCleanupResume routes a record that still holds a cleanup attempt to
// that operation's preview. QA (killed applies): after a killed cleanup
// finish next sent the agent back to merge-pr, and after a killed abandon it
// offered the takeover chain; the preview reads what is left and issues a new
// fingerprint.
func classifyCleanupResume(in Input, decision Decision, index int) (Decision, bool) {
	attempt := in.Record.CleanupAttempt
	if attempt == nil {
		return decision, false
	}
	id := in.Record.ID
	switch attempt.Operation {
	case issueopsnextcontract.CleanupOperationFinish:
		decision.NextCommand = "issueops cleanup finish --id " + id + " --preview --json"
	case issueopsnextcontract.CleanupOperationAbandon:
		decision.NextCommand = "issueops cleanup abandon --id " + id + " --reason <TEXT> --preview --json"
	case issueopsnextcontract.CleanupOperationRemoteBranch:
		decision.NextCommand = "issueops cleanup remote-branch --id " + id + " --preview --json"
	default:
		return decision, false
	}
	decision.Stage = stage(issueopsnextcontract.StageCleanupResume, index)
	decision.Warnings = append(decision.Warnings, "a cleanup "+string(attempt.Operation)+" started at "+attempt.StartedAt+" did not finish (or is still running); its preview reads what is left and issues a new fingerprint")
	return decision, true
}

// classifyMergeInProgress routes a holder whose worktree has an unfinished
// merge to finishing it. Probe: after sync-base apply paused on conflicts,
// next named link-plan, which would have recorded over conflict markers.
func classifyMergeInProgress(in Input, decision Decision) (Decision, bool) {
	execution := in.Record.Execution
	if in.GitOperation != "merge" || execution == nil {
		return decision, false
	}
	prefix := "issueops execution sync-base --id " + in.Record.ID
	switch {
	case execution.Lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in):
		// The work happens in the worktree, as implement's does; no new stage key.
		decision.Stage = stage(issueopsnextcontract.StageImplement, 4)
	case releasedMergeResolution(in):
		// Probe: a merge_conflict recovery on a completed cycle paused on
		// conflicts, and next still offered merge-pr.
		decision.Stage = stage(issueopsnextcontract.StageDone, 10)
		prefix += " --completion-generation " + strconv.FormatUint(execution.Completion.Generation, 10)
	default:
		return decision, false
	}
	decision.Missing = []string{"merge_in_progress"}
	decision.NextCommand = prefix + " --finalize $ACTOR_FLAGS"
	decision.Warnings = append(decision.Warnings, "a merge is in progress in the worktree: resolve the conflict files, then finalize; to drop the merge run "+prefix+" --abort $ACTOR_FLAGS --json")
	return decision, true
}

// releasedMergeResolution is a completed cycle whose worktree holds the
// unfinished merge of a sync-base run under the released completion.
func releasedMergeResolution(in Input) bool {
	execution := in.Record.Execution
	return in.GitOperation == "merge" && execution != nil && execution.Lease.Status == issueopsnextcontract.LeaseStatusReleased &&
		execution.Completion != nil && execution.Completion.Generation != 0
}

func shortHead(oid string) string {
	if oid = strings.TrimSpace(oid); len(oid) > 10 {
		return oid[:10]
	}
	return oid
}
