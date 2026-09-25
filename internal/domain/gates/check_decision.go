package gates

import (
	"fmt"
	"strings"

	"issueops/internal/domain/shelltoken"
)

func CheckArgv(check string) ([]string, string) {
	if shelltoken.HasUnquotedControlOperator(check) {
		return nil, "CHECK contains shell syntax that argv execution does not honor (&&, ||, |, ;, 2>&1): wrap the sequence in one script or python3 -c"
	}
	argv := shelltoken.SplitCommandTokens(check)
	if len(argv) == 0 {
		return nil, "empty CHECK command"
	}
	return argv, ""
}

func ShouldRun(gate Gate) bool {
	return !gate.Checked || EvidencePending(gate.Evidence)
}

type CheckDecision struct {
	Passed   bool
	Evidence string
	Error    string
}

func DecideCheck(expect, stdout, stderr string, exitCode int, timedOut bool) CheckDecision {
	output := strings.TrimRight(stdout, "\n")
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimRight(stderr, "\n")
	}
	evidence := EvidenceTail(output, 200)
	decision := CheckDecision{Evidence: evidence}
	switch {
	case timedOut:
		decision.Error = "check timed out: " + evidence
	case exitCode != 0:
		decision.Error = fmt.Sprintf("exit code %d: %s", exitCode, evidence)
	case strings.TrimSpace(expect) != "" && !ExpectMatches(expect, output):
		decision.Error = "expect not matched: " + evidence
	default:
		decision.Passed = true
	}
	return decision
}
