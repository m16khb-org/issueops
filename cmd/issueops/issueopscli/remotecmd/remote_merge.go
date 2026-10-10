package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteMergePR(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote merge-pr", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id of a completed cycle")
	method := fs.String("method", issueopscontract.RemoteMergeMethodSquash, "merge method: squash, merge, or rebase")
	confirm := fs.Bool("confirm", false, "mark a draft ready and merge; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.MergeRemotePullRequest(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *method, *confirm)
	if result.URL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	// A refused or unverified merge still prints the observation: the
	// blockers are what the caller acts on.
	if *jsonOut {
		if printErr := deps.printJSON(result); printErr != nil {
			return printErr
		}
		return err
	}
	printRemoteMergeResult(result)
	return err
}

func printRemoteMergeResult(result issueopscontract.RemoteMergeResult) {
	fmt.Printf("%s %s state=%s draft=%t checks=%s head=%s expected=%s\n", result.Provider, result.URL, result.State, result.Draft, result.Checks, result.HeadOID, result.ExpectedHeadOID)
	for _, blocker := range result.Blockers {
		fmt.Printf("blocked: %s: %s\n", blocker.Code, blocker.Message)
	}
	switch {
	case result.AlreadyMerged:
		fmt.Println("already merged")
	case result.Merged:
		fmt.Printf("merged (%s) %s\n", result.Method, strings.TrimSpace(result.MergeCommitOID))
	case result.OK:
		fmt.Printf("ready to merge (%s); re-run with --confirm\n", result.Method)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
}
