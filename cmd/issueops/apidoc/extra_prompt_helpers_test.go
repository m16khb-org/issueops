package apidoc

import (
	reviewfiles "issueops/internal/adapter/outbound/apidoc/reviewfiles"
	app "issueops/internal/application/apidoc"
)

func apiDocReviewExtraPrompt(options app.ReviewOptions) (string, error) {
	return reviewfiles.ExtraPrompt(options.Repo, options.PromptFile)
}
