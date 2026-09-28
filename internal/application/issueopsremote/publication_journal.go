package issueopsremote

import (
	"context"
	"strings"

	publicationapp "issueops/internal/application/issueopspublication"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	cycledomain "issueops/internal/domain/issueops"
	domain "issueops/internal/domain/issueopspublication"
	remote "issueops/internal/domain/issueopsremote"
)

type PublicationJournal struct {
	store       PublicationStore
	environment PublicationEnvironment
	authority   PublicationAuthority
}

func NewPublicationJournal(store PublicationStore, environment PublicationEnvironment, authority PublicationAuthority) *PublicationJournal {
	return &PublicationJournal{store: store, environment: environment, authority: authority}
}

func (j *PublicationJournal) BeginCreate(ctx context.Context, prepared contract.PreparedCreate) (contract.Intent, error) {
	operationID, err := j.environment.NewOperationID()
	if err != nil {
		return contract.Intent{}, err
	}
	if err := domain.ValidateOperationID(operationID); err != nil {
		return contract.Intent{}, err
	}
	marker := "<!-- issueops:issueops-v1 operation=" + operationID + " -->"
	request := prepared.Request.Clone()
	request.Body = strings.TrimSpace(request.Body) + "\n\n" + marker
	payload := contract.IntentPayload{SchemaVersion: model.IssueOpsSchemaVersion, OperationID: operationID, Provider: prepared.Command.Provider, Kind: prepared.Eligibility.Kind, Request: request, InvocationState: string(contract.InvocationUnknown)}
	var persisted model.IssueOpsRecord
	err = j.store.WithinTransaction(ctx, prepared.Command.ID, func(tx context.Context) error {
		current, err := j.store.Read(tx, prepared.Command.ID)
		if err != nil {
			return err
		}
		if err := j.authority.Authorize(tx, current, publicationMutationActor(prepared.Command)); err != nil {
			return err
		}
		facts := domain.BeginAuthorityFacts{Prepared: current.Execution != nil, Artifact: current.RemoteArtifact != nil, ExpectedGeneration: prepared.Command.ExpectedGeneration}
		if current.Execution != nil {
			facts.Pending = current.Execution.Pending != nil
			facts.CurrentGeneration = current.Execution.Lease.Generation
		}
		if err := domain.ValidateBeginAuthority(facts); err != nil {
			return err
		}
		payload.Generation = current.Execution.Lease.Generation
		current = cycledomain.BeginPublication(current, model.ExternalIntent{OperationID: operationID, Kind: contract.RemoteIntentKind, Marker: marker, StartedAt: j.environment.Timestamp()})
		persisted, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: operationID, Payload: &payload, RequireAbsent: true})
		return err
	})
	if err != nil {
		return contract.Intent{}, err
	}
	return j.loadSnapshot(ctx, persisted, payload, prepared.Eligibility)
}

func (j *PublicationJournal) MarkRetry(ctx context.Context, intent contract.Intent) (contract.Intent, error) {
	record, expected, err := j.store.DecodeSnapshot(intent)
	if err != nil {
		return contract.Intent{}, err
	}
	updated := domain.RetryPayload(expected)
	err = j.store.WithinTransaction(ctx, record.ID, func(tx context.Context) error {
		current, err := j.store.Read(tx, record.ID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePendingIntent(pendingFacts(current, expected.OperationID), domain.CheckpointRetry); err != nil {
			return err
		}
		stored, err := j.store.ReadPayload(tx, expected.OperationID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePayloadUnchanged(stored, expected, domain.CheckpointRetry); err != nil {
			return err
		}
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: updated.OperationID, Payload: &updated})
		return err
	})
	if err != nil {
		return contract.Intent{}, err
	}
	record, err = j.store.Read(ctx, record.ID)
	if err != nil {
		return contract.Intent{}, err
	}
	return j.loadSnapshot(ctx, record, updated, intent.Eligibility)
}

func (j *PublicationJournal) RecordFailure(ctx context.Context, intent contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
	record, expected, err := j.store.DecodeSnapshot(intent)
	if err != nil {
		return err
	}
	return j.store.WithinTransaction(ctx, record.ID, func(tx context.Context) error {
		current, err := j.store.Read(tx, record.ID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePendingIntent(pendingFacts(current, expected.OperationID), domain.CheckpointFailure); err != nil {
			return err
		}
		payload, err := j.store.ReadPayload(tx, expected.OperationID)
		if err != nil {
			return err
		}
		payload = domain.FailurePayload(payload, string(invocation), expected.RetryCount, knownURL)
		current = cycledomain.FailPublication(current, expected.OperationID, remote.PublicationFailureDiagnostic(cause), j.environment.Timestamp(), false)
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Payload: &payload})
		return err
	})
}

func (j *PublicationJournal) Complete(ctx context.Context, intent contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
	record, payload, err := j.store.DecodeSnapshot(intent)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	err = j.store.WithinTransaction(ctx, record.ID, func(tx context.Context) error {
		current, err := j.store.Read(tx, record.ID)
		if err != nil {
			return err
		}
		facts := domain.ReceiptAuthorityFacts{Prepared: current.Execution != nil, ExpectedOperationID: payload.OperationID, ExpectedGeneration: payload.Generation}
		if current.Execution != nil {
			facts.Pending = current.Execution.Pending != nil
			if facts.Pending {
				facts.PendingOperationID = current.Execution.Pending.OperationID
			}
			lease := current.Execution.Lease
			facts.Generation, facts.LeaseStatus = lease.Generation, string(lease.Status)
			facts.ExpectedHost, facts.ExpectedSessionID, facts.ExpectedAgentID = payload.Request.Host, payload.Request.SessionID, payload.Request.AgentID
			if enforceOriginalGeneration && facts.Pending && facts.PendingOperationID == payload.OperationID {
				facts.CWDMatches = j.environment.PathsMatch(payload.Request.CWD, current.Execution.Workspace.Root)
			}
			if lease.Holder != nil {
				facts.HolderPresent = true
				facts.HolderHost, facts.HolderSessionID, facts.HolderAgentID = lease.Holder.Host, lease.Holder.SessionID, lease.Holder.AgentID
			}
		}
		if err := domain.ValidateReceiptAuthority(facts, enforceOriginalGeneration); err != nil {
			return err
		}
		stored, err := j.store.ReadPayload(tx, payload.OperationID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePayloadUnchanged(stored, payload, domain.CheckpointReceipt); err != nil {
			return err
		}
		authority := remote.ArtifactAuthority{Phase: string(current.Phase), IssueURL: current.IssueURL}
		if current.BranchPrepare != nil {
			authority.CodeProjectKey = current.BranchPrepare.CodeProjectKey
		}
		artifact, err := remote.ProjectArtifact(authority, remote.Artifact{Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url), Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch})
		if err != nil {
			return err
		}
		current = cycledomain.CompletePublication(current, model.IssueOpsRemoteArtifactVerification{Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL, Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch, VerifiedAt: j.environment.Timestamp()})
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Delete: true})
		return err
	})
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

func (j *PublicationJournal) CompleteNotInvoked(ctx context.Context, intent contract.Intent, cause error) (contract.RecordSnapshot, error) {
	record, payload, err := j.store.DecodeSnapshot(intent)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	err = j.store.WithinTransaction(ctx, record.ID, func(tx context.Context) error {
		current, err := j.store.Read(tx, record.ID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePendingIntent(pendingFacts(current, payload.OperationID), domain.CheckpointNotInvoked); err != nil {
			return err
		}
		stored, err := j.store.ReadPayload(tx, payload.OperationID)
		if err != nil {
			return err
		}
		if err := domain.ValidatePayloadUnchanged(stored, payload, domain.CheckpointNotInvoked); err != nil {
			return err
		}
		current = cycledomain.FailPublication(current, payload.OperationID, remote.PublicationFailureDiagnostic(cause), j.environment.Timestamp(), true)
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Delete: true})
		return err
	})
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

func (j *PublicationJournal) LoadIntent(ctx context.Context, id string) (contract.Intent, error) {
	record, err := j.store.Read(ctx, id)
	if err != nil {
		return contract.Intent{}, err
	}
	facts := pendingFacts(record, "")
	kind := ""
	if facts.Pending {
		kind = record.Execution.Pending.Kind
	}
	if err := domain.ValidatePublicationPending(facts.Prepared, facts.Pending, kind); err != nil {
		return contract.Intent{}, err
	}
	payload, err := j.store.ReadPayload(ctx, record.Execution.Pending.OperationID)
	if err != nil {
		return contract.Intent{}, err
	}
	return j.loadSnapshot(ctx, record, payload, publicationEligibility(record, payload))
}

func (j *PublicationJournal) Latest(ctx context.Context, id string) (contract.RecordSnapshot, error) {
	record, err := j.store.Read(ctx, id)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

func (j *PublicationJournal) loadSnapshot(ctx context.Context, record model.IssueOpsRecord, payload contract.IntentPayload, eligibility contract.CreateEligibility) (contract.Intent, error) {
	snapshot, err := j.store.RecordSnapshot(ctx, record.ID)
	if err != nil {
		return contract.Intent{}, err
	}
	raw, err := j.store.PayloadRaw(ctx, payload.OperationID)
	if err != nil {
		return contract.Intent{}, err
	}
	return contract.Intent{Record: snapshot, Raw: raw, OperationID: payload.OperationID, Generation: payload.Generation, Provider: payload.Provider, Kind: payload.Kind, Request: payload.Request.Clone(), InvocationState: contract.InvocationState(payload.InvocationState), RetryCount: payload.RetryCount, KnownURL: payload.KnownURL, Eligibility: eligibility}, nil
}

func pendingFacts(record model.IssueOpsRecord, expected string) domain.PendingIntentFacts {
	facts := domain.PendingIntentFacts{Prepared: record.Execution != nil, ExpectedOperationID: expected}
	if record.Execution != nil && record.Execution.Pending != nil {
		facts.Pending = true
		facts.OperationID = record.Execution.Pending.OperationID
	}
	return facts
}

func publicationEligibility(record model.IssueOpsRecord, payload contract.IntentPayload) contract.CreateEligibility {
	return contract.CreateEligibility{Provider: strings.ToLower(strings.TrimSpace(payload.Provider)), Kind: payload.Kind, Confirm: payload.Request.Confirm, PhasePR: record.Phase == model.IssueOpsPhasePR, ExecutionActive: record.Execution != nil && record.Execution.Lease.Status == model.LeaseStatusActive, NoPending: record.Execution == nil || record.Execution.Pending == nil, NoArtifact: record.RemoteArtifact == nil, BranchAuthority: true, CanonicalLabelsAssignees: len(payload.Request.Labels) > 0 && len(payload.Request.Assignees) > 0}
}

var _ publicationapp.Repository = (*PublicationJournal)(nil)
