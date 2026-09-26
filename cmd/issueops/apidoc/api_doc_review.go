package apidoc

import (
	"flag"
	"fmt"
	"os"

	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
)

type apiDocReviewFinding = contract.ReviewFinding
type apiDocReviewResult = contract.ReviewResult
type apiDocReviewOptions = app.ReviewOptions

func runAPIDoc(args []string) error {
	if len(args) == 0 {
		apiDocUsage()
		return fmt.Errorf("missing api-doc subcommand")
	}
	switch args[0] {
	case "check":
		return runAPIDocCheck(args[1:])
	case "review":
		return runAPIDocReview(args[1:])
	case "static-check":
		return runAPIDocStaticCheck(args[1:])
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

func runAPIDocReview(args []string) error {
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
	root := ResolveTarget(*repo)
	options := apiDocReviewOptions{Repo: root, Files: fs.Args(), All: *all, DiffFile: *diffFile, PromptFile: *promptFile, ResultFile: *resultFile, JSON: *jsonOut}
	result, err := runAPIDocReviewWithOptions(options)
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocReview(result)
	return err
}

func runAPIDocReviewWithOptions(options apiDocReviewOptions) (apiDocReviewResult, error) {
	return (app.ReviewService{Effects: app.ReviewEffects{
		NormalizeFiles: normalizeAPIDocFiles,
		TrackedFiles:   trackedAPIDocFiles,
		StagedFiles:    stagedAPIDocFiles,
		Input:          apiDocInput,
		ExtraPrompt:    apiDocReviewExtraPrompt,
		Evidence:       apiDocReviewEvidence,
		BuildPrompt:    buildAPIDocReviewPrompt,
		Schema:         apiDocReviewSchema,
		ReadResult: func(repo, file string) (string, []byte, error) {
			path := resolveAPIDocReviewResultPath(repo, file)
			data, err := os.ReadFile(path)
			return path, data, err
		},
	}}).Review(options)
}
