package commitsuggest

import (
	"context"
	"fmt"
	domain "issueops/internal/domain/commitsuggest"

	commitsuggestcontract "issueops/internal/contract/commitsuggest"
)

type effects interface {
	NormalizeRoot(string) (string, error)
	Diff(ctx context.Context, root string, staged bool) (string, error)
}

// Service builds a commit message suggestion from the repository diff read
// through Effects.
type Service struct{ Effects effects }

// Suggest reads the staged or unstaged diff of req.RepoRoot and returns a
// result carrying the prompt for the host agent. When the diff needs no
// suggestion the result is OK with Executed false and no prompt. ctx bounds the
// diff: when it ends first, Suggest returns an error that wraps ctx.Err().
func (service Service) Suggest(ctx context.Context, req commitsuggestcontract.Request) (commitsuggestcontract.Result, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return commitsuggestcontract.Result{}, err
	}
	diff, err := service.Effects.Diff(ctx, root, req.Staged)
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
