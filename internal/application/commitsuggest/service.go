package commitsuggest

import (
	"fmt"
	domain "issueops/internal/domain/commitsuggest"

	commitsuggestcontract "issueops/internal/contract/commitsuggest"
)

type Effects interface {
	NormalizeRoot(string) (string, error)
	Diff(root string, staged bool) (string, error)
}

type Service struct{ Effects Effects }

func (service Service) Suggest(req commitsuggestcontract.Request) (commitsuggestcontract.Result, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return commitsuggestcontract.Result{}, err
	}
	diff, err := service.Effects.Diff(root, req.Staged)
	if err != nil {
		return commitsuggestcontract.Result{}, fmt.Errorf("git diff failed: %w", err)
	}
	result := commitsuggestcontract.Result{OK: true, RepoRoot: root, Staged: req.Staged}
	if !domain.NeedsSuggestion(diff) {
		return result, nil
	}
	result.Executed = true
	result.Prompt = BuildPrompt(diff)
	return result, nil
}
