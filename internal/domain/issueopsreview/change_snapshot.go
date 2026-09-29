package issueopsreview

import (
	contract "issueops/internal/contract/issueopsreview"
	"slices"
	"strings"
)

func ChangeBaseCandidates(prepared bool, sha, branch string) []contract.ChangeBaseCandidate {
	if !prepared {
		return nil
	}
	candidates := []contract.ChangeBaseCandidate{}
	if sha = strings.TrimSpace(sha); FullGitObjectID(sha) {
		candidates = append(candidates, contract.ChangeBaseCandidate{Ref: sha, LiteralObject: true})
	}
	if branch = strings.TrimSpace(branch); branch != "" {
		candidates = append(candidates, contract.ChangeBaseCandidate{Ref: "origin/" + branch}, contract.ChangeBaseCandidate{Ref: branch})
	}
	return candidates
}

func FullGitObjectID(value string) bool {
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

func SameChangeSnapshot(first, second []string, firstFingerprint, secondFingerprint string) bool {
	return slices.Equal(first, second) && firstFingerprint == secondFingerprint
}

// A plan edit alone is not implementation evidence.
func ImplementationChange(path string, matchesPlan bool) bool { return path != "" && !matchesPlan }

func ImplementationEvidenceMissing(hasEvidence bool) string {
	if !hasEvidence {
		return "implementation_changes"
	}
	return ""
}
