package issueopsnext

import (
	"strconv"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

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
