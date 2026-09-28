package issueops

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

func TestIssueCreateIntentPersistsBeforeMutationAndBlocksConcurrentBegin(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "42-durable-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "11111111111111111111111111111111",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Durable issue",
		BodySHA256:       strings.Repeat("a", 64),
		Labels:           []string{"quality"},
		Assignees:        []string{"owner"},
		StartedAt:        "2026-08-14T00:00:00Z",
	}

	updated, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IssueCreateIntent == nil || updated.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("intent = %+v", updated.IssueCreateIntent)
	}
	if !strings.Contains(updated.IssueCreateIntent.Marker, request.OperationID) {
		t.Fatalf("marker = %q", updated.IssueCreateIntent.Marker)
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request); err == nil {
		t.Fatal("concurrent begin must fail while outcome is pending")
	}
}

func TestIssueCreateIntentRetriesOnlyProvenNonInvocation(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "43-retry-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "22222222222222222222222222222222",
		Provider:         "gitlab",
		ProjectAuthority: "gitlab.example.com/acme/repo",
		Title:            "Retryable issue",
		BodySHA256:       strings.Repeat("b", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request); err != nil {
		t.Fatal(err)
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Outcome(context.Background(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentNotInvoked,
		Failure:    "process start failed",
		ObservedAt: "2026-08-14T00:01:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	retried, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request)
	if err != nil {
		t.Fatalf("proven non-invocation must permit retry: %v", err)
	}
	if retried.IssueCreateIntent.Attempt != 2 || retried.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("retried intent = %+v", retried.IssueCreateIntent)
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Outcome(context.Background(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentInvokedUnknown,
		Failure:    "timeout after process start",
		ObservedAt: "2026-08-14T00:02:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Outcome(context.Background(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentNotInvoked,
		Failure:    "invalid downgrade",
		ObservedAt: "2026-08-14T00:03:00Z",
	}); err == nil {
		t.Fatal("invoked_unknown must never transition back to not_invoked")
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request); err == nil {
		t.Fatal("ambiguous invocation must block retry")
	}
}

func TestCompleteIssueCreateIntentLinksCanonicalURLAtomically(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "44-complete-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "33333333333333333333333333333333",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Complete issue",
		BodySHA256:       strings.Repeat("c", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request); err != nil {
		t.Fatal(err)
	}
	const issueURL = "https://github.com/acme/repo/issues/44"
	completed, err := newIssueCreateIntentsForTest(stateRoot).Complete(context.Background(), record.ID, issueURL, "2026-08-14T00:03:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if completed.IssueURL != issueURL || completed.IssueCreateIntent == nil {
		t.Fatalf("completed record = %+v", completed)
	}
	if completed.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted ||
		completed.IssueCreateIntent.CanonicalURL != issueURL {
		t.Fatalf("completed intent = %+v", completed.IssueCreateIntent)
	}
	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.IssueURL != issueURL || reloaded.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		t.Fatalf("reloaded record = %+v", reloaded)
	}
}

func TestCompleteIssueCreateIntentRejectsDifferentProjectAuthority(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "45-authority-mismatch"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "0123456789abcdef0123456789abcdef",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Authority-bound issue",
		BodySHA256:       strings.Repeat("d", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := newIssueCreateIntentsForTest(stateRoot).Begin(context.Background(), record.ID, request); err != nil {
		t.Fatal(err)
	}

	_, err = newIssueCreateIntentsForTest(stateRoot).Complete(context.Background(), record.ID, "https://github.com/other/repo/issues/45", "2026-08-14T00:01:00Z")

	if err == nil || !strings.Contains(err.Error(), "project authority") {
		t.Fatalf("error = %v, want project authority mismatch", err)
	}
	reloaded, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if reloaded.IssueURL != "" || reloaded.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("mismatched authority changed durable state: %+v", reloaded)
	}
}

func newIssueCreateIntentsForTest(root string) *remoteapp.IssueCreateIntents {
	return remoteapp.NewIssueCreateIntents(IssueCreateIntentStore{StateRoot: root}, func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 4, time.UTC) })
}

func TestIssueCreateIntentRejectedRetryPreservesRawState(t *testing.T) {
	root := t.TempDir()
	record, err := StartIssueOps(root, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "46-sealed-retry"})
	if err != nil {
		t.Fatal(err)
	}
	service := newIssueCreateIntentsForTest(root)
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{OperationID: "0123456789abcdef0123456789abcdef", Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "sealed title", BodySHA256: strings.Repeat("a", 64), StartedAt: "2026-09-28T00:00:00Z"}
	if _, err := service.Begin(context.Background(), record.ID, request); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Outcome(context.Background(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{Status: issueopscontract.IssueCreateIntentNotInvoked, Failure: "not started", ObservedAt: "2026-09-28T00:01:00Z"}); err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before, exists, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !exists {
		t.Fatalf("initial row: %v %v", exists, err)
	}
	request.Title = "changed title"
	if _, err := service.Begin(context.Background(), record.ID, request); err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("changed sealed request accepted: %v", err)
	}
	if _, err := service.Complete(context.Background(), record.ID, "https://github.com/acme/repo/issues/46", "2026-09-28T00:02:00Z"); err == nil || !strings.Contains(err.Error(), "not invoked") {
		t.Fatalf("not-invoked completion accepted: %v", err)
	}
	after, exists, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !exists || !bytes.Equal(before, after) {
		t.Fatal("rejected mutation changed stored intent")
	}
}

func TestIssueCreateIntentConcurrentBeginsPersistOnlyOneAttempt(t *testing.T) {
	root := t.TempDir()
	record, err := StartIssueOps(root, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "47-concurrent-intents"})
	if err != nil {
		t.Fatal(err)
	}
	service := newIssueCreateIntentsForTest(root)
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{OperationID: "0123456789abcdef0123456789abcdef", Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "one issue", BodySHA256: strings.Repeat("a", 64), StartedAt: "2026-09-28T00:00:00Z"}
	start, results := make(chan struct{}), make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func() {
			<-start
			_, err := service.Begin(context.Background(), record.ID, request)
			results <- err
		}()
	}
	close(start)
	successes := 0
	for i := 0; i < 6; i++ {
		if err := <-results; err == nil {
			successes++
		} else if !strings.Contains(err.Error(), "reconcile before retry") {
			t.Fatalf("concurrent begin: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful starts=%d, want 1", successes)
	}
	stored, err := ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IssueCreateIntent.Attempt != 1 || stored.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending || stored.UpdatedAt != "2026-09-28T01:02:03.000000004Z" {
		t.Fatalf("stored=%+v", stored)
	}
}

func TestIssueCreateIntentFailedCompletionDoesNotPartiallyLink(t *testing.T) {
	root := t.TempDir()
	record, err := StartIssueOps(root, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "47-concurrent-intents"})
	if err != nil {
		t.Fatal(err)
	}
	service := newIssueCreateIntentsForTest(root)
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{OperationID: "0123456789abcdef0123456789abcdef", Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "one issue", BodySHA256: strings.Repeat("a", 64), StartedAt: "2026-09-28T00:00:00Z"}
	if _, err := service.Begin(context.Background(), record.ID, request); err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before, exists, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !exists {
		t.Fatalf("initial row: %v %v", exists, err)
	}
	if _, err := service.Complete(context.Background(), record.ID, "https://github.com/acme/repo/issues/47", strings.Repeat("x", 65)); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("invalid completion encoded: %v", err)
	}
	after, exists, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !exists || !bytes.Equal(before, after) {
		t.Fatal("failed completion encoding partially linked the issue")
	}
}
