package issueops

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestValidateIssueCreateIntentInvariants(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    string
		url       string
		failure   string
		wantError string
	}{
		{"pending", issueopscontract.IssueCreateIntentPending, "", "", ""},
		{"completed", issueopscontract.IssueCreateIntentCompleted, "https://example.test/1", "", ""},
		{"failed", issueopscontract.IssueCreateIntentVerificationFailed, "", "reason", ""},
		{"completed missing url", issueopscontract.IssueCreateIntentCompleted, "", "", "completed issue create intent requires canonical_url"},
		{"pending with url", issueopscontract.IssueCreateIntentPending, "https://example.test/1", "", "issue create intent status pending must not have canonical_url"},
		{"failed missing reason", issueopscontract.IssueCreateIntentVerificationFailed, "", "", "issue create intent status verification_failed requires failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateIssueCreateIntentInvariants(issueopscontract.IssueOpsIssueCreateIntent{
				Status: test.status, CanonicalURL: test.url, Failure: test.failure,
			})
			if got := errorText(err); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
