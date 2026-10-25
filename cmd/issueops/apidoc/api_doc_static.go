package apidoc

import (
	"flag"

	"issueops/cmd/issueops/cliflag"
	app "issueops/internal/application/apidoc"
)

func (c Command) runAPIDocStaticCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc static-check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	jsonOut := fs.Bool("json", false, "print JSON")
	files, err := cliflag.ParseInterspersed(fs, args)
	if err != nil {
		return err
	}
	result, err := c.Service.Static.Check(app.StaticOptions{Repo: c.ResolveTarget(*repo), Files: files, All: *all, JSON: *jsonOut})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result)
	return err
}
