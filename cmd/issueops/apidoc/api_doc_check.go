package apidoc

import (
	"flag"
	app "issueops/internal/application/apidoc"
)

func (c Command) runAPIDocCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	diffFile := fs.String("diff-file", "", "read diff from file instead of git diff --cached")
	promptFile := fs.String("prompt-file", "", "append project-specific review instructions from file")
	resultFile := fs.String("result", "", "read host-agent JSON review result from file")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := c.ResolveTarget(*repo)
	result, err := c.Service.Check(app.StaticOptions{Repo: root, Files: fs.Args(), All: *all, JSON: true}, app.ReviewOptions{Repo: root, Files: fs.Args(), All: *all, DiffFile: *diffFile, PromptFile: *promptFile, ResultFile: *resultFile, JSON: true})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result.Static)
	if result.Static.OK {
		printAPIDocReview(result.Review)
	}
	return err
}
