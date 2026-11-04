package issueopsnext

// stage 기대값 오라클: lease·pending·cleanup·merge 우선순위와 phase 규칙을
// 분류기 구현과 따로 표로 적어 둔다.

import (
	"fmt"
	"slices"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ---------------------------------------------------------------- 상태 사실

// matrixFacts re-derives, independently of the classifier's helpers, the lease
// relations the routing depends on.
type matrixFacts struct {
	hasExec    bool
	status     issueopscontract.LeaseStatus
	pending    bool
	completion *issueopscontract.ExecutionCompletion
	self       bool
	otherLive  bool
	writerless bool
}

func deriveFacts(in Input) matrixFacts {
	e := in.Record.Execution
	if e == nil {
		return matrixFacts{}
	}
	holder := e.Lease.Holder
	live := in.HolderLive == nil || *in.HolderLive
	sameActor := holder != nil && strings.EqualFold(holder.Host, in.ActorHost) && holder.SessionID == in.ActorSessionID && in.ActorSessionID != ""
	self := sameActor && live
	other := holder != nil && !self
	return matrixFacts{
		hasExec: true, status: e.Lease.Status, pending: e.Pending != nil, completion: e.Completion,
		self: self, otherLive: other && live,
		writerless: (e.Lease.Status == issueopsnextcontract.LeaseStatusActive && other && !live) ||
			e.Lease.Status == issueopsnextcontract.LeaseStatusRevoking,
	}
}

func (f matrixFacts) selfActive() bool {
	return f.hasExec && f.status == issueopsnextcontract.LeaseStatusActive && f.self
}

func validCleanup(attempt *issueopscontract.IssueOpsCleanupAttempt) bool {
	return attempt != nil && (attempt.Operation == issueopsnextcontract.CleanupOperationFinish ||
		attempt.Operation == issueopsnextcontract.CleanupOperationAbandon ||
		attempt.Operation == issueopsnextcontract.CleanupOperationRemoteBranch)
}

func (k *matrixChecker) checkLease(c *matrixCase) {
	in, got := c.input, c.got
	if strings.TrimSpace(in.Record.ID) == "" {
		return
	}
	facts := deriveFacts(in)
	wantStatus := ""
	if facts.hasExec {
		wantStatus = string(in.Record.Execution.Lease.Status)
	}
	if got.Lease.Status != wantStatus || got.Lease.HolderIsSelf != facts.self {
		k.violations.add("lease view mirrors the record", k.m, c, fmt.Sprintf("view %+v facts %+v", got.Lease, facts))
	}
	if (got.Exits.PauseCommand != "") != facts.selfActive() {
		k.violations.add("only the live self holder can pause", k.m, c, fmt.Sprintf("pause=%q", got.Exits.PauseCommand))
	}
	if (got.Exits.TakeoverCommand != "") != (facts.hasExec && facts.writerless) {
		k.violations.add("takeover exit exists exactly for a writerless lease", k.m, c, fmt.Sprintf("takeover=%q", got.Exits.TakeoverCommand))
	}
}

// ---------------------------------------------------------------- 라우팅 오라클

// expectedStages names the stages the lease/cleanup/merge/pending precedence
// allows, or nil when the later phase rules decide. The order mirrors the
// rule list documented on Classify.
func expectedStages(in Input) []string {
	facts := deriveFacts(in)
	if validCleanup(in.Record.CleanupAttempt) {
		return []string{issueopsnextcontract.StageCleanupResume}
	}
	if mergeResolution(in, facts) {
		if facts.selfActive() {
			return []string{issueopsnextcontract.StageImplement}
		}
		return []string{issueopsnextcontract.StageDone}
	}
	if stage := expectedPendingStage(facts); stage != "" {
		return []string{stage}
	}
	if !facts.hasExec {
		if rootConflictBlocks(in) {
			return []string{issueopsnextcontract.StageBlockedRoot}
		}
		return nil
	}
	if stage := expectedLeaseStage(facts); stage != "" {
		return []string{stage}
	}
	return nil
}

// mergeResolution: an unfinished merge in the worktree of the live holder, or
// under a released completion that carries a generation.
func mergeResolution(in Input, facts matrixFacts) bool {
	if in.GitOperation != "merge" || !facts.hasExec {
		return false
	}
	return facts.selfActive() || (facts.status == issueopsnextcontract.LeaseStatusReleased && facts.completion != nil && facts.completion.Generation != 0)
}

func expectedPendingStage(facts matrixFacts) string {
	if !facts.hasExec || !facts.pending {
		return ""
	}
	switch {
	case facts.writerless:
		return issueopsnextcontract.StageTakeover
	case facts.status != issueopsnextcontract.LeaseStatusClaimable:
		return issueopsnextcontract.StageBlockedPending
	}
	return ""
}

func rootConflictBlocks(in Input) bool {
	return phaseOrder(in.Record.Phase) >= phaseOrder(issueopsnextcontract.PhasePlan) && strings.TrimSpace(in.RootConflictID) != ""
}

func expectedLeaseStage(facts matrixFacts) string {
	switch {
	case facts.status == issueopsnextcontract.LeaseStatusActive && facts.otherLive:
		return issueopsnextcontract.StageBlockedHolderLive
	case facts.writerless:
		return issueopsnextcontract.StageTakeover
	case (facts.status == issueopsnextcontract.LeaseStatusClaimable || facts.status == issueopsnextcontract.LeaseStatusReleased) && facts.completion == nil:
		return issueopsnextcontract.StageClaim
	}
	return ""
}

func (k *matrixChecker) checkRouting(c *matrixCase) {
	in, got := c.input, c.got
	if strings.TrimSpace(in.Record.ID) == "" {
		if got.Stage.Key != issueopsnextcontract.StageNone || got.Exits.AbandonCommand != "" {
			k.violations.add("an empty record routes to none", k.m, c, got.Stage.Key)
		}
		return
	}
	k.checkAlwaysPresentRules(c)
	if k.m.malformed(c.sel) {
		return
	}
	k.checkMergeInProgress(c)
	if want := expectedStages(in); want != nil {
		if !slices.Contains(want, got.Stage.Key) {
			k.violations.add("lease/pending/cleanup precedence routes as documented", k.m, c, fmt.Sprintf("got %s want %v", got.Stage.Key, want))
		}
		k.checkLeaseStageCommand(c)
		return
	}
	k.checkPhaseRouting(c)
	k.checkReadOnlyStage(c)
}

// checkAlwaysPresentRules are the properties that hold even for malformed
// records: no phase ever routes back, and a live other holder is never given
// holder commands.
func (k *matrixChecker) checkAlwaysPresentRules(c *matrixCase) {
	in, got := c.input, c.got
	if phaseOrder(in.Record.Phase) >= phaseOrder(issueopsnextcontract.PhaseAISlopClean) && in.GitOperation != "merge" {
		switch got.Stage.Key {
		case issueopsnextcontract.StageIssue, issueopsnextcontract.StagePrepare, issueopsnextcontract.StagePlanWrite,
			issueopsnextcontract.StagePlanArchitecture, issueopsnextcontract.StagePlanReview, issueopsnextcontract.StagePlanHandoff,
			issueopsnextcontract.StageImplementEnter, issueopsnextcontract.StageImplement:
			k.violations.add("a phase after implement never routes back to planning stages", k.m, c, fmt.Sprintf("phase %s -> %s", in.Record.Phase, got.Stage.Key))
		}
	}
	if in.Record.Phase == issueopsnextcontract.PhaseDone && in.GitOperation != "merge" {
		switch got.Stage.Key {
		case issueopsnextcontract.StageClean, issueopsnextcontract.StageDocs, issueopsnextcontract.StageVerify,
			issueopsnextcontract.StageCommitPush, issueopsnextcontract.StagePRCreate, issueopsnextcontract.StagePRComplete:
			k.violations.add("done never routes back into the cycle", k.m, c, got.Stage.Key)
		}
	}
	facts := deriveFacts(in)
	if facts.hasExec && facts.status == issueopsnextcontract.LeaseStatusActive && facts.otherLive && !k.m.malformed(c.sel) {
		if got.Exits.PauseCommand != "" {
			k.violations.add("a live other holder gets no holder commands", k.m, c, got.Exits.PauseCommand)
		}
	}
}

// checkMergeInProgress: an unfinished merge in the worktree of the live holder
// (or of a released completion) routes to finishing it, never past it.
func (k *matrixChecker) checkMergeInProgress(c *matrixCase) {
	in, got := c.input, c.got
	facts := deriveFacts(in)
	if !mergeResolution(in, facts) || validCleanup(in.Record.CleanupAttempt) {
		return
	}
	command := plainCommand(got.NextCommand)
	if !strings.HasPrefix(command, "issueops execution sync-base --id "+matrixID) || !strings.Contains(command, " --finalize") ||
		!slices.Equal(got.Missing, []string{"merge_in_progress"}) {
		k.violations.add("an unfinished merge routes to sync-base finalize", k.m, c, fmt.Sprintf("%s %q missing=%v", got.Stage.Key, got.NextCommand, got.Missing))
	}
	if !facts.selfActive() && !strings.Contains(command, " --completion-generation ") {
		k.violations.add("a released merge resolution names the completion generation", k.m, c, got.NextCommand)
	}
}

// checkLeaseStageCommand pins the command each lease-driven stage hands out.
func (k *matrixChecker) checkLeaseStageCommand(c *matrixCase) {
	got, in := c.got, c.input
	command := plainCommand(got.NextCommand)
	switch got.Stage.Key {
	case issueopsnextcontract.StageBlockedPending:
		k.requirePrefix(c, command, "issueops execution reconcile --id "+matrixID+" --preview")
	case issueopsnextcontract.StageBlockedRoot:
		k.requirePrefix(c, command, "issueops list --repo")
	case issueopsnextcontract.StageBlockedHolderLive:
		k.requirePrefix(c, command, "issueops execution status --id "+matrixID)
	case issueopsnextcontract.StageCleanupResume:
		if !strings.Contains(command, " --preview") || !strings.HasPrefix(command, "issueops cleanup ") {
			k.violations.add("cleanup.resume only previews", k.m, c, got.NextCommand)
		}
	case issueopsnextcontract.StageTakeover:
		if in.WriterlessRecovery != "" {
			if command != strings.TrimSuffix(in.WriterlessRecovery, " --json") {
				k.violations.add("takeover uses the injected recovery command", k.m, c, got.NextCommand)
			}
		} else {
			k.requirePrefix(c, command, "issueops execution replace --id "+matrixID+" --expected-generation")
			if !strings.HasSuffix(command, " --preview") {
				k.violations.add("takeover starts with a preview", k.m, c, got.NextCommand)
			}
		}
	case issueopsnextcontract.StageClaim:
		k.checkClaimCommand(c, command)
	}
}

func (k *matrixChecker) checkClaimCommand(c *matrixCase, command string) {
	status := c.input.Record.Execution.Lease.Status
	switch status {
	case issueopsnextcontract.LeaseStatusClaimable:
		k.requirePrefix(c, command, "issueops execution claim --id "+matrixID+" --generation")
		if !strings.Contains(command, "--claim-current-token") {
			k.violations.add("claim names the claim token source", k.m, c, command)
		}
	default:
		if !strings.HasPrefix(command, "issueops execution replace --id "+matrixID+" --expected-generation") || !strings.HasSuffix(command, " --preview") {
			k.violations.add("a released lease starts the chain with a replace preview", k.m, c, command)
		}
	}
}

// plainCommand drops the --json suffix and the quotes the replace-chain
// commands put around the (shell-safe) record id.
func plainCommand(command string) string {
	return strings.ReplaceAll(strings.TrimSuffix(command, " --json"), "'"+matrixID+"'", matrixID)
}

func (k *matrixChecker) requirePrefix(c *matrixCase, command, prefix string) {
	if !strings.HasPrefix(command, prefix) {
		k.violations.add(c.got.Stage.Key+" hands out its documented command", k.m, c, fmt.Sprintf("%q does not start with %q", c.got.NextCommand, prefix))
	}
}

// checkPhaseRouting covers the cases the lease precedence leaves to the phase
// rules: for those the stage is fully determined by the state, so it is
// compared exactly.
func (k *matrixChecker) checkPhaseRouting(c *matrixCase) {
	in, got := c.input, c.got
	want := exactLateStage(in)
	if got.Stage.Key != want {
		k.violations.add("phase routing for this state is as documented", k.m, c, fmt.Sprintf("phase %s got %s want %s (%s)", in.Record.Phase, got.Stage.Key, want, got.NextCommand))
		return
	}
	k.checkLateStageCommand(c)
	if got.Stage.Key == issueopsnextcontract.StageIssue {
		k.checkIssueStage(c)
	}
}

func (k *matrixChecker) checkIssueStage(c *matrixCase) {
	in, got := c.input, c.got
	unsettled := intentIsUnsettled(in.Record.IssueCreateIntent)
	reconcile := strings.HasPrefix(got.NextCommand, "issueops remote reconcile-issue --id "+matrixID)
	if unsettled != reconcile || (unsettled && (len(got.Missing) == 0 || got.Missing[0] != "issue_create_unsettled")) {
		k.violations.add("an unsettled create-issue is reconciled, never retried", k.m, c, got.NextCommand)
	}
}

// matrix-side spec tables for the readiness keys, written out literally so a
// change to the classifier's key lists shows up here.
var (
	specCleanCompletionKeys = []string{"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	specCleanLocalKeys      = []string{"ai_slop_clean_stale", "ai_slop_clean_fingerprint", "current_fingerprint"}
	specDocsLocalKeys       = []string{"project_docs_review", "project_docs_review_stale"}
	specVerifyLocalKeys     = []string{
		"schema_evidence", "schema_evidence_stale", "implementation_review", "implementation_review_stale",
		"feedback_classification", "feedback_resolution", "contract_feedback_issue_update",
	}
	specCommitPushKeys = []string{"worktree_clean", "upstream"}
)

func anyIn(list, keys []string) bool {
	return slices.ContainsFunc(list, func(item string) bool { return slices.Contains(keys, item) })
}

// exactLateStage is the stage the phase rules give once no lease, pending,
// cleanup or merge rule applied.
func exactLateStage(in Input) string {
	switch in.Record.Phase {
	case issueopsnextcontract.PhaseDone:
		return issueopsnextcontract.StageDone
	case issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill, issueopsnextcontract.PhasePlan,
		issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement:
		return exactPlanningStage(in)
	case issueopsnextcontract.PhaseAISlopClean, issueopsnextcontract.PhaseFeedback:
		return exactPostImplementStage(in)
	case issueopsnextcontract.PhasePR:
		return exactPublicationStage(in)
	}
	return issueopsnextcontract.StageUnknown
}

func devilsAdvocateBlocks(in Input) bool {
	missing := in.Completion(issueopsnextcontract.PhaseImplement).Missing
	return slices.Contains(missing, "devils_advocate_review") || slices.Contains(missing, "devils_advocate_review_stale") || in.DevilsAdvocatePlanStale
}

func architectureBlocks(in Input) bool {
	review := in.Record.ArchitectureReview
	if review != nil && review.Approved && review.ReviewerContext == issueopsdomain.ParentReviewPattern {
		return false
	}
	return !in.StagedArchitecture || review == nil || !review.Approved || in.ArchitectureReviewStale
}

func exactPlanningStage(in Input) string {
	phase := in.Record.Phase
	facts := deriveFacts(in)
	switch {
	case !facts.hasExec:
		return exactPreparationStage(in)
	case !facts.selfActive():
		return issueopsnextcontract.StageUnknown
	case phase == issueopsnextcontract.PhaseImplement:
		return issueopsnextcontract.StageImplement
	case phase != issueopsnextcontract.PhasePlan && phase != issueopsnextcontract.PhaseCompatibilityReview:
		return issueopsnextcontract.StageUnknown
	case branchStillNeeded(in):
		return issueopsnextcontract.StagePrepare
	case strings.TrimSpace(in.Record.PlanPath) == "":
		return issueopsnextcontract.StageImplementEnter
	case devilsAdvocateBlocks(in):
		return issueopsnextcontract.StagePlanReview
	}
	return issueopsnextcontract.StageImplementEnter
}

// exactPreparationStage is stages one to three: no execution yet.
func exactPreparationStage(in Input) string {
	phase := in.Record.Phase
	switch {
	case strings.TrimSpace(in.Record.IssueURL) == "":
		if phase == issueopsnextcontract.PhaseProblem || phase == issueopsnextcontract.PhaseGrill {
			return issueopsnextcontract.StageIssue
		}
		return issueopsnextcontract.StageUnknown
	case branchStillNeeded(in):
		return issueopsnextcontract.StagePrepare
	case architectureBlocks(in):
		return issueopsnextcontract.StagePlanArchitecture
	case !in.StagedPlan:
		return issueopsnextcontract.StagePlanWrite
	case devilsAdvocateBlocks(in):
		return issueopsnextcontract.StagePlanReview
	}
	return issueopsnextcontract.StagePlanHandoff
}

// branchStillNeeded restates "provider branch link not verified yet".
func branchStillNeeded(in Input) bool {
	record := in.Record
	return record.Phase == issueopsnextcontract.PhaseProblem || record.Phase == issueopsnextcontract.PhaseGrill ||
		record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" || !record.BranchPrepare.LinkVerified
}

// exactLocalGateStage maps the local readiness gaps onto stages five to eight.
func exactLocalGateStage(missing []string) string {
	switch {
	case anyIn(missing, specCleanLocalKeys):
		return issueopsnextcontract.StageClean
	case anyIn(missing, specDocsLocalKeys):
		return issueopsnextcontract.StageDocs
	case anyIn(missing, specVerifyLocalKeys) || slices.ContainsFunc(missing, func(key string) bool { return strings.HasPrefix(key, "gates_incomplete:") }):
		return issueopsnextcontract.StageVerify
	case !slices.ContainsFunc(missing, func(key string) bool { return !slices.Contains(specCommitPushKeys, key) }):
		return issueopsnextcontract.StageCommitPush
	}
	return issueopsnextcontract.StageUnknown
}

func exactPostImplementStage(in Input) string {
	if in.Record.Execution == nil {
		return issueopsnextcontract.StageUnknown
	}
	if in.Record.Phase == issueopsnextcontract.PhaseAISlopClean &&
		anyIn(in.Completion(issueopsnextcontract.PhaseAISlopClean).Missing, specCleanCompletionKeys) {
		return issueopsnextcontract.StageClean
	}
	if in.Local == nil {
		return issueopsnextcontract.StageUnknown
	}
	return exactLocalGateStage(in.Local.Missing)
}

func exactPublicationStage(in Input) string {
	if in.Record.Execution == nil {
		return issueopsnextcontract.StageUnknown
	}
	if in.Record.RemoteArtifact != nil {
		if in.Record.Execution.Completion == nil {
			return issueopsnextcontract.StagePRComplete
		}
		return issueopsnextcontract.StageUnknown
	}
	if in.Local != nil && len(in.Local.Missing) > 0 {
		if anyIn(in.Local.Missing, specCleanLocalKeys) {
			return issueopsnextcontract.StageClean
		}
		if stage := exactLocalGateStage(in.Local.Missing); stage != issueopsnextcontract.StageUnknown {
			return stage
		}
		return issueopsnextcontract.StagePRCreate
	}
	prHead, head := strings.TrimSpace(in.Record.PRHead), strings.TrimSpace(in.WorktreeHead)
	if in.Local != nil && prHead != "" && head != "" && prHead != head {
		return issueopsnextcontract.StageCommitPush
	}
	return issueopsnextcontract.StagePRCreate
}

// checkLateStageCommand pins the command each phase-driven stage hands out.
func (k *matrixChecker) checkLateStageCommand(c *matrixCase) {
	in, got := c.input, c.got
	command := plainCommand(got.NextCommand)
	wantPrefixes := lateStageCommands(in, got.Stage.Key)
	if wantPrefixes == nil {
		return
	}
	if !slices.ContainsFunc(wantPrefixes, func(prefix string) bool { return strings.HasPrefix(command+" ", prefix) }) {
		k.violations.add(got.Stage.Key+" hands out its documented command", k.m, c, fmt.Sprintf("%q does not start with any of %q", got.NextCommand, wantPrefixes))
	}
}

// lateStageCommands lists the command prefixes a phase-driven stage may hand
// out for this input; nil for stages this check does not pin.
func lateStageCommands(in Input, stage string) []string {
	id := "--id " + matrixID
	switch stage {
	case issueopsnextcontract.StagePrepare:
		return prepareCommands(in)
	case issueopsnextcontract.StagePlanHandoff:
		return []string{"issueops execution prepare " + id + " "}
	case issueopsnextcontract.StagePlanReview:
		if review := in.Record.DevilsAdvocateReview; review != nil && strings.EqualFold(review.Verdict, "stop") && !review.Waived {
			return []string{"issueops regress " + id + " "}
		}
		return []string{"issueops devils-advocate review " + id + " "}
	case issueopsnextcontract.StageImplement:
		if len(in.ChildGateMissing) > 0 {
			return []string{"issueops child status --parent " + matrixID + " "}
		}
		return []string{"issueops phase " + id + " --to ai-slop-clean "}
	}
	return publicationStageCommands(in, stage)
}

func publicationStageCommands(in Input, stage string) []string {
	id := "--id " + matrixID
	switch stage {
	case issueopsnextcontract.StagePRCreate:
		return []string{"issueops remote create-pr " + id + " --expected-generation "}
	case issueopsnextcontract.StagePRComplete:
		return []string{"issueops execution complete " + id + " --generation "}
	case issueopsnextcontract.StageCommitPush:
		return []string{"issueops pr-readiness " + id + " --strict", "issueops phase " + id + " --to pr "}
	case issueopsnextcontract.StageDone:
		return []string{doneCommand(in.Record.RemoteCompletion)}
	}
	return nil
}

// doneCommand is the read-only cleanup step the merge receipts select.
func doneCommand(completion *issueopscontract.IssueOpsRemoteCompletion) string {
	switch {
	case completion != nil && (completion.ReflectedAt != "" || completion.IssueClosedAt != ""):
		return "issueops cleanup finish --id " + matrixID + " --preview"
	case completion != nil && completion.MergedAt != "":
		return "issueops cleanup status --id " + matrixID + " --merged"
	}
	return "issueops remote merge-pr --id " + matrixID
}

// prepareCommands is the command stage two hands out: advance the phase when
// only the phase is behind, otherwise the branch evidence that is missing.
func prepareCommands(in Input) []string {
	record := in.Record
	prepared := record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.BaseSHA) != ""
	switch {
	case record.Phase == issueopsnextcontract.PhaseProblem:
		return []string{"issueops phase --id " + matrixID + " --to grill "}
	case prepared && record.BranchPrepare.LinkVerified:
		return []string{"issueops phase --id " + matrixID + " --to plan "}
	}
	return []string{"issueops branch prepare --id " + matrixID + " ", "issueops branch await-link --id " + matrixID + " "}
}

// checkReadOnlyStage asserts that stages which must not change state hand out
// only read or preview commands. Terminal done is the main one.
func (k *matrixChecker) checkReadOnlyStage(c *matrixCase) {
	got := c.got
	command := plainCommand(got.NextCommand)
	var allowed []string
	switch got.Stage.Key {
	case issueopsnextcontract.StageDone:
		allowed = []string{
			"issueops remote merge-pr --id " + matrixID,
			"issueops cleanup finish --id " + matrixID + " --preview",
			"issueops cleanup status --id " + matrixID + " --merged",
		}
	case issueopsnextcontract.StageUnknown:
		allowed = []string{"issueops status --id " + matrixID}
	default:
		return
	}
	if !slices.ContainsFunc(allowed, func(prefix string) bool { return strings.HasPrefix(command, prefix) }) {
		k.violations.add(got.Stage.Key+" is read-only", k.m, c, fmt.Sprintf("%q not in %v", got.NextCommand, allowed))
	}
	for _, mutating := range []string{" --confirm", " --apply", " --finalize", " --abort", " --revoke", " --reseed", " --delete"} {
		if strings.Contains(command, mutating) {
			k.violations.add(got.Stage.Key+" never hands out a mutating flag", k.m, c, got.NextCommand)
		}
	}
	if got.Stage.Key == issueopsnextcontract.StageDone && got.Stage.Index != 10 {
		k.violations.add("done is stage ten", k.m, c, fmt.Sprint(got.Stage.Index))
	}
}
