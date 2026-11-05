package issueopsreview

import (
	contract "issueops/internal/contract/issueopsreview"
	"slices"
	"strings"
)

// ChangeBaseCandidates lists the refs to try as the diff base, most specific first:
// the literal base SHA when it is a full object ID, then origin/<branch>, then the
// local branch. It returns nil when no base was prepared.
func ChangeBaseCandidates(prepared bool, sha, branch string) []contract.ChangeBaseCandidate {
	if !prepared {
		return nil
	}
	candidates := []contract.ChangeBaseCandidate{}
	if sha = strings.TrimSpace(sha); fullGitObjectID(sha) {
		candidates = append(candidates, contract.ChangeBaseCandidate{Ref: sha, LiteralObject: true})
	}
	if branch = strings.TrimSpace(branch); branch != "" {
		candidates = append(candidates, contract.ChangeBaseCandidate{Ref: "origin/" + branch}, contract.ChangeBaseCandidate{Ref: branch})
	}
	return candidates
}

func fullGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// SameChangeSnapshot reports whether two observations saw the same changed paths in
// the same order and the same content fingerprint.
func SameChangeSnapshot(first, second []string, firstFingerprint, secondFingerprint string) bool {
	return slices.Equal(first, second) && firstFingerprint == secondFingerprint
}

// ImplementationChange reports whether a changed path counts as implementation
// evidence: it must be non-empty and not match the plan file, because a plan edit
// alone is not implementation evidence.
func ImplementationChange(path string, matchesPlan bool) bool { return path != "" && !matchesPlan }

// ImplementationEvidenceMissing returns the implementation_changes gate key when the
// change set holds no implementation evidence, and "" otherwise.
func ImplementationEvidenceMissing(hasEvidence bool) string {
	if !hasEvidence {
		return "implementation_changes"
	}
	return ""
}
