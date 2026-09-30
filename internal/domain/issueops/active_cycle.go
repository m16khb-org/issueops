package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// CyclePathMatch carries path observations; selection never resolves paths itself.
type CyclePathMatch struct {
	Record               model.IssueOpsRecord
	Repository, Worktree bool
}

func SelectUmbrellaCycle(matches []CyclePathMatch, childIssueURL string) (model.IssueOpsRecord, bool) {
	childIssueURL = strings.TrimSpace(childIssueURL)
	if childIssueURL == "" {
		return model.IssueOpsRecord{}, false
	}
	for _, candidate := range matches {
		record := candidate.Record
		if !candidate.Repository || record.Phase == model.IssueOpsPhaseDone {
			continue
		}
		for _, link := range record.IssueLinks {
			if link.Type == "child" && strings.TrimSpace(link.URL) == childIssueURL {
				return record, true
			}
		}
	}
	return model.IssueOpsRecord{}, false
}

// A linked worktree is more specific than a shared source checkout. Preserve
// scan order when several records match the source repository.
func SelectWorkspaceCycle(matches []CyclePathMatch) (model.IssueOpsRecord, bool) {
	var repoMatch model.IssueOpsRecord
	repoMatched := false
	for _, candidate := range matches {
		if candidate.Record.Phase == model.IssueOpsPhaseDone {
			continue
		}
		if candidate.Worktree {
			return candidate.Record, true
		}
		if !repoMatched && candidate.Repository {
			repoMatch, repoMatched = candidate.Record, true
		}
	}
	return repoMatch, repoMatched
}

func PreparedCycleBase(record model.IssueOpsRecord) (string, bool) {
	if record.BranchPrepare == nil {
		return "", false
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	return base, base != ""
}
