package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestUmbrellaBranchGateReasonRequiresPreparedBranch(t *testing.T) {
	record := issueops.Record{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
	}
	reason := umbrellaBranchGateReason(record)
	if reason == "" {
		t.Fatal("umbrella cycle without branch_prepare must not be allowed to create child work items")
	}
	if !strings.Contains(reason, "branch prepare") {
		t.Fatalf("reason %q must name the command that resolves it", reason)
	}
}

func TestUmbrellaBranchGateReasonRequiresBranchIdentity(t *testing.T) {
	record := issueops.Record{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
		BranchPrepare: &issueops.BranchPrepare{
			Provider:   "github",
			IssueURL:   "https://github.com/acme/repo/issues/78",
			Branch:     "79-something-else",
			BaseBranch: "main",
		},
	}
	if reason := umbrellaBranchGateReason(record); reason == "" {
		t.Fatal("branch_prepare recorded for a different branch must not satisfy the umbrella gate")
	}
}

func TestUmbrellaBranchGateReasonAcceptsPreparedUmbrella(t *testing.T) {
	record := issueops.Record{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
		BranchPrepare: &issueops.BranchPrepare{
			Provider:   "github",
			IssueURL:   "https://github.com/acme/repo/issues/78",
			Branch:     "78-umbrella",
			BaseBranch: "main",
		},
	}
	if reason := umbrellaBranchGateReason(record); reason != "" {
		t.Fatalf("prepared umbrella branch must pass the gate, got %q", reason)
	}
}
