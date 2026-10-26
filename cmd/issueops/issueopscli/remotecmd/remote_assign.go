package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"issueops/cmd/issueops/jsonout"
)

func (command Command) runRemoteAssign(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote assign", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", "assignee target: issue or pr (a GitLab MR is pr)")
	var add repeatedFlag
	fs.Var(&add, "add", "username to assign (repeatable; existing assignees are kept)")
	confirm := fs.Bool("confirm", false, "add the assignees; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.AssignRemote(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, add, *confirm)
	if result.TargetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		result.NextCommand = confirmCommand("assign", args)
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
	case result.Assigned:
		fmt.Printf("assignees on %s: %s\n", result.TargetURL, strings.Join(result.Assignees, ", "))
	case result.OK:
		fmt.Printf("would assign %s on %s; re-run with --confirm\n", strings.Join(result.Add, ", "), result.TargetURL)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
	return err
}
