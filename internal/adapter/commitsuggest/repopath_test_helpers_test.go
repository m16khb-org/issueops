package commitsuggest

import (
	"issueops/internal/adapter/repopath"
	app "issueops/internal/application/commitsuggest"
	model "issueops/internal/contract/commitsuggest"
)

func SuggestCommit(req model.CommitSuggestRequest) (model.CommitSuggestResult, error) {
	return (app.Service{Effects: Effects{Normalize: repopath.NormalizeRoot}}).Suggest(req)
}
