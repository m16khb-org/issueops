package projectcli

import (
	"flag"
	"fmt"
	app "issueops/internal/application/commitsuggest"
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	"os"
)

func runProjectCommitSuggest(service app.Service, args []string) error {
	fs := flag.NewFlagSet("project commit-suggest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	staged := fs.Bool("staged", false, "suggest commit based on staged changes (git diff --cached)")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := service.Suggest(commitsuggestcontract.CommitSuggestRequest{
		RepoRoot: *repo,
		Staged:   *staged,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}

	if !result.Executed {
		fmt.Fprintln(os.Stderr, "No changes detected. Nothing to suggest.")
		return nil
	}

	fmt.Println(result.Prompt)
	return nil
}
