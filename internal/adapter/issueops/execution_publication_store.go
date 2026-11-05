package issueops

import (
	"context"
	"encoding/json"
	"fmt"

	"issueops/internal/adapter/outbound/sqlstore"
	application "issueops/internal/application/issueopsremote"
	"issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

// RemotePublicationStore persists remote publication intents and the cycle record in the
// state under StateRoot.
type RemotePublicationStore struct{ StateRoot string }

// WithinTransaction keeps the caller's request values (authority guard, trace)
// while still ignoring cancellation once a publication transition starts.
func (s RemotePublicationStore) WithinTransaction(ctx context.Context, id string, transition func(context.Context) error) error {
	return withIssueOpsLock(context.WithoutCancel(ctx), s.StateRoot, id, transition)
}
func (s RemotePublicationStore) Read(_ context.Context, id string) (issueops.Record, error) {
	return ReadIssueOps(s.StateRoot, id)
}

// RecordSnapshot returns a copy of the raw stored bytes of the cycle record id.
func (s RemotePublicationStore) RecordSnapshot(_ context.Context, id string) (contract.RecordSnapshot, error) {
	raw, err := readRemotePublicationRaw(s.StateRoot, issueOpsBucket, id)
	return contract.RecordSnapshot{ID: id, Raw: raw}, err
}

// PayloadRaw returns a copy of the raw stored bytes of the external intent operationID.
func (s RemotePublicationStore) PayloadRaw(_ context.Context, operationID string) ([]byte, error) {
	return readRemotePublicationRaw(s.StateRoot, externalIntentBucket, operationID)
}

// DecodeSnapshot decodes the cycle record and payload captured in intent.
func (s RemotePublicationStore) DecodeSnapshot(intent contract.Intent) (issueops.Record, contract.IntentPayload, error) {
	return publicationIntentSnapshot(intent)
}

// Persist saves the record transition together with the intent write or delete in a
// single transaction.
func (s RemotePublicationStore) Persist(ctx context.Context, record issueops.Record, intent contract.IntentMutation) (issueops.Record, error) {
	mutation := port.RecordMutation{Bucket: externalIntentBucket, ID: intent.OperationID, Delete: intent.Delete, RequireAbsent: intent.RequireAbsent}
	if !intent.Delete {
		data, err := json.Marshal(intent.Payload)
		if err != nil {
			return issueops.Record{}, err
		}
		mutation.Data = data
	}
	return persistExecutionTransitionWithMutations(ctx, s.StateRoot, record, nil, []port.RecordMutation{mutation})
}

// ReadPayload reads and validates the stored external intent payload for operationID. A
// missing row, undecodable JSON, or a payload with a foreign schema, operation ID,
// generation, provider or kind is an error.
func (s RemotePublicationStore) ReadPayload(_ context.Context, operationID string) (contract.IntentPayload, error) {
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return contract.IntentPayload{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return contract.IntentPayload{}, err
	}
	if !ok {
		return contract.IntentPayload{}, fmt.Errorf("external intent payload is missing")
	}
	var payload contract.IntentPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return contract.IntentPayload{}, fmt.Errorf("decode external intent payload: %w", err)
	}
	if payload.SchemaVersion != issueops.SchemaVersion || payload.OperationID != operationID || payload.Generation == 0 || payload.Provider == "" || payload.Kind == "" {
		return contract.IntentPayload{}, fmt.Errorf("external intent payload is invalid")
	}
	return payload, nil
}

func readRemotePublicationRaw(stateRoot, bucket, id string) ([]byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("remote publication raw row %s/%s is missing", bucket, id)
	}
	return append([]byte(nil), raw...), nil
}

var _ application.PublicationStore = RemotePublicationStore{}
