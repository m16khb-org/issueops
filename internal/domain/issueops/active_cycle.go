// Package issueops holds the pure domain policy of an IssueOps cycle: phase
// progression and readiness gates, branch and worktree preparation, execution
// lease validation and replacement, child cycle delegation, and cleanup
// previews. It depends only on the contract model and does no I/O: callers in
// the application layer pass in observations (git, filesystem, provider) and
// apply the returned records, missing-gate lists, and commands.
package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// CyclePathMatch carries path observations; selection never resolves paths itself.
type CyclePathMatch struct {
	Record               model.Record
	Repository, Worktree bool
}

// SelectUmbrellaCycle returns the first open cycle whose repository matched
// and that links childIssueURL as a child issue. Done cycles, non-repository
// matches, and a blank URL yield false.
func SelectUmbrellaCycle(matches []CyclePathMatch, childIssueURL string) (model.Record, bool) {
	childIssueURL = strings.TrimSpace(childIssueURL)
	if childIssueURL == "" {
		return model.Record{}, false
	}
	for _, candidate := range matches {
		record := candidate.Record
		if !candidate.Repository || record.Phase == model.PhaseDone {
			continue
		}
		for _, link := range record.IssueLinks {
			if link.Type == "child" && strings.TrimSpace(link.URL) == childIssueURL {
				return record, true
			}
		}
	}
	return model.Record{}, false
}

// SelectWorkspaceCycle returns the open cycle that owns the current
// workspace. A linked worktree is more specific than a shared source
// checkout, so a worktree match wins; otherwise the first repository match in
// scan order is used. Done cycles are skipped.
func SelectWorkspaceCycle(matches []CyclePathMatch) (model.Record, bool) {
	var repoMatch model.Record
	repoMatched := false
	for _, candidate := range matches {
		if candidate.Record.Phase == model.PhaseDone {
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

// PreparedCycleBase returns the trimmed base branch sealed by branch prepare.
// The bool is false when the record has no preparation or its base branch is
// blank.
func PreparedCycleBase(record model.Record) (string, bool) {
	if record.BranchPrepare == nil {
		return "", false
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	return base, base != ""
}
