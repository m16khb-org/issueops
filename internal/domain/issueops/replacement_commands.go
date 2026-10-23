package issueops

import (
	"issueops/internal/contract/issueops"
	"strconv"
	"strings"
)

// ReplacementClaimCommand is the actor-free claim for a reader that does not
// know the claiming actor (status, next). Without ACTOR_FLAGS claim observes
// the session itself, including its cwd, so it runs from the canonical
// worktree.
func ReplacementClaimCommand(id string, generation uint64) string {
	return "issueops execution claim --id " + quoteReplacementArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-current-token"
}

// ReplacementClaimCommandAs is the exact claim for the actor that just
// finalized or reseeded the lease: its ACTOR_FLAGS with the canonical
// worktree as --cwd, so it runs from wherever that actor is.
func ReplacementClaimCommandAs(id string, generation uint64, actor issueops.NativeActor, root string) string {
	return ReplacementClaimCommand(id, generation) + replacementActorFlags(actor, root)
}

func ReplacementReseedCommand(id string, generation, completionGeneration uint64, fingerprint string, actor issueops.NativeActor, cwd string) string {
	process := actor.SessionProcess
	command := "issueops execution replace --id " + quoteReplacementArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10)
	if completionGeneration != 0 {
		command += " --completion-generation " + strconv.FormatUint(completionGeneration, 10)
	}
	command += " --reseed --inventory-fingerprint " + fingerprint +
		" --host " + quoteReplacementArg(actor.Host) +
		" --session-id " + quoteReplacementArg(actor.SessionID)
	if actor.AgentID != "" {
		command += " --agent-id " + quoteReplacementArg(actor.AgentID)
	}
	command += " --session-pid " + strconv.Itoa(process.PID) +
		" --session-started-at " + quoteReplacementArg(process.StartedAt) +
		" --session-executable " + quoteReplacementArg(process.Executable) +
		" --cwd " + quoteReplacementArg(cwd) + " --confirm"
	return command
}

// ReplacementRevokeCommand는 인수 체인의 revoke 단계를 렌더한다. reseed와 달리
// `--reason`은 사람이 채워야 하므로 자리표시자로 남는다 — 그래서 이 명령은
// 그대로 실행하는 exact가 아니라 값을 채우는 template이다.
func ReplacementRevokeCommand(id string, generation uint64, fingerprint string, actor issueops.NativeActor, cwd string) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) +
		" --revoke --reason <TEXT> --inventory-fingerprint " + fingerprint +
		replacementActorFlags(actor, cwd) + " --confirm"
}

// ReplacementFinalizeCommand는 finalize-preview가 관측한 quiescence fingerprint로
// 인수 체인의 finalize 단계를 렌더한다. 채울 값이 없으므로 exact다.
func ReplacementFinalizeCommand(id string, generation uint64, quiescenceFingerprint string, actor issueops.NativeActor, cwd string) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) +
		" --finalize --quiescence-fingerprint " + quiescenceFingerprint +
		replacementActorFlags(actor, cwd) + " --confirm"
}

func replacementActorFlags(actor issueops.NativeActor, cwd string) string {
	flags := " --host " + quoteReplacementArg(actor.Host) +
		" --session-id " + quoteReplacementArg(actor.SessionID)
	if actor.AgentID != "" {
		flags += " --agent-id " + quoteReplacementArg(actor.AgentID)
	}
	if process := actor.SessionProcess; process != nil {
		flags += " --session-pid " + strconv.Itoa(process.PID) +
			" --session-started-at " + quoteReplacementArg(process.StartedAt) +
			" --session-executable " + quoteReplacementArg(process.Executable)
	}
	return flags + " --cwd " + quoteReplacementArg(cwd)
}

func quoteReplacementArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
