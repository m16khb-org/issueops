package remotecmd

import (
	"context"
	"flag"
	"fmt"
)

func runRemoteReconcileIssue(ctx context.Context, args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reconcile-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "adopt the unique live verified issue")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := remoteDeps.ReconcileIssueCreate(ctx, remoteDeps.IssueOpsStateRoot(), *id, *confirm, deps.verifyLive)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.WouldAdopt {
		fmt.Printf("would adopt: %s\n", result.CandidateURL)
	} else {
		fmt.Printf("adopted: %s\n", result.CandidateURL)
	}
	return nil
}
