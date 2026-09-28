package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

type orcaFailureState struct {
	Record      leasecontract.Record
	RecordRaw   []byte
	IntentRaw   []byte
	OperationID string
}

func recordOrcaIntentFailure(ctx context.Context, store port.RecordRawCASStore, state orcaFailureState, invocation string, cause error, now func() time.Time, redact func(string) string) error {
	if len(state.RecordRaw) == 0 || len(state.IntentRaw) == 0 {
		return fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	codec := preparationcontract.IntentCodec{}
	current, err := codec.Decode(state.OperationID, state.IntentRaw)
	if err != nil {
		return err
	}
	if now == nil {
		now = time.Now
	}
	intentState := preparationapp.IntentState{Snapshot: preparationcontract.Snapshot{Record: state.Record, RecordRaw: state.RecordRaw}, Intent: current, IntentRaw: state.IntentRaw, FailureAt: now().UTC().Format(time.RFC3339Nano)}
	if err := preparationapp.ValidateIntentState(intentState); err != nil {
		return err
	}
	record, updated, err := preparationapp.ApplyOrcaFailure(intentState, invocation, func() string { return boundedOrcaFailureDiagnostic(cause, redact) })
	if err != nil {
		return err
	}
	if typed, ok := errors.AsType[*port.OrcaError](cause); ok {
		updated.OrcaRequestID, updated.OrcaPromptRequestID = preparationdomain.AdoptFailureRequestIDs(preparationdomain.FailureRequestIDFacts{
			SealedDispatch: updated.OrcaRequestID, SealedPrompt: updated.OrcaPromptRequestID,
			CallPhase: typed.CallPhase, ObservedDispatch: typed.DispatchRequestID,
			ObservedOrchestration: typed.OrchestrationRequestID,
			DispatchValid:         port.ValidateOrcaRequestID(typed.DispatchRequestID) == nil,
			OrchestrationValid:    port.ValidateOrcaRequestID(typed.OrchestrationRequestID) == nil,
		})
	}
	recordData, err := recordcodec.EncodeLease(record)
	if err != nil {
		return err
	}
	intentData, err := codec.Encode(updated)
	if err != nil {
		return err
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: record.ID, Data: state.RecordRaw},
		{Bucket: "external_intent_v1", ID: state.OperationID, Data: state.IntentRaw},
	}, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordData},
		{Bucket: "external_intent_v1", ID: state.OperationID, Data: intentData},
	})
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return fmt.Errorf("stale raw record snapshot")
		}
		return fmt.Errorf("stale raw intent snapshot")
	}
	return err
}

func boundedOrcaFailureDiagnostic(cause error, redact func(string) string) string {
	message := "external operation failed"
	if redact != nil && cause != nil {
		message = strings.TrimSpace(redact(cause.Error()))
		if message == "" {
			message = "external operation failed"
		}
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}
