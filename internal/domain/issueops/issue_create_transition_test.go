package issueops

import (
	issueopscontract "issueops/internal/contract/issueops"
	"testing"
)

func TestValidateIssueCreateTransitionRejectsRetryEnablingDowngrade(t *testing.T) {
	if err := ValidateIssueCreateTransition(issueopscontract.IssueCreateIntentPending, issueopscontract.IssueCreateIntentInvokedUnknown); err != nil {
		t.Fatalf("pending -> invoked_unknown: %v", err)
	}
	if err := ValidateIssueCreateTransition(issueopscontract.IssueCreateIntentInvokedUnknown, issueopscontract.IssueCreateIntentVerificationFailed); err != nil {
		t.Fatalf("invoked_unknown -> verification_failed: %v", err)
	}
	if err := ValidateIssueCreateTransition(issueopscontract.IssueCreateIntentInvokedUnknown, issueopscontract.IssueCreateIntentNotInvoked); err == nil {
		t.Fatal("invoked_unknown -> not_invoked must be rejected")
	}
	if err := ValidateIssueCreateTransition(issueopscontract.IssueCreateIntentVerificationFailed, issueopscontract.IssueCreateIntentInvokedUnknown); err == nil {
		t.Fatal("verification_failed -> invoked_unknown must be rejected")
	}
}
