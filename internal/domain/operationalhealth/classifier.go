package operationalhealth

import (
	"fmt"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"sort"
	"strings"
)

func EvaluateCycleAuthority(cycle operationalhealthcontract.Cycle, opts Options) operationalhealthcontract.CycleAuthority {
	phase := strings.TrimSpace(cycle.Phase)
	status := strings.TrimSpace(cycle.LeaseStatus)
	if !cycleIdentityKnown(cycle, phase, status) {
		return AuthorityUnknown
	}
	if status == "" {
		return evaluateLeaselessCycleAuthority(cycle, phase, opts)
	}
	if !executionIdentityComplete(cycle) {
		return AuthorityUnknown
	}
	if phase == "done" {
		return evaluateDoneCycleAuthority(cycle, status)
	}
	if cycle.CompletionPresent {
		return AuthorityUnknown
	}
	if preserveContains(opts.PreserveCycleIDs, cycle.ID) {
		if !preservableIdentityComplete(cycle) {
			return AuthorityUnknown
		}
		return AuthorityPreserved
	}
	if status == "active" {
		return evaluateActiveHolderAuthority(cycle)
	}
	if retainedLeaseIdentityComplete(cycle) {
		return AuthorityPreserved
	}
	return AuthorityUnknown
}

// cycleIdentityKnown은 phase, ID, repo, lease 상태가 모두 알려진 값인지 판단한다.
func cycleIdentityKnown(cycle operationalhealthcontract.Cycle, phase, status string) bool {
	if !knownPhase(phase) || strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" {
		return false
	}
	return knownLeaseStatus(status)
}

// evaluateDoneCycleAuthority는 done phase 사이클이 완료 기록과 함께 해제된 경우에만 dead로 판정한다.
func evaluateDoneCycleAuthority(cycle operationalhealthcontract.Cycle, status string) operationalhealthcontract.CycleAuthority {
	if status == "released" && cycle.CompletionPresent && retainedLeaseIdentityComplete(cycle) {
		return operationalhealthcontract.AuthorityDead
	}
	return AuthorityUnknown
}

func evaluateLeaselessCycleAuthority(cycle operationalhealthcontract.Cycle, phase string, opts Options) operationalhealthcontract.CycleAuthority {
	if phase == "done" || hasExecutionProjection(cycle) {
		return AuthorityUnknown
	}
	if preserveContains(opts.PreserveCycleIDs, cycle.ID) && !preservableIdentityComplete(cycle) {
		return AuthorityUnknown
	}
	return AuthorityPreserved
}

func evaluateActiveHolderAuthority(cycle operationalhealthcontract.Cycle) operationalhealthcontract.CycleAuthority {
	if !holderIdentityComplete(cycle) {
		return AuthorityUnknown
	}
	switch strings.TrimSpace(cycle.HolderProcessStatus) {
	case operationalhealthcontract.ProcessStatusLive:
		return AuthorityLive
	case operationalhealthcontract.ProcessStatusDead, operationalhealthcontract.ProcessStatusIdentityMismatch:
		return operationalhealthcontract.AuthorityDead
	default:
		return AuthorityUnknown
	}
}

func classifyCycleAuthorities(builder *findingBuilder, snapshot Snapshot, opts Options) map[string]operationalhealthcontract.CycleAuthority {
	authorities := make(map[string]operationalhealthcontract.CycleAuthority, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		id := strings.TrimSpace(cycle.ID)
		authority := EvaluateCycleAuthority(cycle, opts)
		authorities[id] = authority
		if cycle.ExecutionFailurePresent {
			builder.add(operationalhealthcontract.FindingExecutionFailure, "cycle", id, "cycle has a durable execution failure; inspect issueops list and reconcile the failed operation", clean(cycle.WorktreePath))
		}
		if cycle.CleanupFailurePresent {
			builder.add(operationalhealthcontract.FindingCleanupFailure, "cycle", id, "cycle has a durable cleanup failure; inspect issueops list and resume cleanup from preview", clean(cycle.WorktreePath))
		}
		if cycle.IssueCreateFailurePresent {
			builder.add(operationalhealthcontract.FindingIssueCreateFailure, "cycle", id, "cycle has an ambiguous or failed durable issue creation; run issueops remote reconcile-issue", clean(cycle.WorktreePath))
		}
		switch {
		case authority == AuthorityUnknown:
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "cycle", id, "cycle phase, execution lease, or durable identity is unsupported or incomplete", clean(cycle.WorktreePath))
		case authority == operationalhealthcontract.AuthorityDead && strings.TrimSpace(cycle.Phase) != "done":
			builder.add(operationalhealthcontract.FindingDeadOwner, "cycle", id, "cycle execution lease has no live or preserved authority", clean(cycle.WorktreePath))
		}
	}
	return authorities
}

func validatePreservedCycles(builder *findingBuilder, opts Options, cycleCounts map[string]int) {
	_, invalidPreserveCycles := normalizedSet(opts.PreserveCycleIDs)
	for _, invalid := range invalidPreserveCycles {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "preserve_cycle", invalid, "preserved cycle ids must be non-empty and unique", "")
	}
	for id := range preserveSet(opts.PreserveCycleIDs) {
		if cycleCounts[id] != 1 {
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "cycle", id, "preserved cycle must exist exactly once", "")
		}
	}
}

func validateWorktreeResidues(builder *findingBuilder, snapshot Snapshot, gitPathOwners map[string][]string) {
	for _, worktree := range snapshot.GitWorktrees {
		path := clean(worktree.Path)
		if worktree.Canonical || (path == clean(snapshot.RepoRoot) && strings.TrimSpace(worktree.Branch) == strings.TrimSpace(snapshot.CanonicalBranch)) {
			continue
		}
		if len(gitPathOwners[path]) == 1 {
			continue
		}
		builder.add(operationalhealthcontract.FindingWorktreeResidue, "git_worktree", path, "non-canonical Git worktree has no live or invocation-preserved owner", path)
	}
}

func validateBranchResidues(builder *findingBuilder, snapshot Snapshot, activeRepoCycles []operationalhealthcontract.Cycle) {
	branchResidue := make([]string, 0)
	activeBranches := map[string]struct{}{strings.TrimSpace(snapshot.CanonicalBranch): {}}
	for _, cycle := range activeRepoCycles {
		if branch := strings.TrimSpace(cycle.Branch); branch != "" {
			activeBranches[branch] = struct{}{}
		}
	}
	for _, ref := range append(append([]operationalhealthcontract.GitRef(nil), snapshot.LocalRefs...), snapshot.RemoteRefs...) {
		branch := strings.TrimSpace(ref.Branch)
		if _, ok := activeBranches[branch]; ok {
			continue
		}
		branchResidue = append(branchResidue, fmt.Sprintf("%s:%s@%s", strings.TrimSpace(ref.Location), strings.TrimSpace(ref.Name), strings.TrimSpace(ref.OID)))
	}
	if len(branchResidue) > 0 {
		sort.Strings(branchResidue)
		builder.add(operationalhealthcontract.FindingNonMainBranchResidue, "branch", "non_main", fmt.Sprintf("%d non-canonical refs have no live or invocation-preserved owner: %s", len(branchResidue), strings.Join(branchResidue, ",")), "")
	}
}

func Classify(snapshot Snapshot, opts Options) operationalhealthcontract.Result {
	builder := findingBuilder{seen: map[string]struct{}{}}
	if opts.Now.IsZero() {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "clock", "now", "operational classification requires an explicit current time", "")
	}
	for _, problem := range snapshot.InventoryProblems {
		id := firstNonEmpty(problem.Code, problem.Source, "inventory")
		builder.add(operationalhealthcontract.FindingInventoryUnknown, problem.Source, id, firstNonEmpty(problem.Detail, problem.Code, "inventory collection failed"), "")
	}
	validateCanonicalSource(&builder, snapshot)

	resources := newResourceIndex(snapshot)
	cycleCounts := resources.cycles.counts()
	gitPathCounts := resources.gitPaths.counts()

	addDuplicateFindings(&builder, "cycle", cycleCounts)
	addDuplicateFindings(&builder, "git_worktree", gitPathCounts)
	validateLeaseHolderIndexes(&builder, snapshot.Cycles, snapshot.LeaseHolderIndexes)

	authorities := classifyCycleAuthorities(&builder, snapshot, opts)

	activeCycles := make([]operationalhealthcontract.Cycle, 0, len(snapshot.Cycles))
	activeRepoCycles := make([]operationalhealthcontract.Cycle, 0, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		authority := authorities[strings.TrimSpace(cycle.ID)]
		if authority == AuthorityLive || authority == AuthorityPreserved {
			activeCycles = append(activeCycles, cycle)
			if clean(cycle.Repo) == clean(snapshot.RepoRoot) {
				activeRepoCycles = append(activeRepoCycles, cycle)
			}
		}
	}

	gitPathOwners := ownerIndex(activeRepoCycles, func(cycle operationalhealthcontract.Cycle) string { return clean(cycle.WorktreePath) })
	for id, cycleIDs := range gitPathOwners {
		if len(cycleIDs) > 1 {
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "git_worktree", id, "resource is claimed by multiple active cycles: "+strings.Join(cycleIDs, ","), "")
		}
	}

	for _, cycle := range activeCycles {
		validateCycleGitWorktree(&builder, resources, cycle, authorities[strings.TrimSpace(cycle.ID)], clean(cycle.Repo) == clean(snapshot.RepoRoot), gitPathCounts)
	}

	validatePreservedCycles(&builder, opts, cycleCounts)
	validateWorktreeResidues(&builder, snapshot, gitPathOwners)
	validateBranchResidues(&builder, snapshot, activeRepoCycles)

	for _, artifact := range snapshot.StateArtifacts {
		path := clean(artifact.Path)
		builder.add(operationalhealthcontract.FindingStateArtifactResidue, "state_artifact", path, "state directory contains an unexpected runtime or recovery artifact", path)
	}

	findings := builder.sorted()
	return operationalhealthcontract.Result{Healthy: len(findings) == 0, Findings: findings}
}

func SeverityForFinding(code string) string {
	if code == operationalhealthcontract.FindingInventoryUnknown {
		return "error"
	}
	return "warning"
}
