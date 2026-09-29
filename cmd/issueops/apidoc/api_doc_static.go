package apidoc

import (
	"flag"

	app "issueops/internal/application/apidoc"
)

func (c Command) runAPIDocStaticCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc static-check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := c.Service.Static.Check(app.StaticOptions{Repo: c.ResolveTarget(*repo), Files: fs.Args(), All: *all, JSON: *jsonOut})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result)
	return err
}
