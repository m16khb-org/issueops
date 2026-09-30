package issueops

import (
	"issueops/internal/contract/issueops"
	"strconv"
	"strings"
)

func ReplacementClaimCommand(id string, generation uint64, _ string) string {
	return "issueops execution claim --id " + quoteReplacementArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-current-token"
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
	process := actor.SessionProcess
	command := "issueops execution replace --id " + quoteReplacementArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) +
		" --revoke --reason <TEXT> --inventory-fingerprint " + fingerprint +
		" --host " + quoteReplacementArg(actor.Host) +
		" --session-id " + quoteReplacementArg(actor.SessionID)
	if actor.AgentID != "" {
		command += " --agent-id " + quoteReplacementArg(actor.AgentID)
	}
	if process != nil {
		command += " --session-pid " + strconv.Itoa(process.PID) +
			" --session-started-at " + quoteReplacementArg(process.StartedAt) +
			" --session-executable " + quoteReplacementArg(process.Executable)
	}
	command += " --cwd " + quoteReplacementArg(cwd) + " --confirm"
	return command
}

func ReplacementResumeCommand(id string, generation uint64) string {
	return "issueops execution resume --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --confirm"
}
func quoteReplacementArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
