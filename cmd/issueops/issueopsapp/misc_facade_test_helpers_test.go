package issueopsapp

import (
	"errors"

	"issueops/cmd/issueops/apidoc"
	"issueops/cmd/issueops/pathutil"
	statecontract "issueops/internal/contract/state"
)

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func findUp(start, marker string) (string, bool) {
	return pathutil.FindUp(start, marker)
}

func splitLines(s string) []string {
	return pathutil.SplitLines(s)
}

func splitCSV(s string) []string {
	return pathutil.SplitCSV(s)
}

func containsString(items []string, want string) bool {
	return pathutil.ContainsString(items, want)
}

func stateDoctorHasIssueCode(issues []statecontract.StateDoctorIssue, want string) bool {
	return pathutil.StateDoctorHasIssueCode(issues, want)
}

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestControllerStatic(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt, evidence string) string {
	return apidoc.BuildReviewPrompt(files, diff, extraPrompt, evidence)
}

func apiDocReviewEvidence(repo string, files []string) string {
	return apidoc.Evidence(repo, files)
}

func apiDocReviewSchema() map[string]any {
	return apidoc.ReviewSchema()
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return apidoc.NormalizeFiles(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return apidoc.IsCandidate(file)
}

type apiDocStaticViolation = apidoc.StaticViolation

var errAPIDocReviewGateFailed = apidoc.ErrReviewGateFailed
var errAPIDocStaticGateFailed = apidoc.ErrStaticGateFailed
