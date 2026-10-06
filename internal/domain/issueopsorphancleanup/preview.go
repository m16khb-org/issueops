package issueopsorphancleanup

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	contract "issueops/internal/contract/issueopsorphancleanup"
	corehealth "issueops/internal/domain/operationalhealth"
)

func NormalizeRequest(request contract.Request) (contract.Request, error) {
	request.ID = strings.TrimSpace(request.ID)
	request.Branch = strings.TrimSpace(request.Branch)
	request.Artifact.Provider = strings.ToLower(strings.TrimSpace(request.Artifact.Provider))
	request.Artifact.Kind = strings.ToLower(strings.TrimSpace(request.Artifact.Kind))
	request.Artifact.URL = strings.TrimSpace(request.Artifact.URL)
	if request.ID == "" {
		return contract.Request{}, fmt.Errorf("orphan cleanup id is required")
	}
	if request.Branch == "" {
		return contract.Request{}, fmt.Errorf("orphan cleanup branch is required")
	}
	if request.Artifact.Provider == "" || request.Artifact.Kind == "" || request.Artifact.URL == "" {
		return contract.Request{}, fmt.Errorf("orphan cleanup provider, artifact kind, and artifact URL are required")
	}
	return request, nil
}

func ResultForRequest(request contract.Request) contract.Result {
	return contract.Result{
		OK:                   true,
		Preview:              true,
		ID:                   request.ID,
		RepoRoot:             request.RepoRoot,
		WorktreePath:         request.WorktreePath,
		Branch:               request.Branch,
		Provider:             request.Artifact.Provider,
		ArtifactKind:         request.Artifact.Kind,
		RemoteArtifactURL:    request.Artifact.URL,
		RecordAbsent:         true,
		RemoteBranchDeletion: "remote branch is intentionally untouched; deletion requires separate explicit approval",
		Missing:              []string{},
		Warnings:             []string{},
	}
}

func InspectInventory(result *contract.Result, request contract.Request, snapshot corehealth.Snapshot) {
	if !samePath(snapshot.RepoRoot, request.RepoRoot) {
		Missing(result, "repo_root_match")
		return
	}
	if len(snapshot.InventoryProblems) > 0 {
		Missing(result, "inventory_complete")
	}
	canonicalCount := 0
	for _, worktree := range snapshot.GitWorktrees {
		if worktree.Canonical && samePath(worktree.Path, request.RepoRoot) {
			canonicalCount++
		}
		if !samePath(worktree.Path, request.WorktreePath) {
			continue
		}
		result.TargetWorktreeCount++
		result.TargetCanonical = result.TargetCanonical || worktree.Canonical
		if result.HeadSHA == "" {
			result.HeadSHA = strings.TrimSpace(worktree.Head)
		}
		if strings.TrimSpace(worktree.Branch) != request.Branch {
			// branch_match: `missing`은 충족되지 않은 요구의 목록이므로 요구형으로
			// 적는다. cleanup status가 같은 조건에 쓰는 이름과 같아진다(#185).
			Missing(result, "branch_match")
		}
	}
	if canonicalCount != 1 {
		Missing(result, "canonical_repo_root")
	}
	if result.TargetWorktreeCount != 1 {
		Missing(result, "target_worktree_count")
	}
	if result.TargetCanonical {
		Missing(result, "canonical_worktree")
	}
	if !validGitOID(result.HeadSHA) {
		Missing(result, "worktree_head")
	} else {
		result.RecoveryHead = result.HeadSHA
		result.RecoveryPath = filepath.Join(filepath.Dir(request.WorktreePath), filepath.Base(request.WorktreePath)+"-recovery-"+result.HeadSHA[:12])
	}
	matchingLocalRefs := 0
	for _, ref := range snapshot.LocalRefs {
		if strings.TrimSpace(ref.Location) != "local" || strings.TrimSpace(ref.Branch) != request.Branch {
			continue
		}
		matchingLocalRefs++
		result.LocalBranchOID = strings.TrimSpace(ref.OID)
	}
	if matchingLocalRefs != 1 || result.LocalBranchOID == "" || result.LocalBranchOID != result.HeadSHA {
		Missing(result, "local_branch_head")
	}
	for _, cycle := range snapshot.Cycles {
		if strings.TrimSpace(cycle.ID) == request.ID {
			result.RecordAbsent = false
			Missing(result, "record_present")
		}
		ownsTargetBranch := samePath(cycle.Repo, request.RepoRoot) && strings.TrimSpace(cycle.Branch) == request.Branch
		if !samePath(cycle.WorktreePath, request.WorktreePath) && !ownsTargetBranch {
			continue
		}
		result.RecordAbsent = false
		Missing(result, "target_record_present")
		authority := corehealth.EvaluateCycleAuthority(cycle, corehealth.Options{})
		if authority == corehealth.AuthorityLive || authority == corehealth.AuthorityPreserved {
			Missing(result, "target_lifecycle_owner")
		} else if authority == corehealth.AuthorityUnknown {
			Missing(result, "target_lifecycle_authority_unknown")
		}
	}
	for _, index := range snapshot.LeaseHolderIndexes {
		if strings.TrimSpace(index.LifecycleID) == request.ID {
			Missing(result, "target_lease_authority")
		}
	}
	for _, worktree := range snapshot.OrcaWorktrees {
		if samePath(worktree.Path, request.WorktreePath) {
			Missing(result, "orca_worktree_authority")
		}
	}
}

func validGitOID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, runeValue := range value {
		if !((runeValue >= '0' && runeValue <= '9') || (runeValue >= 'a' && runeValue <= 'f') || (runeValue >= 'A' && runeValue <= 'F')) {
			return false
		}
	}
	return true
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// Inventory paths have already been canonicalized at the filesystem boundary.
func samePath(left, right string) bool { return left != "" && right != "" && left == right }

func Missing(result *contract.Result, code string) {
	if code = strings.TrimSpace(code); code != "" {
		result.Missing = append(result.Missing, code)
	}
}
func Finish(result *contract.Result) {
	result.Missing = uniqueSorted(result.Missing)
	result.Warnings = uniqueSorted(result.Warnings)
	result.Ready = len(result.Missing) == 0
}

// LocalObservation contains filesystem and Git facts, not execution policy.
type LocalObservation struct {
	PathsObservable    bool
	CleanObservable    bool
	Clean              bool
	StateOutsideTarget bool
}

func LocalPreview(request contract.Request, snapshot corehealth.Snapshot, observed LocalObservation) contract.Result {
	result := ResultForRequest(request)
	result.InventoryRefreshed = true
	if !observed.PathsObservable {
		Missing(&result, "inventory_complete")
	}
	InspectInventory(&result, request, snapshot)
	if !observed.CleanObservable {
		Missing(&result, "worktree_git_status")
	} else {
		result.TargetClean = observed.Clean
		if !observed.Clean {
			Missing(&result, "worktree_clean")
		}
	}
	if !observed.StateOutsideTarget {
		Missing(&result, "state_store_outside_target")
	}
	return result
}
func MergeEvidence(result *contract.Result, verified bool) {
	result.RemoteMerged = verified
	if !verified {
		Missing(result, "remote_artifact_merged")
		result.Warnings = append(result.Warnings, "remote merge evidence could not be verified through the configured provider")
	}
}
