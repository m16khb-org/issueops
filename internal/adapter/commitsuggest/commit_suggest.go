package commitsuggest

import (
	commitsuggestapp "issueops/internal/application/commitsuggest"
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	"os/exec"
)

func SuggestCommit(req commitsuggestcontract.CommitSuggestRequest) (commitsuggestcontract.CommitSuggestResult, error) {
	return (commitsuggestapp.Service{Effects: commitEffects{}}).Suggest(req)
}

func BuildPrompt(diff string) string { return commitsuggestapp.BuildPrompt(diff) }

type commitEffects struct{}

func (commitEffects) NormalizeRoot(root string) (string, error) { return NormalizeRepoRoot(root) }

func (commitEffects) Diff(root string, staged bool) (string, error) {
	args := []string{"-C", root, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	output, err := exec.Command("git", args...).Output()
	return string(output), err
}
