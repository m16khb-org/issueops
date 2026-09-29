package remote

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestGateLedgerIssueNumberPrefersLinkedIssue(t *testing.T) {
	record := model.IssueOpsRecord{
		IssueURL:      "https://github.com/acme/repo/issues/21",
		BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/acme/repo/issues/22"},
	}
	if got := GateLedgerIssueNumber(record.IssueURL, record.BranchPrepare.IssueURL); got != "21" {
		t.Fatalf("linked issue number=%q", got)
	}
	record.IssueURL = ""
	if got := GateLedgerIssueNumber(record.IssueURL, record.BranchPrepare.IssueURL); got != "22" {
		t.Fatalf("branch preparation issue number=%q", got)
	}
}
