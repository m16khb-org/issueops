package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"issueops/cmd/issueops/jsonout"
)

func (command Command) runRemoteLabel(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote label", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", "label target: issue or pr (a GitLab MR is pr)")
	var add repeatedFlag
	fs.Var(&add, "add", "label to add (repeatable; existing labels are kept)")
	confirm := fs.Bool("confirm", false, "add the labels; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.LabelRemote(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, add, *confirm)
	if result.TargetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		result.NextCommand = confirmCommand("label", args)
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
	case result.Added:
		fmt.Printf("labels on %s: %s\n", result.TargetURL, strings.Join(result.Labels, ", "))
	case result.OK:
		fmt.Printf("would add %s to %s; re-run with --confirm\n", strings.Join(result.Add, ", "), result.TargetURL)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
	return err
}
