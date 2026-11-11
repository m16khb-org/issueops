package issueopsnext

import (
	"slices"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// classifyImplementation은 홀더가 이 세션인 4~8단계를 가른다.
func classifyImplementation(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Execution != nil && record.Execution.Lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) {
		if next, ok := classifyHolderImplement(in, decision); ok {
			return next, true
		}
	}
	// ai-slop-clean 이후 phase는 lease 홀더가 진행한다. execution이 없으면
	// 이상 상태이므로 clean·verify 명령 대신 unknown(status 안내)으로 보낸다.
	if record.Execution == nil {
		return decision, false
	}
	if record.Phase == issueopsnextcontract.PhaseAISlopClean {
		if missing := intersect(in.Completion(issueopsnextcontract.PhaseAISlopClean).Missing, cleanCompletionKeys); len(missing) > 0 {
			decision.Stage = stage(issueopsnextcontract.StageClean, 5)
			decision.Missing = missing
			decision.NextCommand = ownerCommand(id, missing[0])
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
			decision.NextCommand = ownerCommand(id, key)
		case strings.TrimSpace(record.PlanPath) == "":
			decision.Missing = []string{"plan_path"}
			decision.NextCommand = linkPlanCommand(record)
			if devilsAdvocateGate(in) != "" {
				decision.Warnings = append(decision.Warnings, revisePlanAfterPrepareWarning)
			}
		case devilsAdvocateGate(in) != "":
			// QA (Codex): a revise verdict recorded after execution prepare
			// still routed to compatibility review and phase --to implement,
			// which refuses; the agent found no way to change the plan.
			key := devilsAdvocateGate(in)
			decision.Stage = stage(issueopsnextcontract.StagePlanReview, 3)
			decision.Missing = []string{key}
			decision.NextCommand = devilsAdvocateCommand(in)
			decision.Warnings = append(decision.Warnings, revisePlanAfterPrepareWarning)
		case record.CompatibilityReview == nil || !record.CompatibilityReview.Approved:
			decision.Missing = []string{"compatibility_review"}
			decision.NextCommand = ownerCommand(id, "compatibility_review")
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
		decision.NextCommand = ownerCommand(id, missing[0])
		return decision, true
	}
	if missing := intersect(local, docsLocalKeys); len(missing) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageDocs, 6)
		decision.Missing = missing
		decision.NextCommand = ownerCommand(id, missing[0])
		return decision, true
	}
	verify := intersect(local, verifyLocalKeys)
	verify = append(verify, prefixed(local, "gates_incomplete:")...)
	if len(verify) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageVerify, 7)
		decision.Missing = verify
		decision.NextCommand = ownerCommand(id, verify[0])
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

// revisePlanAfterPrepareWarning names where a plan is revised once
// execution prepare has run: artifact stage refuses then, and the linked plan
// is edited in place until the cycle enters implement.
const revisePlanAfterPrepareWarning = "the plan review is not passed: after execution prepare, revise the linked plan file (plan_path, link-plan first) in place, then record a fresh devil's-advocate review; artifact stage no longer applies"
