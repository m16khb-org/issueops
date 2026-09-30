package issueopspublication

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopspublication"
)

func TestValidateCandidateMatchesSealedCreateRequest(t *testing.T) {
	request := contract.ProviderCreateRequest{
		ProjectKey: "group/project", Title: "Ship the fix", Body: "body", HeadBranch: "work", BaseBranch: "main",
		ExpectedHeadSHA: strings.Repeat("a", 40), Labels: []string{"backend", "release"}, Assignees: []string{"owner"}, Draft: true,
	}
	candidate := contract.Candidate{
		URL: "https://gitlab.com/group/project/-/merge_requests/1", ProjectKey: request.ProjectKey, SourceProjectKey: request.ProjectKey,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch, HeadSHA: request.ExpectedHeadSHA,
		Title: "Draft: Ship the fix", BodySHA256: BodySHA256(request.Body),
		Labels: []string{"release", " backend ", "backend", "invalid\x00label"}, Assignees: []string{"owner"}, Draft: true, State: "opened",
	}
	if err := ValidateCandidate(request, candidate, candidate.URL); err != nil {
		t.Fatalf("matching candidate rejected: %v", err)
	}
	candidate.HeadSHA = strings.Repeat("b", 40)
	if err := ValidateCandidate(request, candidate, candidate.URL); err == nil || !strings.Contains(err.Error(), "exact durable intent") {
		t.Fatalf("different head accepted: %v", err)
	}
	candidate.HeadSHA = request.ExpectedHeadSHA
	if err := ValidateCandidate(request, candidate, "https://gitlab.com/group/project/-/merge_requests/2"); err == nil || !strings.Contains(err.Error(), "known URL") {
		t.Fatalf("different known URL accepted: %v", err)
	}
}

func TestCandidateDraftIdentityKeepsSettledArtifacts(t *testing.T) {
	for _, test := range []struct {
		candidate contract.Candidate
		want      string
	}{
		{contract.Candidate{Title: "Draft: Ship the fix", Draft: true}, "Ship the fix"},
		{contract.Candidate{Title: "WIP: Ship the fix", Draft: true}, "Ship the fix"},
		{contract.Candidate{Title: "Ship the fix", Draft: true}, "Ship the fix"},
		{contract.Candidate{Title: "Draft: Ship the fix"}, "Draft: Ship the fix"},
	} {
		if got := CandidateTitle(test.candidate); got != test.want {
			t.Fatalf("candidate title = %q, want %q", got, test.want)
		}
	}
	for _, test := range []struct {
		candidate contract.Candidate
		expected  bool
		want      bool
	}{
		{contract.Candidate{Draft: true, State: "opened"}, true, true},
		{contract.Candidate{State: "opened"}, true, false},
		{contract.Candidate{State: "merged"}, true, true},
		{contract.Candidate{State: "closed"}, true, true},
		{contract.Candidate{Draft: true, State: "merged"}, false, false},
		{contract.Candidate{}, true, false},
	} {
		if got := CandidateDraftMatches(test.candidate, test.expected); got != test.want {
			t.Fatalf("draft match = %v, want %v for %+v", got, test.want, test.candidate)
		}
	}
}
