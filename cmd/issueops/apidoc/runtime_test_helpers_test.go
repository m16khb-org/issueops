package apidoc

import (
	"errors"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
	"issueops/internal/adapter/preflight"
	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
	domain "issueops/internal/domain/apidoc"
	"os"
)

type apiDocReviewOptions = app.ReviewOptions
type apiDocStaticOptions = app.StaticOptions
type apiDocReviewResult = contract.ReviewResult
type apiDocStaticResult = contract.StaticResult
type apiDocCheckResult = contract.CheckResult
type apiDocReviewFinding = contract.ReviewFinding

var ErrReviewGateFailed = app.ErrReviewGateFailed
var ErrReviewResultRequired = app.ErrReviewResultRequired
var ErrStaticGateFailed = app.ErrStaticGateFailed

func IsReviewGateError(err error) bool {
	return errors.Is(err, app.ErrReviewGateFailed) || errors.Is(err, app.ErrReviewResultRequired)
}
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
func runAPIDocReviewWithOptions(o apiDocReviewOptions) (apiDocReviewResult, error) {
	return testAPIDocService().Reviewer.Review(o)
}
func runAPIDocStaticCheckWithOptions(o apiDocStaticOptions) (apiDocStaticResult, error) {
	return testAPIDocService().Static.Check(o)
}
func Evidence(repo string, files []string) string         { return reviewfiles.Evidence(repo, files) }
func ReviewExtraPrompt(repo, file string) (string, error) { return reviewfiles.ExtraPrompt(repo, file) }
func Diff(repo string, files []string, file string) (string, error) {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Diff(repo, files, file)
}
func Input(repo string, files []string, file string, all bool) (string, error) {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Input(repo, files, file, all)
}
func StagedFiles(repo string) []string {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Staged(repo)
}
func TrackedFiles(repo string) []string {
	return (reviewfiles.Files{GitCmd: preflight.GitCmd}).Tracked(repo)
}
func NormalizeFiles(repo string, files []string) []string { return reviewfiles.Normalize(repo, files) }
func IsCandidate(file string) bool                        { return domain.IsCandidate(file) }
