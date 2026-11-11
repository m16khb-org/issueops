package issueopsnext

import (
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

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
		// QA: earlier wordings read as "maybe already merged" before the merge.
		// merge-pr records the merge, so only a merge outside issueops lands here.
		decision.Warnings = append(decision.Warnings, "next does not read the PR: if it was merged outside issueops, this preview answers already_merged:true and records it")
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

// prBase is the branch the cycle's PR targets: the sealed base branch.
func prBase(record issueopsnextcontract.Record) string {
	if record.BranchPrepare == nil {
		return ""
	}
	return record.BranchPrepare.BaseBranch
}
