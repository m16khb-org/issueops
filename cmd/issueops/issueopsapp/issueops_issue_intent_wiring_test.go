package issueopsapp

import (
	"strings"
	"testing"

	"issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestIssueCreateIntentCompositionLinksReadyCycle(t *testing.T) {
	root := t.TempDir()
	record, err := issueops.StartIssueOps(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "48-issue-intent-wiring"})
	if err != nil {
		t.Fatal(err)
	}
	record.Intent = &model.IssueOpsIntentContract{IntentClass: "trivial", RawRequest: "create issue", InterpretedIntent: "track work", SuccessCriteria: []string{"issue linked"}}
	if _, err := issueops.WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	request := model.IssueOpsIssueCreateIntentRequest{OperationID: "0123456789abcdef0123456789abcdef", Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "New issue", BodySHA256: strings.Repeat("a", 64), StartedAt: "2026-09-28T00:00:00Z"}
	if _, err := beginIssueCreateIntent(root, record.ID, request); err != nil {
		t.Fatal(err)
	}
	if _, err := recordIssueCreateOutcome(root, record.ID, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentInvokedUnknown, Failure: "response lost", ObservedAt: "2026-09-28T00:01:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := beginIssueCreateIntent(root, record.ID, request); err == nil {
		t.Fatal("ambiguous invocation created another intent")
	}
	if _, err := completeIssueCreateIntent(root, record.ID, "https://github.com/acme/repo/issues/48", "2026-09-28T00:02:00Z"); err != nil {
		t.Fatal(err)
	}
	stored, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Phase != model.IssueOpsPhasePlan || stored.IssueURL != "https://github.com/acme/repo/issues/48" || stored.IssueCreateIntent.Status != model.IssueCreateIntentCompleted || stored.IssueCreateIntent.Failure != "" {
		t.Fatalf("completion did not link the ready cycle: phase=%s url=%s intent=%+v", stored.Phase, stored.IssueURL, stored.IssueCreateIntent)
	}
}
