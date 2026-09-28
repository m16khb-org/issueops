package issueopsbranch

import (
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type ActiveCycleReader struct {
	Scan      func() ([]model.IssueOpsRecord, error)
	CleanPath func(string) string
}

func (r ActiveCycleReader) UmbrellaForChildIssue(repo, issueURL string) (model.IssueOpsRecord, bool) {
	issueURL = strings.TrimSpace(issueURL)
	if issueURL == "" {
		return model.IssueOpsRecord{}, false
	}
	return domain.SelectUmbrellaCycle(r.observe(repo, false), issueURL)
}

func (r ActiveCycleReader) ForWorkspace(workspace string) (model.IssueOpsRecord, bool) {
	return domain.SelectWorkspaceCycle(r.observe(workspace, true))
}

func (r ActiveCycleReader) PreparedBaseBranchForWorkspace(workspace string) (string, bool) {
	record, ok := r.ForWorkspace(workspace)
	if !ok {
		return "", false
	}
	return domain.PreparedCycleBase(record)
}

func (r ActiveCycleReader) observe(path string, worktree bool) []domain.CyclePathMatch {
	path = r.CleanPath(path)
	if path == "" {
		return nil
	}
	records, err := r.Scan()
	if err != nil {
		return nil
	}
	matches := make([]domain.CyclePathMatch, 0, len(records))
	for _, record := range records {
		match := domain.CyclePathMatch{Record: record, Repository: r.CleanPath(record.Repo) == path}
		if worktree {
			match.Worktree = r.CleanPath(record.WorktreePath) == path
		}
		matches = append(matches, match)
	}
	return matches
}
