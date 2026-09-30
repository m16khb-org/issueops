package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
)

func (command Command) runRemoteReflectCompletion(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reflect-completion", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	bodyFile := fs.String("body-file", "", "progress report markdown file written for human readers; required with --confirm")
	confirm := fs.Bool("confirm", false, "write to the remote issue; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	var resultBody string
	if strings.TrimSpace(*bodyFile) != "" {
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
		resultBody = strings.TrimSpace(string(data))
	}
	_, result, report, err := command.Operations.ReflectRemoteCompletion(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *providerOverride, resultBody, *confirm, deps.VerifyMerged)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(reflectCompletionResponse{IssueProviderUpdateIssueBodySectionResult: result, Readability: report})
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
