package apidoc

import (
	"errors"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
	"issueops/internal/adapter/preflight"
	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
	"os"
)

func IsStaticGateError(err error) bool { return errors.Is(err, app.ErrStaticGateFailed) }
func testCommand() Command {
	return Command{Service: testAPIDocService(), ResolveTarget: func(target string) string {
		if target != "" {
			return target
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return cwd
	}}
}
func runAPIDoc(args []string) error            { return testCommand().Run(args) }
func runAPIDocReview(args []string) error      { return testCommand().runAPIDocReview(args) }
func runAPIDocStaticCheck(args []string) error { return testCommand().runAPIDocStaticCheck(args) }
func runAPIDocCheck(args []string) error       { return testCommand().runAPIDocCheck(args) }
func runAPIDocReviewWithOptions(o app.ReviewOptions) (contract.ReviewResult, error) {
	return testAPIDocService().Reviewer.Review(o)
}
func runAPIDocStaticCheckWithOptions(o app.StaticOptions) (contract.StaticResult, error) {
	return testAPIDocService().Static.Check(o)
}

func Diff(repo string, files []string, file string) (string, error) {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Diff(repo, files, file)
}
func Input(repo string, files []string, file string, all bool) (string, error) {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Input(repo, files, file, all)
}
func TrackedFiles(repo string) []string {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Tracked(repo)
}
