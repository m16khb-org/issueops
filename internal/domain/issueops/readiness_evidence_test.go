package issueops

import (
	"slices"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestSplitDecisionMissing(t *testing.T) {
	if got := SplitDecisionMissing(issueopscontract.IssueOpsRecord{}); !slices.Equal(got, []string{"split_decision"}) {
		t.Fatalf("empty record = %v", got)
	}
	if got := SplitDecisionMissing(issueopscontract.IssueOpsRecord{Decisions: []issueopscontract.IssueOpsDecision{{Kind: " Scope "}}}); len(got) != 0 {
		t.Fatalf("scope decision = %v", got)
	}
	if got := SplitDecisionMissing(issueopscontract.IssueOpsRecord{IssueLinks: []issueopscontract.IssueOpsIssueLink{{Type: "Child"}}}); len(got) != 0 {
		t.Fatalf("child link = %v", got)
	}
}

func TestDomainReviewMissing(t *testing.T) {
	if got := DomainReviewMissing(issueopscontract.IssueOpsRecord{}); !slices.Equal(got, []string{"domain_review"}) {
		t.Fatalf("empty record = %v", got)
	}
	if got := DomainReviewMissing(issueopscontract.IssueOpsRecord{DomainReview: &issueopscontract.IssueOpsDomainReview{ReviewedAt: "now"}}); len(got) != 0 {
		t.Fatalf("reviewed record = %v", got)
	}
}

func TestTargetBranchMatchMissing(t *testing.T) {
	if got := TargetBranchMatchMissing(issueopscontract.IssueOpsRecord{}); len(got) != 0 {
		t.Fatalf("without artifact = %v", got)
	}
	record := issueopscontract.IssueOpsRecord{
		BranchPrepare:  &issueopscontract.IssueOpsBranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{TargetBranch: "other"},
	}
	if got := TargetBranchMatchMissing(record); !slices.Equal(got, []string{"target_branch_match"}) {
		t.Fatalf("mismatched branch = %v", got)
	}
	record.RemoteArtifact.TargetBranch = "main"
	if got := TargetBranchMatchMissing(record); len(got) != 0 {
		t.Fatalf("matching branch = %v", got)
	}
}
