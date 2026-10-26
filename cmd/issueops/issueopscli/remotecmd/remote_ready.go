package remotecmd

import (
	"context"
	"flag"
	"fmt"

	"issueops/cmd/issueops/jsonout"
	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteMarkReady(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote mark-ready", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id of a completed cycle")
	confirm := fs.Bool("confirm", false, "clear the draft state; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.MarkRemotePullRequestReady(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *confirm)
	if result.URL == "" {
		return deps.printErrorResult(*jsonOut, err)
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
	printRemoteReadyResult(result)
	return err
}

func printRemoteReadyResult(result issueopscontract.RemoteReadyResult) {
	fmt.Printf("%s %s state=%s draft=%t head=%s expected=%s\n", result.Provider, result.URL, result.State, result.Draft, result.HeadOID, result.ExpectedHeadOID)
	switch {
	case result.AlreadyReady:
		fmt.Println("already ready for review")
	case result.MarkedReady:
		fmt.Println("marked ready for review")
	case result.OK:
		fmt.Println("draft; re-run with --confirm to mark it ready for review")
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
}
