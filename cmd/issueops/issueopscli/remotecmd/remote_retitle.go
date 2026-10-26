package remotecmd

import (
	"context"
	"flag"
	"fmt"

	"issueops/cmd/issueops/jsonout"
)

func (command Command) runRemoteRetitle(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote retitle", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", "title target: issue or pr (a GitLab MR is pr)")
	title := fs.String("title", "", "new title (one line)")
	confirm := fs.Bool("confirm", false, "change the title; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.RetitleRemote(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, *title, *confirm)
	if result.TargetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		result.NextCommand = confirmCommand("retitle", args)
	}
	if *jsonOut {
		if err != nil {
			if printErr := deps.printJSON(jsonout.WithError(result, err)); printErr != nil {
				return printErr
			}
			return jsonout.Reported(err)
		}
		return deps.printJSON(result)
	}
	switch {
	case result.Retitled:
		fmt.Printf("retitled %s: %s\n", result.TargetURL, result.ReadBack)
	case result.OK:
		fmt.Printf("would retitle %s to %q; re-run with --confirm\n", result.TargetURL, result.Title)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
	return err
}
