package remotecmd

import (
	"context"
	"flag"
	"fmt"
)

func (command Command) runRemoteReflectCompletion(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reflect-completion", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	confirm := fs.Bool("confirm", false, "write to the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	_, result, err := command.Operations.ReflectRemoteCompletion(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *providerOverride, *confirm, deps.VerifyMerged)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.Updated {
		fmt.Printf("reflected completion section: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func (command Command) runRemoteCloseIssue(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote close-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	confirm := fs.Bool("confirm", false, "close the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	_, result, err := command.Operations.CloseRemoteIssue(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *providerOverride, *confirm, deps.VerifyMerged)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.Closed {
		fmt.Printf("closed issue: %s\n", result.IssueURL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}
