package issueops

import (
	"slices"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestSplitDecisionMissing(t *testing.T) {
	if got := splitDecisionMissing(issueopscontract.Record{}); !slices.Equal(got, []string{"split_decision"}) {
		t.Fatalf("empty record = %v", got)
	}
	if got := splitDecisionMissing(issueopscontract.Record{Decisions: []issueopscontract.Decision{{Kind: " Scope "}}}); len(got) != 0 {
		t.Fatalf("scope decision = %v", got)
	}
	if got := splitDecisionMissing(issueopscontract.Record{IssueLinks: []issueopscontract.IssueLink{{Type: "Child"}}}); len(got) != 0 {
		t.Fatalf("child link = %v", got)
	}
}

func TestDomainReviewMissing(t *testing.T) {
	if got := domainReviewMissing(issueopscontract.Record{}); !slices.Equal(got, []string{"domain_review"}) {
		t.Fatalf("empty record = %v", got)
	}
	if got := domainReviewMissing(issueopscontract.Record{DomainReview: &issueopscontract.DomainReview{ReviewedAt: "now"}}); len(got) != 0 {
		t.Fatalf("reviewed record = %v", got)
	}
}

func TestTargetBranchMatchMissing(t *testing.T) {
	if got := TargetBranchMatchMissing(issueopscontract.Record{}); len(got) != 0 {
		t.Fatalf("without artifact = %v", got)
	}
	record := issueopscontract.Record{
		BranchPrepare:  &issueopscontract.BranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &issueopscontract.RemoteArtifactVerification{TargetBranch: "other"},
	}
	if got := TargetBranchMatchMissing(record); !slices.Equal(got, []string{"target_branch_match"}) {
		t.Fatalf("mismatched branch = %v", got)
	}
	record.RemoteArtifact.TargetBranch = "main"
	if got := TargetBranchMatchMissing(record); len(got) != 0 {
		t.Fatalf("matching branch = %v", got)
	}
}
