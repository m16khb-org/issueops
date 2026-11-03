package remotecmd

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteRetitle(args []string, deps Deps) error {
	return runPreviewCommand(command, args, deps, previewSpec[issueopscontract.RemoteRetitleResult, string]{
		subcommand:   "retitle",
		onUsage:      "title target: issue or pr (a GitLab MR is pr)",
		confirmUsage: "change the title; without this, preview only",
		bindValue:    stringValueFlag("title", "new title (one line)"),
		operation:    command.Operations.RetitleRemote,
		view: func(result *issueopscontract.RemoteRetitleResult) previewView {
			return valuePreviewView(result.TargetURL, &result.NextCommand, result.Retitled, result.OK, "retitled %s: %s", result.ReadBack, fmt.Sprintf("would retitle %s to %q; re-run with --confirm", result.TargetURL, result.Title))
		},
	})
}
