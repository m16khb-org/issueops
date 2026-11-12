package commitsuggest

import (
	"context"

	"issueops/internal/adapter/repopath"
	app "issueops/internal/application/commitsuggest"
	model "issueops/internal/contract/commitsuggest"
)

func SuggestCommit(req model.Request) (model.Result, error) {
	return (app.Service{Effects: Effects{Normalize: repopath.NormalizeRoot}}).Suggest(context.Background(), req)
}
