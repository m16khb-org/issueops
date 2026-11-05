package issueopsbranch

import (
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// ActiveCycleReader finds the live cycle that owns a repository or worktree
// path. Scan lists every cycle record and CleanPath canonicalizes a path for
// comparison; a scan failure or an empty path yields no match.
type ActiveCycleReader struct {
	Scan      func() ([]model.Record, error)
	CleanPath func(string) string
}

// UmbrellaForChildIssue returns the umbrella cycle of repo that tracks
// issueURL as a child issue, and false when issueURL is blank or none matches.
func (r ActiveCycleReader) UmbrellaForChildIssue(repo, issueURL string) (model.Record, bool) {
	issueURL = strings.TrimSpace(issueURL)
	if issueURL == "" {
		return model.Record{}, false
	}
	return domain.SelectUmbrellaCycle(r.observe(repo, false), issueURL)
}

// ForWorkspace returns the active cycle whose worktree is workspace.
func (r ActiveCycleReader) ForWorkspace(workspace string) (model.Record, bool) {
	return domain.SelectWorkspaceCycle(r.observe(workspace, true))
}

// PreparedBaseBranchForWorkspace returns the base branch sealed by branch
// preparation for the cycle owning workspace, and false when there is no such
// cycle or it has not been prepared.
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
