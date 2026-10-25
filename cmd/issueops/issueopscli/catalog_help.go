package issueopscli

import (
	"strings"

	clicontract "issueops/internal/contract/cli"
	cliparse "issueops/internal/domain/cli"
)

// catalogHelp is a command's --help text taken from the usage catalog, so the
// help cannot drift from it. QA: hand-written help lines had lost
// RECORD_ACTOR_FLAGS (feedback resolve, plan-prep record, and twelve more)
// while the commands require them. The actor legend follows when a line uses
// an actor abbreviation.
func catalogHelp(paths ...string) string {
	var lines []string
	for _, path := range paths {
		for line := range strings.SplitSeq(clicontract.IssueOpsUsageCatalog, "\n") {
			if trimmed := strings.TrimSpace(line); cliparse.IssueOpsUsageKey(trimmed) == path {
				lines = append(lines, trimmed)
			}
		}
	}
	if len(lines) == 0 {
		return "Usage: issueops " + strings.Join(paths, ", ") + "\n"
	}
	help := "Usage: " + strings.Join(lines, "\n       ") + "\n"
	if strings.Contains(help, "ACTOR_FLAGS") {
		legend, _, _ := strings.Cut(clicontract.IssueOpsActorFlagLegend, "\n\n")
		help += legend + "\n"
	}
	return help
}
