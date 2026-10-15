package commandparse

import "issueops/internal/domain/shelltoken"

// parseExactForTest tokenizes a command line the way a shell would and parses
// the words after the executable as argv.
func parseExactForTest(command string) (ExactIssueOpsCommand, bool) {
	tokens := shelltoken.SplitCommandTokens(command)
	if len(tokens) == 0 {
		return ExactIssueOpsCommand{}, false
	}
	return ParseExactIssueOpsArgs(tokens[1:])
}
