package remotecmd

import (
	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteLabel(args []string, deps Deps) error {
	return runPreviewCommand(command, args, deps, previewSpec[issueopscontract.RemoteLabelResult, []string]{
		subcommand:   "label",
		onUsage:      "label target: issue or pr (a GitLab MR is pr)",
		confirmUsage: "add the labels; without this, preview only",
		bindValue:    repeatedValueFlag("add", "label to add (repeatable; existing labels are kept)"),
		operation:    command.Operations.LabelRemote,
		view: func(result *issueopscontract.RemoteLabelResult) previewView {
			return listPreviewView(result.TargetURL, &result.NextCommand, result.Added, result.OK, "labels on %s: %s", "would add %s to %s; re-run with --confirm", result.Labels, result.Add)
		},
	})
}
