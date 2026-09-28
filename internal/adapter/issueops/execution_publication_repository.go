package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	application "issueops/internal/application/issueopspublication"
	"issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

type RemotePublicationRepository struct {
	stateRoot      string
	now            func() time.Time
	newOperationID func() (string, error)
}

func NewRemotePublicationRepository(stateRoot string, now func() time.Time, newOperationID func() (string, error)) *RemotePublicationRepository {
	if newOperationID == nil {
		newOperationID = newExecutionOperationID
	}
	return &RemotePublicationRepository{stateRoot: stateRoot, now: now, newOperationID: newOperationID}
}

func (r *RemotePublicationRepository) BeginCreate(_ context.Context, prepared contract.PreparedCreate) (contract.Intent, error) {
	operationID, err := r.newOperationID()
	if err != nil {
		return contract.Intent{}, err
	}
	command := prepared.Command
	pending, payload, err := beginRemotePullRequestIntentWithOperationID(
		r.stateRoot, command.ID, publicationActor(command.Actor), command.CWD, command.ExpectedGeneration,
		port.IssueProviderCreatePullRequestRequest(prepared.Request.Clone()), command.Provider, prepared.Eligibility.Kind, operationID, r.now,
	)
	if err != nil {
		return contract.Intent{}, err
	}
	return r.loadSnapshot(pending, payload, prepared.Eligibility)
}

func (r *RemotePublicationRepository) LoadIntent(_ context.Context, id string) (contract.Intent, error) {
	record, err := ReadIssueOps(r.stateRoot, id)
	if err != nil {
		return contract.Intent{}, err
	}
	if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.Kind != externalIntentRemotePR {
		return contract.Intent{}, fmt.Errorf("remote publication intent is not pending")
	}
	payload, err := readExternalRemotePRPayload(r.stateRoot, record.Execution.Pending.OperationID)
	if err != nil {
		return contract.Intent{}, err
	}
	eligibility := publicationEligibility(record, payload.Provider, payload.Kind, payload.Request)
	eligibility.NoPending = false
	return r.loadSnapshot(record, payload, eligibility)
}

func (r *RemotePublicationRepository) MarkRetry(_ context.Context, intent contract.Intent) (contract.Intent, error) {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return contract.Intent{}, err
	}
	payload, err = markRemotePullRequestRetry(r.stateRoot, record.ID, payload)
	if err != nil {
		return contract.Intent{}, err
	}
	record, err = ReadIssueOps(r.stateRoot, record.ID)
	if err != nil {
		return contract.Intent{}, err
	}
	return r.loadSnapshot(record, payload, intent.Eligibility)
}

func (r *RemotePublicationRepository) RecordFailure(_ context.Context, intent contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return err
	}
	return recordRemotePullRequestFailure(r.stateRoot, record.ID, payload.OperationID, string(invocation), payload.RetryCount, knownURL, cause, r.now)
}

func (r *RemotePublicationRepository) Complete(_ context.Context, intent contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	record, err = finishRemotePullRequestIntent(r.stateRoot, record.ID, payload, url, enforceOriginalGeneration, r.now)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return publicationRecordSnapshot(r.stateRoot, record.ID)
}

func (r *RemotePublicationRepository) CompleteNotInvoked(_ context.Context, intent contract.Intent, cause error) (contract.RecordSnapshot, error) {
	record, payload, err := publicationIntentSnapshot(intent)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	record, err = finishRemotePullRequestPreInvocationFailure(r.stateRoot, record.ID, payload, cause, r.now)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return publicationRecordSnapshot(r.stateRoot, record.ID)
}

func (r *RemotePublicationRepository) Latest(_ context.Context, id string) (contract.RecordSnapshot, error) {
	record, err := ReadIssueOps(r.stateRoot, id)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return publicationRecordSnapshot(r.stateRoot, record.ID)
}

func (r *RemotePublicationRepository) loadSnapshot(record issueops.IssueOpsRecord, payload externalRemotePRPayload, eligibility contract.CreateEligibility) (contract.Intent, error) {
	snapshot, err := publicationRecordSnapshot(r.stateRoot, record.ID)
	if err != nil {
		return contract.Intent{}, err
	}
	raw, err := readRemotePublicationRaw(r.stateRoot, externalIntentBucket, payload.OperationID)
	if err != nil {
		return contract.Intent{}, err
	}
	return contract.Intent{
		Record: snapshot, Raw: raw, OperationID: payload.OperationID, Generation: payload.Generation,
		Provider: payload.Provider, Kind: payload.Kind, Request: publicationRequest(payload.Request),
		InvocationState: contract.InvocationState(payload.InvocationState), RetryCount: payload.RetryCount,
		KnownURL: payload.KnownURL, Eligibility: eligibility,
	}, nil
}

func publicationRecordSnapshot(stateRoot, id string) (contract.RecordSnapshot, error) {
	raw, err := readRemotePublicationRaw(stateRoot, issueOpsBucket, id)
	return contract.RecordSnapshot{ID: id, Raw: raw}, err
}

func publicationIntentSnapshot(intent contract.Intent) (issueops.IssueOpsRecord, externalRemotePRPayload, error) {
	var record issueops.IssueOpsRecord
	if len(intent.Record.Raw) == 0 {
		return record, externalRemotePRPayload{}, fmt.Errorf("publication record raw bytes are required")
	}
	if err := json.Unmarshal(intent.Record.Raw, &record); err != nil {
		return record, externalRemotePRPayload{}, fmt.Errorf("decode publication record: %w", err)
	}
	if len(intent.Raw) == 0 {
		return record, externalRemotePRPayload{}, fmt.Errorf("remote publication intent raw bytes are required")
	}
	var payload externalRemotePRPayload
	if err := json.Unmarshal(intent.Raw, &payload); err != nil {
		return record, payload, fmt.Errorf("decode remote publication intent: %w", err)
	}
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID == "" || payload.OperationID != intent.OperationID || payload.Generation == 0 {
		return record, payload, fmt.Errorf("remote publication intent state is invalid")
	}
	return record, payload, nil
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

func publicationEligibility(record issueops.IssueOpsRecord, provider, kind string, request port.IssueProviderCreatePullRequestRequest) contract.CreateEligibility {
	return contract.CreateEligibility{
		Provider: strings.ToLower(strings.TrimSpace(provider)), Kind: kind, Confirm: request.Confirm,
		PhasePR:         record.Phase == issueops.IssueOpsPhasePR,
		ExecutionActive: record.Execution != nil && record.Execution.Lease.Status == issueops.LeaseStatusActive,
		NoPending:       record.Execution == nil || record.Execution.Pending == nil,
		NoArtifact:      record.RemoteArtifact == nil, BranchAuthority: true,
		CanonicalLabelsAssignees: len(request.Labels) > 0 && len(request.Assignees) > 0,
	}
}

var _ application.Repository = (*RemotePublicationRepository)(nil)
