package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func issueCreateRequestForTest() model.IssueCreateIntentRequest {
	return model.IssueCreateIntentRequest{OperationID: "0123456789abcdef0123456789abcdef", Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "Issue creation", BodySHA256: strings.Repeat("a", 64), Labels: []string{"quality"}, StartedAt: "2026-09-28T00:00:00Z"}
}

func TestIssueCreateLifecycleRequiresProvenNonInvocationAndSealedRequest(t *testing.T) {
	request := issueCreateRequestForTest()
	started, err := BeginIssueCreateIntent(model.Record{}, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BeginIssueCreateIntent(started, request); err == nil {
		t.Fatal("pending intent was retried")
	}
	notInvoked, err := RecordIssueCreateOutcome(started, model.IssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "not started", ObservedAt: "2026-09-28T00:01:00Z"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if started.IssueCreateIntent.Status != model.IssueCreateIntentPending {
		t.Fatal("outcome mutated its input")
	}
	changed := request
	changed.Title = "changed request"
	if _, err := BeginIssueCreateIntent(notInvoked, changed); err == nil {
		t.Fatal("changed sealed request accepted")
	}
	retried, err := BeginIssueCreateIntent(notInvoked, request)
	if err != nil {
		t.Fatal(err)
	}
	if retried.IssueCreateIntent.Attempt != 2 || retried.IssueCreateIntent.Status != model.IssueCreateIntentPending || notInvoked.IssueCreateIntent.Status != model.IssueCreateIntentNotInvoked {
		t.Fatalf("retry transition=%+v original=%+v", retried.IssueCreateIntent, notInvoked.IssueCreateIntent)
	}
	ambiguous, err := RecordIssueCreateOutcome(retried, model.IssueCreateOutcome{Status: model.IssueCreateIntentInvokedUnknown, Failure: "response lost", ObservedAt: "2026-09-28T00:02:00Z"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BeginIssueCreateIntent(ambiguous, request); err == nil {
		t.Fatal("ambiguous intent was retried")
	}
	if _, err := RecordIssueCreateOutcome(ambiguous, model.IssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "downgrade", ObservedAt: "now"}, ""); err == nil {
		t.Fatal("ambiguous intent downgraded")
	}
}

func TestIssueCreateCompletionBindsAuthorityAndAdvancesReadyPlan(t *testing.T) {
	request := issueCreateRequestForTest()
	for _, ready := range []bool{false, true} {
		original := model.Record{Phase: model.PhaseGrill}
		if ready {
			original.Intent = &model.IntentContract{IntentClass: "trivial", RawRequest: "create issue", InterpretedIntent: "record work", SuccessCriteria: []string{"linked issue"}}
		}
		pending, err := BeginIssueCreateIntent(original, request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CompleteIssueCreateIntent(pending, "https://github.com/other/repo/issues/1", "now", "github.com/other/repo"); err == nil {
			t.Fatal("foreign project linked")
		}
		completed, err := CompleteIssueCreateIntent(pending, " https://github.com/acme/repo/issues/1 ", "2026-09-28T00:03:00Z", request.ProjectAuthority)
		if err != nil {
			t.Fatal(err)
		}
		if completed.IssueURL != "https://github.com/acme/repo/issues/1" || completed.IssueCreateIntent.Status != model.IssueCreateIntentCompleted || pending.IssueURL != "" || pending.IssueCreateIntent.Status != model.IssueCreateIntentPending {
			t.Fatal("completion changed input or lost canonical link")
		}
		wantPhase := model.PhaseGrill
		if ready {
			wantPhase = model.PhasePlan
		}
		if completed.Phase != wantPhase {
			t.Fatalf("phase=%s want=%s", completed.Phase, wantPhase)
		}
		if _, err := CompleteIssueCreateIntent(completed, completed.IssueURL, "later", request.ProjectAuthority); err == nil {
			t.Fatal("completed intent completed twice")
		}
	}
}

func TestIssueCreateOutcomeCarriesFailureCause(t *testing.T) {
	request := issueCreateRequestForTest()
	started, err := BeginIssueCreateIntent(model.Record{}, request)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := RecordIssueCreateOutcome(started, model.IssueCreateOutcome{
		Status: model.IssueCreateIntentInvokedUnknown, Failure: "x", FailureCommand: "gh issue create",
		FailureHTTPStatus: 404, ObservedAt: "2026-09-28T00:01:00Z",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := unknown.IssueCreateIntent; got.Failure != "x" || got.FailureCommand != "gh issue create" || got.FailureHTTPStatus != 404 {
		t.Fatalf("invoked_unknown cause = %q %q %d", got.Failure, got.FailureCommand, got.FailureHTTPStatus)
	}

	completed, err := CompleteIssueCreateIntent(unknown, "https://github.com/acme/repo/issues/1", "2026-09-28T00:03:00Z", request.ProjectAuthority)
	if err != nil {
		t.Fatal(err)
	}
	if got := completed.IssueCreateIntent; got.Status != model.IssueCreateIntentCompleted || got.Failure != "" || got.FailureCommand != "" || got.FailureHTTPStatus != 0 {
		t.Fatalf("completed kept cause = %q %q %d", got.Failure, got.FailureCommand, got.FailureHTTPStatus)
	}

	for _, row := range []struct {
		name, command string
		status        int
		wantCommand   string
		wantStatus    int
	}{
		{"unknown command and status", "curl", 302, "", 0},
		{"status above range", "gh issue create", 600, "gh issue create", 0},
	} {
		recorded, err := RecordIssueCreateOutcome(started, model.IssueCreateOutcome{
			Status: model.IssueCreateIntentInvokedUnknown, Failure: "x", FailureCommand: row.command,
			FailureHTTPStatus: row.status, ObservedAt: "2026-09-28T00:01:00Z",
		}, "")
		if err != nil {
			t.Fatalf("%s: %v", row.name, err)
		}
		if got := recorded.IssueCreateIntent; got.Failure != "x" || got.FailureCommand != row.wantCommand || got.FailureHTTPStatus != row.wantStatus {
			t.Fatalf("%s: cause = %q %q %d", row.name, got.Failure, got.FailureCommand, got.FailureHTTPStatus)
		}
	}

	notInvoked, err := RecordIssueCreateOutcome(started, model.IssueCreateOutcome{
		Status: model.IssueCreateIntentNotInvoked, Failure: "not started", FailureCommand: "gh issue create",
		FailureHTTPStatus: 403, ObservedAt: "2026-09-28T00:01:00Z",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := notInvoked.IssueCreateIntent; got.FailureCommand != "gh issue create" || got.FailureHTTPStatus != 403 {
		t.Fatalf("not_invoked cause = %q %d", got.FailureCommand, got.FailureHTTPStatus)
	}
	retried, err := BeginIssueCreateIntent(notInvoked, request)
	if err != nil {
		t.Fatal(err)
	}
	if got := retried.IssueCreateIntent; got.Failure != "" || got.FailureCommand != "" || got.FailureHTTPStatus != 0 {
		t.Fatalf("retry kept cause = %q %q %d", got.Failure, got.FailureCommand, got.FailureHTTPStatus)
	}
}
