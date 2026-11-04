// Package cli is the inbound adapter that renders the CLI command catalog and its
// usage text from the shared contract catalog.
package cli

import (
	"strings"

	contract "issueops/internal/contract/cli"
	clidomain "issueops/internal/domain/cli"
)

// Commands returns the root commands followed by the lifecycle commands.
func Commands() []contract.Command {
	return append(contract.RootCommands(), contract.LifecycleCommands()...)
}

// IssueOpsUsageLines returns the issueops usage catalog split into one entry per
// line.
func IssueOpsUsageLines() []string { return strings.Split(contract.IssueOpsUsageCatalog, "\n") }

// LifecycleUsage returns the usage text for the lifecycle commands: every catalog
// line followed by the actor flag legend.
func LifecycleUsage() string {
	return "Usage:\n" +
		strings.Join(IssueOpsUsageLines(), "\n") + "\n\n" +
		contract.IssueOpsActorFlagLegend + "\n"
}

// ChildUsage returns the usage text for the `child` subcommands (start, status,
// list, accept, reject, drop) followed by the actor flag legend.
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
