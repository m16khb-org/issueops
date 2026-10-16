package operationalhealth

import (
	"fmt"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"strings"
)

func validateCanonicalSource(builder *findingBuilder, snapshot Snapshot) {
	repo := clean(snapshot.RepoRoot)
	branch := strings.TrimSpace(snapshot.CanonicalBranch)
	head := strings.TrimSpace(snapshot.SourceHead)
	if repo == "" || branch == "" || head == "" {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "source", "source", "canonical source repository, branch, and HEAD identity must be complete", repo)
		return
	}
	if !snapshot.SourceClean {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "source", repo, "canonical source checkout is not clean", repo)
	}
	canonicalWorktrees := make([]operationalhealthcontract.GitWorktree, 0, 1)
	for _, worktree := range snapshot.GitWorktrees {
		if worktree.Canonical || clean(worktree.Path) == repo {
			canonicalWorktrees = append(canonicalWorktrees, worktree)
		}
	}
	if len(canonicalWorktrees) != 1 || clean(canonicalWorktrees[0].Path) != repo || !canonicalWorktrees[0].Canonical || strings.TrimSpace(canonicalWorktrees[0].Branch) != branch || strings.TrimSpace(canonicalWorktrees[0].Head) != head || !canonicalWorktrees[0].Clean {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "source_worktree", repo, "canonical Git worktree must occur once and match the clean source branch and HEAD", repo)
	}
	validateCanonicalRef(builder, snapshot.LocalRefs, "local", branch, head)
	validateCanonicalRef(builder, snapshot.RemoteRefs, "remote", branch, head)
}

func validateCanonicalRef(builder *findingBuilder, refs []operationalhealthcontract.GitRef, location, branch, head string) {
	matches := make([]operationalhealthcontract.GitRef, 0, 1)
	for _, ref := range refs {
		if strings.TrimSpace(ref.Location) == location && strings.TrimSpace(ref.Branch) == branch {
			matches = append(matches, ref)
		}
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].OID) != head {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "branch", location+":"+branch, fmt.Sprintf("canonical %s ref must occur once at source HEAD %s", location, head), "")
	}
}

func knownPhase(value string) bool {
	switch value {
	case "problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done":
		return true
	default:
		return false
	}
}

func knownLeaseStatus(value string) bool {
	switch value {
	case "", "claimable", "active", "revoking", "released":
		return true
	default:
		return false
	}
}

func hasExecutionProjection(cycle operationalhealthcontract.Cycle) bool {
	return cycle.Generation != 0 || strings.TrimSpace(cycle.WorktreePath) != "" || !holderIdentityEmpty(cycle)
}

func executionIdentityComplete(cycle operationalhealthcontract.Cycle) bool {
	return strings.TrimSpace(cycle.Branch) != "" && strings.TrimSpace(cycle.WorktreePath) != "" && cycle.Generation != 0
}

func retainedLeaseIdentityComplete(cycle operationalhealthcontract.Cycle) bool {
	switch strings.TrimSpace(cycle.LeaseStatus) {
	case "claimable", "released":
		return holderIdentityEmpty(cycle)
	case "revoking":
		return holderIdentityComplete(cycle)
	default:
		return false
	}

}

func holderIdentityComplete(cycle operationalhealthcontract.Cycle) bool {
	host := strings.TrimSpace(cycle.HolderHost)
	return validNativeHost(host) && strings.TrimSpace(cycle.HolderSessionID) != "" &&
		cycle.HolderPID > 0 && strings.TrimSpace(cycle.HolderStartedAt) != "" && strings.TrimSpace(cycle.HolderExecutable) != ""
}

func validNativeHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "codex" || host == "claude" || host == "omo" || host == "omp"
}

func holderIdentityEmpty(cycle operationalhealthcontract.Cycle) bool {
	return strings.TrimSpace(cycle.HolderHost) == "" && strings.TrimSpace(cycle.HolderSessionID) == "" &&
		strings.TrimSpace(cycle.HolderAgentID) == "" && cycle.HolderPID == 0 && strings.TrimSpace(cycle.HolderStartedAt) == "" &&
		strings.TrimSpace(cycle.HolderExecutable) == ""
}

func preservableIdentityComplete(cycle operationalhealthcontract.Cycle) bool {
	if strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" || strings.TrimSpace(cycle.Branch) == "" {
		return false
	}
	status := strings.TrimSpace(cycle.LeaseStatus)
	if status == "" {
		return !hasExecutionProjection(cycle)
	}
	if !knownLeaseStatus(status) || !executionIdentityComplete(cycle) {
		return false
	}
	if status == "active" {
		return holderIdentityComplete(cycle) && strings.TrimSpace(cycle.HolderProcessStatus) == operationalhealthcontract.ProcessStatusLive
	}
	return retainedLeaseIdentityComplete(cycle)
}
