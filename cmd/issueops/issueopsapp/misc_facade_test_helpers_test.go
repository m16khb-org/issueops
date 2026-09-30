package issueopsapp

import (
	"errors"

	"issueops/cmd/issueops/apidoc/reviewprompt"
	"issueops/cmd/issueops/pathutil"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
	statecontract "issueops/internal/contract/state"
	domain "issueops/internal/domain/apidoc"
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
	return domain.CheckNestController(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return domain.CheckNestDTO(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt, evidence string) string {
	return reviewprompt.Build(files, diff, extraPrompt, evidence)
}

func apiDocReviewEvidence(repo string, files []string) string {
	return reviewfiles.Evidence(repo, files)
}

func apiDocReviewSchema() map[string]any {
	return reviewprompt.Schema()
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return reviewfiles.Normalize(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return domain.IsCandidate(file)
}

type apiDocStaticViolation = contract.Violation

var errAPIDocReviewGateFailed = app.ErrReviewGateFailed
var errAPIDocStaticGateFailed = app.ErrStaticGateFailed
