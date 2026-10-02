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

type RemotePublicationStore struct{ StateRoot string }

// WithinTransaction keeps the caller's request values (authority guard, trace)
// while still ignoring cancellation once a publication transition starts.
func (s RemotePublicationStore) WithinTransaction(ctx context.Context, id string, transition func(context.Context) error) error {
	return withIssueOpsLock(context.WithoutCancel(ctx), s.StateRoot, id, transition)
}
func (s RemotePublicationStore) Read(_ context.Context, id string) (issueops.IssueOpsRecord, error) {
	return ReadIssueOps(s.StateRoot, id)
}
func (s RemotePublicationStore) RecordSnapshot(_ context.Context, id string) (contract.RecordSnapshot, error) {
	raw, err := readRemotePublicationRaw(s.StateRoot, issueOpsBucket, id)
	return contract.RecordSnapshot{ID: id, Raw: raw}, err
}
func (s RemotePublicationStore) PayloadRaw(_ context.Context, operationID string) ([]byte, error) {
	return readRemotePublicationRaw(s.StateRoot, externalIntentBucket, operationID)
}
func (s RemotePublicationStore) DecodeSnapshot(intent contract.Intent) (issueops.IssueOpsRecord, contract.IntentPayload, error) {
	return publicationIntentSnapshot(intent)
}
func (s RemotePublicationStore) Persist(ctx context.Context, record issueops.IssueOpsRecord, intent contract.IntentMutation) (issueops.IssueOpsRecord, error) {
	mutation := port.RecordMutation{Bucket: externalIntentBucket, ID: intent.OperationID, Delete: intent.Delete, RequireAbsent: intent.RequireAbsent}
	if !intent.Delete {
		data, err := json.Marshal(intent.Payload)
		if err != nil {
			return issueops.IssueOpsRecord{}, err
		}
		mutation.Data = data
	}
	return persistExecutionTransitionWithMutations(ctx, s.StateRoot, record, nil, []port.RecordMutation{mutation})
}

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
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID != operationID || payload.Generation == 0 || payload.Provider == "" || payload.Kind == "" {
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
