package issueopsnext

import (
	"slices"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsdomain "issueops/internal/domain/issueops"
)

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
		decision.NextCommand = ownerCommand(id, key)
		return decision, true
	}
	if !in.StagedPlan {
		decision.Stage = stage(issueopsnextcontract.StagePlanWrite, 3)
		decision.Missing = []string{"plan_artifact"}
		decision.NextCommand = ownerCommand(id, "plan_artifact")
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
	decision.NextCommand = ownerCommand(record.ID, firstOr(decision.Missing, "intent_contract"))
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
	decision.NextCommand = ownerCommand(record.ID, key)
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
	return ownerCommand(in.Record.ID, "devils_advocate_review")
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
