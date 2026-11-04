package issueopsremote

import (
	"context"
	"strings"

	publicationapp "issueops/internal/application/issueopspublication"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	cycledomain "issueops/internal/domain/issueops"
	domain "issueops/internal/domain/issueopspublication"
	"issueops/internal/domain/issueopsremote"
)

// PublicationJournal 타입은 PR/MR 게시 의도를 기록과 payload로 durable하게 남기고 시작·재시도·실패·완료를 한 트랜잭션 안에서 전이시킨다.
type PublicationJournal struct {
	store       PublicationStore
	environment PublicationEnvironment
	authority   authority
}

// NewPublicationJournal 함수는 저장소, 환경, 권한 검사를 묶어 PublicationJournal을 만든다.
func NewPublicationJournal(store PublicationStore, environment PublicationEnvironment, authority authority) *PublicationJournal {
	return &PublicationJournal{store: store, environment: environment, authority: authority}
}

// BeginCreate 메서드는 작업 ID와 marker를 붙인 게시 의도를 기록에 시작으로 남긴다. 행위자 권한, 실행 lease, 기존 아티팩트·대기 의도가 없음을 같은 트랜잭션에서 검증한다.
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
	payload := contract.IntentPayload{SchemaVersion: model.SchemaVersion, OperationID: operationID, Provider: prepared.Command.Provider, Kind: prepared.Eligibility.Kind, Request: request, InvocationState: string(contract.InvocationUnknown)}
	var persisted model.Record
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

// MarkRetry 메서드는 대기 중인 게시 의도의 재시도 횟수를 올린다. 저장된 payload가 바뀌었으면 거부한다.
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

// RecordFailure 메서드는 provider 호출 결과(invocation 상태, 알려진 URL)와 실패 진단을 대기 의도에 남긴다. 의도는 유지되어 재시도·복구에 쓰인다.
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
		current = cycledomain.FailPublication(current, expected.OperationID, issueopsremote.PublicationFailureDiagnostic(cause), j.environment.Timestamp(), false)
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Payload: &payload})
		return err
	})
}

// Complete 메서드는 검증된 URL로 아티팩트를 기록에 확정하고 대기 의도를 지운다. enforceOriginalGeneration이 true이면 의도를 시작한 lease 세대와 작업 디렉터리가 같아야 한다.
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
		facts := j.receiptAuthorityFacts(current, payload, enforceOriginalGeneration)
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
		authority := issueopsremote.ArtifactAuthority{Phase: string(current.Phase), IssueURL: current.IssueURL}
		if current.BranchPrepare != nil {
			authority.CodeProjectKey = current.BranchPrepare.CodeProjectKey
		}
		artifact, err := issueopsremote.ProjectArtifact(authority, issueopsremote.Artifact{Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url), Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch})
		if err != nil {
			return err
		}
		current = cycledomain.CompletePublication(current, model.RemoteArtifactVerification{Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL, Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch, VerifiedAt: j.environment.Timestamp()})
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Delete: true})
		return err
	})
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

// CompleteNotInvoked 메서드는 provider가 호출되지 않았음이 확인된 의도를 실패로 기록하고 지워 같은 게시를 다시 시작할 수 있게 한다.
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
		current = cycledomain.FailPublication(current, payload.OperationID, issueopsremote.PublicationFailureDiagnostic(cause), j.environment.Timestamp(), true)
		_, err = j.store.Persist(tx, current, contract.IntentMutation{OperationID: payload.OperationID, Delete: true})
		return err
	})
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

// LoadIntent 메서드는 기록에 대기 중인 게시 의도를 읽어 스냅샷과 함께 돌려준다. 대기 의도가 없거나 게시 종류가 아니면 오류다.
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

// Latest 메서드는 기록의 최신 스냅샷을 돌려준다.
func (j *PublicationJournal) Latest(ctx context.Context, id string) (contract.RecordSnapshot, error) {
	record, err := j.store.Read(ctx, id)
	if err != nil {
		return contract.RecordSnapshot{}, err
	}
	return j.store.RecordSnapshot(ctx, record.ID)
}

func (j *PublicationJournal) loadSnapshot(ctx context.Context, record model.Record, payload contract.IntentPayload, eligibility contract.CreateEligibility) (contract.Intent, error) {
	snapshot, err := j.store.RecordSnapshot(ctx, record.ID)
	if err != nil {
		return contract.Intent{}, err
	}
	raw, err := j.store.PayloadRaw(ctx, payload.OperationID)
	if err != nil {
		return contract.Intent{}, err
	}
	intent := contract.Intent{Record: snapshot, Raw: raw, OperationID: payload.OperationID, Generation: payload.Generation, Provider: payload.Provider, Kind: payload.Kind, Request: payload.Request.Clone(), InvocationState: contract.InvocationState(payload.InvocationState), RetryCount: payload.RetryCount, KnownURL: payload.KnownURL, Eligibility: eligibility}
	if record.Execution != nil && record.Execution.Pending != nil {
		intent.PendingStartedAt = record.Execution.Pending.StartedAt
	}
	return intent, nil
}

func (j *PublicationJournal) receiptAuthorityFacts(current model.Record, payload contract.IntentPayload, enforceOriginalGeneration bool) domain.ReceiptAuthorityFacts {
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
	return facts
}

func pendingFacts(record model.Record, expected string) domain.PendingIntentFacts {
	facts := domain.PendingIntentFacts{Prepared: record.Execution != nil, ExpectedOperationID: expected}
	if record.Execution != nil && record.Execution.Pending != nil {
		facts.Pending = true
		facts.OperationID = record.Execution.Pending.OperationID
	}
	return facts
}

func publicationEligibility(record model.Record, payload contract.IntentPayload) contract.CreateEligibility {
	return contract.CreateEligibility{Provider: strings.ToLower(strings.TrimSpace(payload.Provider)), Kind: payload.Kind, Confirm: payload.Request.Confirm, PhasePR: record.Phase == model.PhasePR, ExecutionActive: record.Execution != nil && record.Execution.Lease.Status == model.LeaseStatusActive, NoPending: record.Execution == nil || record.Execution.Pending == nil, NoArtifact: record.RemoteArtifact == nil, BranchAuthority: true, CanonicalLabelsAssignees: len(payload.Request.Labels) > 0 && len(payload.Request.Assignees) > 0}
}

var _ publicationapp.Repository = (*PublicationJournal)(nil)
