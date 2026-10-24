package gates

import (
	"fmt"
	"regexp"
	"strings"

	"issueops/internal/domain/shelltoken"
)

func CheckArgv(check string) ([]string, string) {
	if shelltoken.HasUnquotedControlOperator(check) {
		return nil, "CHECK contains shell syntax that argv execution does not honor (&&, ||, |, ;, 2>&1): put the sequence in a script file and run that file directly, or use python3 -c (sh -c and bash -c are refused)"
	}
	if shelltoken.HasShellExpansion(check) {
		return nil, "CHECK contains a shell expansion ($(...), ${...}, $NAME) that argv execution passes through literally: run the command directly, or put it in a script file and run that file directly"
	}
	argv := shelltoken.SplitCommandTokens(check)
	if len(argv) == 0 {
		return nil, "empty CHECK command"
	}
	if name, _, ok := strings.Cut(argv[0], "="); ok && envNameRe.MatchString(name) {
		// Without a shell, NAME=value is not an assignment but the program name.
		return nil, fmt.Sprintf("CHECK starts with the assignment %s, which argv execution runs as a program name, not an environment variable: write env %s", argv[0], strings.Join(argv, " "))
	}
	return argv, ""
}

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
		if trimmed := strings.TrimSpace(expect); !strings.HasPrefix(trimmed, "/") && literalLooksLikeRegex(trimmed) {
			decision.Error += " (EXPECT is matched as a literal line; write /" + trimmed + "/ for a regex)"
		} else if line, ok := lineStartingWith(output, trimmed); ok && !strings.HasPrefix(trimmed, "/") {
			// QA: `EXPECT: PASS` against "PASS: ..." read as a substring
			// match; a prefix only counts when a blank follows it.
			decision.Error += fmt.Sprintf(" (the line %q starts with EXPECT but a prefix matches only when a space or tab follows; write the whole line or /^%s/)", line, regexp.QuoteMeta(trimmed))
		} else if line, ok := lineContaining(output, trimmed); ok && trimmed != "" && !strings.HasPrefix(trimmed, "/") {
			// QA: `EXPECT: mod` against grep -n's "5:Operations: ... mod"
			// was read as a substring match.
			decision.Error += fmt.Sprintf(" (the line %q contains EXPECT mid-line, but a literal EXPECT matches a whole line or a line prefix followed by a space; write /%s/ to match anywhere)", line, regexp.QuoteMeta(trimmed))
		}
	default:
		decision.Passed = true
	}
	return decision
}

func lineStartingWith(output, prefix string) (string, bool) {
	for line := range strings.SplitSeq(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, prefix) {
			return line, true
		}
	}
	return "", false
}

func lineContaining(output, text string) (string, bool) {
	for line := range strings.SplitSeq(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); strings.Contains(line, text) {
			return line, true
		}
	}
	return "", false
}
