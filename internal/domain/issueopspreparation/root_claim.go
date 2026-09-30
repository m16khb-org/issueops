package issueopspreparation

func CanonicalRootClaimConflict(selfID, target, candidateID, worktreeRoot, executionRoot string) bool {
	if target == "" || candidateID == selfID {
		return false
	}
	return worktreeRoot == target || executionRoot == target
}
