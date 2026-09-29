package apidoc

import (
	"flag"
	"fmt"
	"os"

	app "issueops/internal/application/apidoc"
)

func (c Command) runAPIDoc(args []string) error {
	if len(args) == 0 {
		apiDocUsage()
		return fmt.Errorf("missing api-doc subcommand")
	}
	switch args[0] {
	case "check":
		return c.runAPIDocCheck(args[1:])
	case "review":
		return c.runAPIDocReview(args[1:])
	case "static-check":
		return c.runAPIDocStaticCheck(args[1:])
	default:
		apiDocUsage()
		return fmt.Errorf("unknown api-doc subcommand %q", args[0])
	}
}

func apiDocUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops api-doc review [--repo PATH] [--all] [--diff-file FILE] [--prompt-file FILE] [--result FILE] [--json] [--] [FILES...]
  issueops api-doc static-check [--repo PATH] [--all] [--json] [--] [FILES...]
  issueops api-doc check [--repo PATH] [--all] [--diff-file FILE] [--prompt-file FILE] [--result FILE] [--json] [--] [FILES...]
`)
}

func (c Command) runAPIDocReview(args []string) error {
	fs := flag.NewFlagSet("api-doc review", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "review all tracked API documentation candidate files instead of staged changes")
	diffFile := fs.String("diff-file", "", "read diff from file instead of git diff --cached")
	promptFile := fs.String("prompt-file", "", "append project-specific review instructions from file")
	resultFile := fs.String("result", "", "read host-agent JSON review result from file")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := c.ResolveTarget(*repo)
	options := app.ReviewOptions{Repo: root, Files: fs.Args(), All: *all, DiffFile: *diffFile, PromptFile: *promptFile, ResultFile: *resultFile, JSON: *jsonOut}
	result, err := c.Service.Reviewer.Review(options)
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocReview(result)
	return err
}
