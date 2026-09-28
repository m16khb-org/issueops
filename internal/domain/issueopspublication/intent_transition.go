package issueopspublication

import (
	"fmt"
	"reflect"
	"strings"

	contract "issueops/internal/contract/issueopspublication"
)

type IntentCheckpoint string

const (
	CheckpointRetry      IntentCheckpoint = "retry CAS"
	CheckpointFailure    IntentCheckpoint = "failure receipt"
	CheckpointReceipt    IntentCheckpoint = "remote receipt CAS"
	CheckpointNotInvoked IntentCheckpoint = "terminal pre-invocation receipt"
)

type PendingIntentFacts struct {
	Prepared            bool
	Pending             bool
	OperationID         string
	ExpectedOperationID string
}

func ValidatePendingIntent(facts PendingIntentFacts, checkpoint IntentCheckpoint) error {
	if !facts.Prepared || !facts.Pending || facts.OperationID != facts.ExpectedOperationID {
		return fmt.Errorf("external intent changed before %s", checkpoint)
	}
	return nil
}

func ValidatePayloadUnchanged(stored, expected contract.IntentPayload, checkpoint IntentCheckpoint) error {
	if !reflect.DeepEqual(stored, expected) {
		return fmt.Errorf("external intent payload changed before %s", checkpoint)
	}
	return nil
}

func RetryPayload(payload contract.IntentPayload) contract.IntentPayload {
	payload.InvocationState = string(contract.InvocationUnknown)
	payload.RetryCount++
	return payload
}

func FailurePayload(payload contract.IntentPayload, invocation string, retryCount int, knownURL string) contract.IntentPayload {
	payload.InvocationState, payload.RetryCount = invocation, retryCount
	if strings.TrimSpace(knownURL) != "" {
		payload.KnownURL = strings.TrimSpace(knownURL)
	}
	return payload
}
