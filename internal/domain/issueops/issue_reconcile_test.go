package issueops

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestIssueReconcileRequiresUniqueCompleteSearch(t *testing.T) {
	for _, tc := range []struct {
		truncated bool
		count     int
		message   string
	}{
		{true, 1, "truncated"}, {false, 0, "found 0"}, {false, 2, "found 2"},
	} {
		if err := ValidateIssueReconcileSearch(tc.truncated, tc.count); err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("search %+v: %v", tc, err)
		}
	}
	if err := ValidateIssueReconcileSearch(false, 1); err != nil {
		t.Fatal(err)
	}
}

func TestIssueReconcileRequiresSealedProjectAndContent(t *testing.T) {
	body := "sealed body"
	digest := sha256.Sum256([]byte(body))
	intent := model.IssueOpsIssueCreateIntent{Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "Title", BodySHA256: fmt.Sprintf("%x", digest)}
	for _, tc := range []struct{ url, title, body, message string }{
		{"github.com/other/repo", "Title", body, "authority"},
		{"github.com/acme/repo", "Changed", body, "title and body digest"},
		{"github.com/acme/repo", "Title", "changed", "title and body digest"},
	} {
		if err := ValidateIssueReconcileCandidate(intent, tc.url, tc.title, tc.body); err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("candidate %+v: %v", tc, err)
		}
	}
	if err := ValidateIssueReconcileCandidate(intent, "github.com/acme/repo", "Title", body); err != nil {
		t.Fatal(err)
	}
}

func TestIssueReconcileRejectsMissingOrCompletedIntent(t *testing.T) {
	if err := ValidateIssueReconcileIntent(nil); err == nil {
		t.Fatal("missing intent accepted")
	}
	if err := ValidateIssueReconcileIntent(&model.IssueOpsIssueCreateIntent{Status: model.IssueCreateIntentCompleted}); err == nil {
		t.Fatal("completed intent accepted")
	}
	if err := ValidateIssueReconcileIntent(&model.IssueOpsIssueCreateIntent{Status: model.IssueCreateIntentInvokedUnknown}); err != nil {
		t.Fatal(err)
	}
}
