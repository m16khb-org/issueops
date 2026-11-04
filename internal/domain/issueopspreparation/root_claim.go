package issueopspreparation

// CanonicalRootClaimConflict reports whether candidateID, another cycle,
// already claims target as its worktree or execution root. An empty target or
// the cycle's own ID never conflicts.
func CanonicalRootClaimConflict(selfID, target, candidateID, worktreeRoot, executionRoot string) bool {
	if target == "" || candidateID == selfID {
		return false
	}
	return worktreeRoot == target || executionRoot == target
}
