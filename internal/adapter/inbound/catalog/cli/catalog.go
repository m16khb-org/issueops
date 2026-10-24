package cli

import (
	"strings"

	contract "issueops/internal/contract/cli"
	clidomain "issueops/internal/domain/cli"
)

func Commands() []contract.Command {
	return append(contract.RootCommands(), contract.LifecycleCommands()...)
}
func IssueOpsUsageLines() []string { return strings.Split(contract.IssueOpsUsageCatalog, "\n") }

func LifecycleUsage() string {
	return "Usage:\n" +
		strings.Join(IssueOpsUsageLines(), "\n") + "\n\n" +
		contract.IssueOpsActorFlagLegend + "\n"
}

func ChildUsage() string {
	var lines []string
	for _, line := range IssueOpsUsageLines() {
		switch clidomain.IssueOpsUsageKey(line) {
		case "child start", "child status", "child list",
			"child accept", "child reject", "child drop":
			lines = append(lines, line)
		}
	}
	return "Usage:\n" +
		strings.Join(lines, "\n") + "\n\n" +
		contract.IssueOpsActorFlagLegend
}

// RemoteUsage is `issueops remote --help`: every `remote` line of the
// catalog. The group used to keep its own copy, which drifted (create-pr
// lacked --body-file, --provider and --score-file that next_command uses).
func RemoteUsage() string {
	var lines []string
	for _, line := range IssueOpsUsageLines() {
		if strings.HasPrefix(clidomain.IssueOpsUsageKey(line), "remote ") {
			lines = append(lines, line)
		}
	}
	return "Usage:\n" +
		strings.Join(lines, "\n") + "\n\n" +
		contract.IssueOpsActorFlagLegend
}
