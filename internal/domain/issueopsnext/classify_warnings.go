package issueopsnext

import (
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

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
	if writerlessLease(in) {
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
	out = append(out, childCreateWarnings(in)...)
	if in.Record.Execution == nil {
		return out
	}
	for _, rule := range worktreeWarningRules {
		if warning := rule(in); warning != "" {
			out = append(out, warning)
		}
	}
	return out
}

// childCreateWarnings는 아직 정착하지 않은 child create 작업마다 경고 하나를 낸다.
// QA (killed create-child): next kept naming the plan stage while a
// child create stayed pending, and create-child refuses until it settles.
func childCreateWarnings(in Input) []string {
	var out []string
	for _, op := range in.Record.ChildCreateOperations {
		if issueCreateUnsettled(&op.IssueCreateIntent) {
			out = append(out, "child create "+op.OperationID+" is unsettled ("+op.Status+"); before any create-child run: issueops remote reconcile-child --id "+in.Record.ID+" --operation-id "+op.OperationID+" $RECORD_ACTOR_FLAGS --json")
		}
	}
	return out
}

// worktreeWarningRules는 execution이 있는 사이클의 worktree 경고 규칙이며, 경고
// 순서는 이 목록 순서와 같다. 각 규칙은 해당 없으면 빈 문자열을 돌려준다.
var worktreeWarningRules = []func(Input) string{
	worktreeMissingWarning,
	worktreeBranchWarning,
	worktreeDetachedWarning,
	gitOperationWarning,
	completionHeadWarning,
}

// worktreeMissingWarning은 canonical worktree가 사라졌을 때의 경고다.
// A cleanup attempt removes the worktree on purpose; the restore/takeover
// advice below would contradict cleanup.resume.
func worktreeMissingWarning(in Input) string {
	execution := in.Record.Execution
	root := strings.TrimSpace(execution.Workspace.Root)
	if root == "" || in.WorktreePresent || in.Record.CleanupAttempt != nil {
		return ""
	}
	warning := "canonical worktree is missing at " + root
	if execution.Completion != nil {
		return warning
	}
	// Probed: a worktree recreated with git worktree add lacks the
	// sealed artifacts prepare wrote, so link-plan and the holder
	// commands fail there and nothing in issueops rebuilds them.
	// Abandon needs a lease without a writer: the holder releases it;
	// anyone else (a resumed session too) has no pause command and
	// gets there through the takeover chain up to finalize.
	warning += "; issueops does not rebuild it (the sealed plan artifacts lived there): restore that directory, or end the cycle with exits.abandon_command after "
	if holderIsSelf(in) {
		return warning + "releasing the lease (exits.pause_command, --cwd still that path)"
	}
	return warning + "the takeover chain reaches a released lease (stop after --finalize; do not claim)"
}

// worktreeBranchWarning은 worktree가 사이클 branch가 아닌 다른 branch에 있을 때의 경고다.
// Probe: after `git checkout -b other` in the worktree, link-plan and the
// record commands still succeeded and commits would land on the wrong
// branch, discovered only at publication.
func worktreeBranchWarning(in Input) string {
	workspace := in.Record.Execution.Workspace
	branch, want := strings.TrimSpace(in.WorktreeBranch), strings.TrimSpace(workspace.Branch)
	if !in.WorktreePresent || branch == "" || want == "" || branch == want {
		return ""
	}
	return "canonical worktree is on branch " + branch + ", not the cycle branch " + want + ": switch back with git -C " + strings.TrimSpace(workspace.Root) + " switch " + want
}

// worktreeDetachedWarning은 worktree가 detached HEAD일 때의 경고다.
// Probe: a detached HEAD (git checkout <sha>) read as "" and went
// unwarned; commits made there belong to no branch. A rebase detaches
// too and has its own warning below.
func worktreeDetachedWarning(in Input) string {
	workspace := in.Record.Execution.Workspace
	want := strings.TrimSpace(workspace.Branch)
	if !in.WorktreePresent || strings.TrimSpace(in.WorktreeBranch) != "" || strings.TrimSpace(in.WorktreeHead) == "" || want == "" || in.GitOperation == "rebase" {
		return ""
	}
	return "canonical worktree is on a detached HEAD (" + shortHead(in.WorktreeHead) + "), not the cycle branch " + want + ": switch back with git -C " + strings.TrimSpace(workspace.Root) + " switch " + want + " (note the SHAs of any commits made while detached first)"
}

// gitOperationWarning은 worktree에 열려 있는 git 작업이 있을 때의 경고다.
// issueops never starts these; one left open (detached HEAD for a
// rebase) makes every record and seal read a half-applied tree.
func gitOperationWarning(in Input) string {
	switch op := in.GitOperation; op {
	case "rebase", "cherry-pick", "revert":
		return "a git " + op + " is in progress in the canonical worktree: finish it or run git -C " + strings.TrimSpace(in.Record.Execution.Workspace.Root) + " " + op + " --abort before continuing"
	}
	return ""
}

// completionHeadWarning은 worktree head가 기록된 completion head와 다를 때의 경고다.
func completionHeadWarning(in Input) string {
	completion := in.Record.Execution.Completion
	head := strings.TrimSpace(in.WorktreeHead)
	if head == "" || completion == nil {
		return ""
	}
	final := strings.TrimSpace(completion.FinalHead)
	if final == "" || final == head {
		return ""
	}
	return "worktree head " + head + " differs from the recorded completion head " + final
}
