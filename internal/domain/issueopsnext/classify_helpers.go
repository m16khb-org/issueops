package issueopsnext

import (
	"slices"
	"strings"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	"issueops/internal/domain/shelltoken"
)

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
