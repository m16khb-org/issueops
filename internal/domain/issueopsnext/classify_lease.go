package issueopsnext

import (
	"strconv"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsdomain "issueops/internal/domain/issueops"
)

// classifyBlockedExecution은 pending execution과 이미 점유된 canonical root처럼
// lease를 보기 전에 사이클을 멈추는 두 차단 규칙이다.
func classifyBlockedExecution(in Input, decision Decision, index int) (Decision, bool) {
	execution := in.Record.Execution
	if execution != nil && execution.Pending != nil {
		// Reconcile needs the live lease holder. When the holder died inside
		// the write (QA: a killed create-pr), take the lease over first; the
		// new holder reconciles the pending intent it inherits.
		lease := execution.Lease
		if writerlessLease(in) {
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
	if execution == nil && issueopsdomain.PhaseRank(in.Record.Phase) >= issueopsdomain.PhaseRank(issueopsnextcontract.PhasePlan) &&
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
	switch {
	case lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsOther(in) && live(in.HolderLive):
		decision.Stage = stage(issueopsnextcontract.StageBlockedHolderLive, index)
		decision.NextCommand = "issueops execution status --id " + id + " --json"
		return decision, true
	case writerlessLease(in):
		decision.Stage = stage(issueopsnextcontract.StageTakeover, index)
		decision.NextCommand = takeoverCommand(in)
		return decision, true
	case (lease.Status == issueopsnextcontract.LeaseStatusClaimable || lease.Status == issueopsnextcontract.LeaseStatusReleased) &&
		execution.Completion == nil:
		decision.Stage = stage(issueopsnextcontract.StageClaim, index)
		decision.NextCommand = claimCommand(in)
		return decision, true
	}
	return decision, false
}

// holderIsOther는 lease에 자기가 아닌 holder가 기록돼 있는 상태다. 호출자는
// execution이 있음을 보장한다.
func holderIsOther(in Input) bool {
	return in.Record.Execution.Lease.Holder != nil && !holderIsSelf(in)
}

// writerlessLease는 쓰기 주체가 없어 takeover 체인으로 가야 하는 lease다:
// 죽은 다른 holder가 쥔 active이거나 revoking. 호출자는 execution이 있음을 보장한다.
func writerlessLease(in Input) bool {
	status := in.Record.Execution.Lease.Status
	return (status == issueopsnextcontract.LeaseStatusActive && holderIsOther(in) && !live(in.HolderLive)) ||
		status == issueopsnextcontract.LeaseStatusRevoking
}

// claimCommand는 claim 단계의 첫 명령이다. 호출자는 execution이 있음을 보장한다.
func claimCommand(in Input) string {
	execution := in.Record.Execution
	lease := execution.Lease
	id := in.Record.ID
	// The chain's first command, not a status read that only returns it.
	if lease.Status != issueopsnextcontract.LeaseStatusClaimable {
		return issueopsdomain.ReplacementPreviewCommand(id, lease.Generation)
	}
	command := issueopsdomain.ReplacementClaimCommand(id, lease.Generation)
	// The actor-free claim works only from the canonical worktree, and
	// QA agents distrusted an "exact" claim without flags. With an
	// observable receipt, pin this session's vector and the worktree.
	if root := strings.TrimSpace(execution.Workspace.Root); root != "" && in.ClaimActorFlags != nil {
		if flags := in.ClaimActorFlags(root); flags != "" {
			command += " " + flags
		}
	}
	return command
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
