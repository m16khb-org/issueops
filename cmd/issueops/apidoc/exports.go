package apidoc

import (
	"errors"

	"issueops/cmd/issueops/apidoc/reviewfiles"
)

type (
	ReviewOptions = apiDocReviewOptions
	ReviewResult  = apiDocReviewResult
	StaticOptions = apiDocStaticOptions
	StaticResult  = apiDocStaticResult
)

func Run(args []string) error {
	return runAPIDoc(args)
}

func RunReviewWithOptions(options ReviewOptions) (ReviewResult, error) {
	return runAPIDocReviewWithOptions(options)
}

func RunStaticCheckWithOptions(options StaticOptions) (StaticResult, error) {
	return runAPIDocStaticCheckWithOptions(options)
}

func Evidence(repo string, files []string) string {
	return reviewfiles.Evidence(repo, files)
}

func IsReviewGateError(err error) bool {
	return errors.Is(err, ErrReviewGateFailed) || errors.Is(err, ErrReviewResultRequired)
}

func IsStaticGateError(err error) bool {
	return errors.Is(err, ErrStaticGateFailed)
}
