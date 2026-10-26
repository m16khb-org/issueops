package remotecmd

import (
	"context"
	"flag"
	"fmt"

	"issueops/cmd/issueops/jsonout"
)

func (command Command) runRemoteMilestone(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote milestone", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", "milestone target: issue or pr (a GitLab MR is pr)")
	set := fs.String("set", "", "title of an existing milestone")
	confirm := fs.Bool("confirm", false, "set the milestone; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.MilestoneRemote(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, *set, *confirm)
	if result.TargetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		result.NextCommand = confirmCommand("milestone", args)
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
	case result.Set:
		fmt.Printf("milestone of %s: %s\n", result.TargetURL, result.ReadBack)
	case result.OK:
		fmt.Printf("would set milestone %q on %s; re-run with --confirm\n", result.Milestone, result.TargetURL)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
	return err
}
