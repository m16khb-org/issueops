package issueopspublication

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopspublication"
)

func TestIntentTransitionsPreserveOriginalRequest(t *testing.T) {
	original := contract.IntentPayload{OperationID: "op", Generation: 3, InvocationState: "not_invoked_proven", Request: contract.ProviderCreateRequest{Body: "body", Labels: []string{"bug"}}, KnownURL: "https://github.com/team/repo/pull/3"}
	retried := RetryPayload(original)
	if retried.RetryCount != 1 || retried.InvocationState != "unknown" || original.RetryCount != 0 || original.InvocationState != "not_invoked_proven" {
		t.Fatalf("retry=%+v", retried)
	}
	failed := FailurePayload(retried, "unknown", 2, "  ")
	if failed.KnownURL != original.KnownURL || failed.RetryCount != 2 {
		t.Fatalf("lost known URL: %+v", failed)
	}
	failed = FailurePayload(failed, "unknown", 2, " https://github.com/team/repo/pull/4 ")
	if failed.KnownURL != "https://github.com/team/repo/pull/4" {
		t.Fatalf("known URL=%q", failed.KnownURL)
	}
}

func TestIntentCheckpointsRejectStaleOperationAndPayload(t *testing.T) {
	payload := contract.IntentPayload{OperationID: "op", Generation: 1}
	for _, checkpoint := range []IntentCheckpoint{CheckpointRetry, CheckpointFailure, CheckpointReceipt, CheckpointNotInvoked} {
		if err := ValidatePendingIntent(PendingIntentFacts{Prepared: true, Pending: true, OperationID: "other", ExpectedOperationID: "op"}, checkpoint); err == nil || !strings.Contains(err.Error(), string(checkpoint)) {
			t.Fatalf("checkpoint %s: %v", checkpoint, err)
		}
		changed := payload
		changed.RetryCount = 1
		if err := ValidatePayloadUnchanged(changed, payload, checkpoint); err == nil {
			t.Fatalf("accepted changed payload at %s", checkpoint)
		}
		if err := ValidatePayloadUnchanged(payload, payload, checkpoint); err != nil {
			t.Fatal(err)
		}
	}
}
