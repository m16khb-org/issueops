package issueopspublication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopspublication"
)

func BodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func CandidateTitle(candidate contract.Candidate) string {
	title := strings.TrimSpace(candidate.Title)
	if !candidate.Draft {
		return title
	}
	for _, prefix := range []string{"Draft:", "WIP:"} {
		if len(title) >= len(prefix) && strings.EqualFold(title[:len(prefix)], prefix) {
			return strings.TrimSpace(title[len(prefix):])
		}
	}
	return title
}

func CandidateDraftMatches(candidate contract.Candidate, expectedDraft bool) bool {
	if candidate.Draft == expectedDraft {
		return true
	}
	if !expectedDraft || candidate.Draft {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(candidate.State)) {
	case "merged", "closed":
		return true
	default:
		return false
	}
}

func ValidateCandidate(request contract.ProviderCreateRequest, candidate contract.Candidate, knownURL string) error {
	if strings.TrimSpace(candidate.ProjectKey) != request.ProjectKey || strings.TrimSpace(candidate.SourceProjectKey) != request.ProjectKey ||
		strings.TrimSpace(candidate.HeadBranch) != request.HeadBranch || strings.TrimSpace(candidate.BaseBranch) != request.BaseBranch ||
		strings.TrimSpace(candidate.HeadSHA) != request.ExpectedHeadSHA || CandidateTitle(candidate) != request.Title ||
		strings.TrimSpace(candidate.BodySHA256) != BodySHA256(request.Body) || !CandidateDraftMatches(candidate, request.Draft) ||
		!sameCanonicalSet(candidate.Labels, request.Labels) || !sameCanonicalSet(candidate.Assignees, request.Assignees) {
		return fmt.Errorf("remote reconcile candidate does not match the exact durable intent")
	}
	if knownURL != "" && strings.TrimSpace(candidate.URL) != knownURL {
		return fmt.Errorf("remote reconcile candidate URL differs from the durable known URL")
	}
	return nil
}

func sameCanonicalSet(left, right []string) bool {
	leftSet, rightSet := canonicalSet(left), canonicalSet(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for value := range rightSet {
		if _, ok := leftSet[value]; !ok {
			return false
		}
	}
	return true
}

func canonicalSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !strings.Contains(value, "\x00") {
			result[value] = struct{}{}
		}
	}
	return result
}
