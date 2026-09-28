package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
	"issueops/internal/port"
)

func TestRemoteCandidateValidationDelegatesExactIntentAndChecksProject(t *testing.T) {
	const url = "https://github.com/acme/repo/pull/1"
	record := issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/42"}
	payload := publicationcontract.IntentPayload{
		Provider: "github", Kind: "pr", KnownURL: url,
		Request: publicationcontract.ProviderCreateRequest{
			ProjectKey: "acme/repo", Title: "Ship the fix", Body: "body", HeadBranch: "work", BaseBranch: "main",
			ExpectedHeadSHA: strings.Repeat("a", 40), Labels: []string{"backend"}, Assignees: []string{"owner"}, Draft: true,
		},
	}
	candidate := port.IssueProviderReconcilePullRequestCandidate{
		URL: url, ProjectKey: "acme/repo", SourceProjectKey: "acme/repo", HeadBranch: "work", BaseBranch: "main",
		HeadSHA: strings.Repeat("a", 40), Title: "Draft: Ship the fix", BodySHA256: publicationdomain.BodySHA256("body"),
		Labels: []string{"backend"}, Assignees: []string{"owner"}, Draft: true, State: "opened",
	}
	if err := validateRemotePullRequestCandidate(record, payload, candidate); err != nil {
		t.Fatalf("matching candidate rejected: %v", err)
	}
	candidate.URL = "https://github.com/other/repo/pull/1"
	payload.KnownURL = ""
	if err := validateRemotePullRequestCandidate(record, payload, candidate); err == nil {
		t.Fatal("candidate from another project accepted")
	}
}
