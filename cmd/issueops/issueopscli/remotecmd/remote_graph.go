package remotecmd

import (
	"context"
	"flag"
	"fmt"
)

func runIssueGraphSync(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote sync-graph", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "execute sync; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := remoteDeps.SyncIssueGraph(context.Background(), remoteDeps.IssueOpsStateRoot(), *id, *confirm)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if !*confirm {
		if result["link_count"] == 0 {
			fmt.Println("no issue graph links to sync")
		} else {
			fmt.Println(result["message"])
		}
		return nil
	}
	fmt.Printf("synced: %v links\n", result["link_count"])
	return nil
}
