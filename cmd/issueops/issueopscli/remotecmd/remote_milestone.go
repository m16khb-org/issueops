package remotecmd

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteMilestone(args []string, deps Deps) error {
	return runPreviewCommand(command, args, deps, previewSpec[issueopscontract.RemoteMilestoneResult, string]{
		subcommand:   "milestone",
		onUsage:      "milestone target: issue or pr (a GitLab MR is pr)",
		confirmUsage: "set the milestone; without this, preview only",
		bindValue:    stringValueFlag("set", "title of an existing milestone"),
		operation:    command.Operations.MilestoneRemote,
		view: func(result *issueopscontract.RemoteMilestoneResult) previewView {
			return valuePreviewView(result.TargetURL, &result.NextCommand, result.Set, result.OK, "milestone of %s: %s", result.ReadBack, fmt.Sprintf("would set milestone %q on %s; re-run with --confirm", result.Milestone, result.TargetURL))
		},
	})
}
