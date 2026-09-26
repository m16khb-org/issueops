package commitsuggest

import (
	"fmt"
	"strings"

	commitsuggestcontract "issueops/internal/contract/commitsuggest"
)

type Effects interface {
	NormalizeRoot(string) (string, error)
	Diff(root string, staged bool) (string, error)
}

type Service struct{ Effects Effects }

func (service Service) Suggest(req commitsuggestcontract.CommitSuggestRequest) (commitsuggestcontract.CommitSuggestResult, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return commitsuggestcontract.CommitSuggestResult{}, err
	}
	diff, err := service.Effects.Diff(root, req.Staged)
	if err != nil {
		return commitsuggestcontract.CommitSuggestResult{}, fmt.Errorf("git diff failed: %w", err)
	}
	result := commitsuggestcontract.CommitSuggestResult{OK: true, RepoRoot: root, Staged: req.Staged}
	if strings.TrimSpace(diff) == "" {
		return result, nil
	}
	result.Executed = true
	result.Prompt = BuildPrompt(diff)
	return result, nil
}
