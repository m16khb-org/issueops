package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// WorkBaseCommit is one base-side commit a work branch carries.
// FromEarlierBase marks a commit taken from a base branch the cycle has since
// retargeted away from.
type WorkBaseCommit struct {
	OID             string
	FromEarlierBase bool
}

// BaseCommitsInWork lists the base-side commits a work branch already
// carries: the sealed base it was cut from and every base tip a sync-base
// merged into it. A new base tip must hold each of them before merging it is
// safe.
func BaseCommitsInWork(record model.IssueOpsRecord) []WorkBaseCommit {
	if record.BranchPrepare == nil {
		return nil
	}
	current := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	var commits []WorkBaseCommit
	add := func(oid, branch string) {
		oid = strings.TrimSpace(oid)
		if oid == "" {
			return
		}
		for _, existing := range commits {
			if strings.EqualFold(existing.OID, oid) {
				return
			}
		}
		branch = strings.TrimSpace(branch)
		commits = append(commits, WorkBaseCommit{OID: oid, FromEarlierBase: branch != "" && branch != current})
	}
	sealedFrom := current
	if retargets := record.BranchPrepare.Retargets; len(retargets) > 0 {
		sealedFrom = retargets[0].FromBase
	}
	add(record.BranchPrepare.BaseSHA, sealedFrom)
	if record.Execution != nil {
		for _, event := range record.Execution.SyncBaseEvents {
			add(event.BaseOID, event.BaseBranch)
		}
	}
	return commits
}

// BaseHoldsWorkBase reports whether a base tip holds every base-side commit
// of the work branch, so merging it brings nothing the base dropped.
//
// A commit is held when it is an ancestor of the tip. A commit from a base
// the cycle retargeted away from is also held when its content is in the tip
// (contentAbsorbed: merging it into the tip changes nothing), which is how a
// squash-merged parent of a stacked branch looks. Commits of the current base
// keep the ancestor rule: a tree-equal rewrite of the same branch (a history
// purge) would otherwise let the PR merge bring the dropped commits back.
func BaseHoldsWorkBase(commits []WorkBaseCommit, ancestor, contentAbsorbed func(string) bool) bool {
	for _, commit := range commits {
		if ancestor(commit.OID) {
			continue
		}
		if commit.FromEarlierBase && contentAbsorbed(commit.OID) {
			continue
		}
		return false
	}
	return true
}
