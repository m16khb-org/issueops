package statuscli

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	verifyworkapp "issueops/internal/application/verifywork"
)

func RunVerifyWork(service verifyworkapp.Service, args []string) error {
	fs := flag.NewFlagSet("verify-work", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	all := fs.Bool("all", false, "guard all relevant files instead of staged files")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	result := service.Run(*repo, *all, fs.Args())
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		fmt.Printf("verify-work ok=%v repo=%s\n", result.OK, result.Repo)
		for _, item := range result.EvidenceMatrix {
			fmt.Printf("- %s: %s (%s)\n", item.Name, item.Status, item.Summary)
		}
		for _, warning := range result.Warnings {
			fmt.Printf("warning: %s\n", warning)
		}
	}
	if !result.OK {
		incomplete := fmt.Errorf("verify-work found incomplete evidence")
		if *jsonOut {
			// The printed evidence matrix is the verdict.
			return jsonout.Reported(incomplete)
		}
		return incomplete
	}
	return nil
}
