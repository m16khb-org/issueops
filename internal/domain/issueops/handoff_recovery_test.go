package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestDeliveryRecoveryRequiresExactDurableIdentity(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	for _, test := range []struct {
		sealed, observed string
		found            bool
		want             string
	}{
		{id, id, true, id}, {"", id, true, id}, {"", "", false, ""},
	} {
		got, err := DeliveryDurableRequestID(test.sealed, test.observed, test.found, "dispatch")
		if err != nil || got != test.want {
			t.Fatalf("got=%q err=%v", got, err)
		}
	}
	for _, test := range []struct {
		sealed, observed string
		found            bool
	}{
		{id, id, false}, {id, "", true}, {id, "22222222-2222-4222-8222-222222222222", true}, {"bad", id, true}, {"", "bad", true},
	} {
		if _, err := DeliveryDurableRequestID(test.sealed, test.observed, test.found, "dispatch"); err == nil {
			t.Fatalf("accepted %+v", test)
		}
	}
}

func TestStagedDeliveryCannotAuthorizeAnotherExternalCall(t *testing.T) {
	attempt := DeliveryAttempt{OperationID: "operation", Stage: "dispatch", LifecycleID: "io-test", Host: "codex", SourceGeneration: 3, PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64)}
	observation := attempt.Probe(model.IssueOpsHandoffDeliveryLauncher{}, "dispatch")
	observation.AttemptID = attempt.AttemptID("dispatch")
	observation.CallStaged = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateObserved}
	if err := RejectStagedDeliveryWithoutResponse(attempt, []model.IssueOpsHandoffDeliveryObservation{observation}); err == nil {
		t.Fatal("staged response loss accepted")
	}
	observation.Ambiguous = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateObserved}
	if err := RejectStagedDeliveryWithoutResponse(attempt, []model.IssueOpsHandoffDeliveryObservation{observation}); err != nil {
		t.Fatal(err)
	}
}
