package statuscli

import (
	"flag"
	"fmt"
	verifyworkapp "issueops/internal/application/verifywork"
)

func RunVerifyWork(service verifyworkapp.Service, args []string) error {
	fs := flag.NewFlagSet("verify-work", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	all := fs.Bool("all", false, "guard all relevant files instead of staged files")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
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
		return fmt.Errorf("verify-work found incomplete evidence")
	}
	return nil
}
