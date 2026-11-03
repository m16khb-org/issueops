package remotecmd

import (
	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteAssign(args []string, deps Deps) error {
	return runPreviewCommand(command, args, deps, previewSpec[issueopscontract.RemoteAssignResult, []string]{
		subcommand:   "assign",
		onUsage:      "assignee target: issue or pr (a GitLab MR is pr)",
		confirmUsage: "add the assignees; without this, preview only",
		bindValue:    repeatedValueFlag("add", "username to assign (repeatable; existing assignees are kept)"),
		operation:    command.Operations.AssignRemote,
		view: func(result *issueopscontract.RemoteAssignResult) previewView {
			return listPreviewView(result.TargetURL, &result.NextCommand, result.Assigned, result.OK, "assignees on %s: %s", "would assign %s on %s; re-run with --confirm", result.Assignees, result.Add)
		},
	})
}
