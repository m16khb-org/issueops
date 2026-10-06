package issueopsapp

import (
	"errors"

	"issueops/cmd/issueops/apidoc/reviewprompt"
	"issueops/cmd/issueops/pathutil"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
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
