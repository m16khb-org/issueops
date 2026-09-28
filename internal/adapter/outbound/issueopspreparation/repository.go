package issueopspreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	remote "issueops/internal/domain/issueopsremote"
	"path/filepath"
	"strings"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

const (
	recordBucket = "issueops_v1"
	holderBucket = "lease_holder_v1"
	intentBucket = "external_intent_v1"
)

type DiagnosticRedactor func(string) string

type SQLiteRepository struct {
	store  port.RecordInventoryStore
	redact DiagnosticRedactor
}

func NewSQLiteRepository(store port.RecordInventoryStore) *SQLiteRepository {
	return NewSQLiteRepositoryWithDiagnosticRedactor(store, nil)
}

func NewSQLiteRepositoryWithDiagnosticRedactor(store port.RecordInventoryStore, redact DiagnosticRedactor) *SQLiteRepository {
	return &SQLiteRepository{store: store, redact: redact}
}

func (repository *SQLiteRepository) Load(_ context.Context, id string) (preparationcontract.Snapshot, error) {
	if repository.store == nil {
		return preparationcontract.Snapshot{}, fmt.Errorf("preparation record store is unavailable")
	}
	data, ok, err := repository.store.Get(recordBucket, id)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	if !ok {
		return preparationcontract.Snapshot{}, fmt.Errorf("issueops record %s not found", id)
	}
	record, err := recordcodec.DecodeLease(id, data)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	return preparationcontract.Snapshot{
		Record: record, RecordRaw: append([]byte(nil), data...), ClaimTokenPath: currentClaimTokenPath(record),
	}, nil
}

func (repository *SQLiteRepository) EnsureRootUnclaimed(_ context.Context, selfID, root string) error {
	if repository.store == nil {
		return fmt.Errorf("preparation record store is unavailable")
	}
	return ensureRootUnclaimed(repository.store, selfID, root)
}

func (repository *SQLiteRepository) CommitDirect(ctx context.Context, commit preparationapp.DirectCommit) (preparationcontract.Result, error) {
	if repository.store == nil {
		return preparationcontract.Result{ID: commit.Command.ID}, fmt.Errorf("preparation record store is unavailable")
	}
	if err := preparationapp.ValidateDirectCommit(commit); err != nil {
		return preparationcontract.Result{ID: commit.Command.ID}, err
	}
	var result preparationcontract.Result
	err := repository.store.WithSpan(ctx, func(spanCtx context.Context) error {
		current, err := repository.Load(spanCtx, commit.Command.ID)
		if err != nil {
			return err
		}
		commit.ArtifactDir = remote.IssueArtifactDir(current.Record.IssueURL)
		record, projected, err := preparationapp.ApplyDirectCommit(current.Record, commit)
		if err != nil {
			return err
		}
		if err := ensureRootUnclaimed(repository.store, current.Record.ID, commit.Workspace.Root); err != nil {
			return err
		}
		actor := *record.Execution.Lease.Holder
		data, err := recordcodec.EncodeLease(record)
		if err != nil {
			return err
		}
		indexData, err := json.Marshal(holderIndex{
			SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID,
			Generation: 1, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		})
		if err != nil {
			return err
		}
		if err := repository.store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: recordBucket, ID: record.ID, Data: data},
			{Bucket: holderBucket, ID: holderIndexKey(actor), Data: indexData, RequireAbsent: true},
		}); err != nil {
			return err
		}
		result = projected
		return nil
	})
	if err != nil {
		return preparationcontract.Result{ID: commit.Command.ID}, err
	}
	return result, nil
}

func (repository *SQLiteRepository) BeginIntent(ctx context.Context, begin preparationapp.OrcaBegin) (preparationapp.IntentState, error) {
	if repository.store == nil {
		return preparationapp.IntentState{}, fmt.Errorf("preparation record store is unavailable")
	}
	if err := preparationapp.ValidateOrcaBegin(begin); err != nil {
		return preparationapp.IntentState{}, err
	}
	var state preparationapp.IntentState
	err := repository.store.WithSpan(ctx, func(spanCtx context.Context) error {
		current, err := repository.Load(spanCtx, begin.Snapshot.Record.ID)
		if err != nil {
			return err
		}
		if err := preparationapp.EnsureOrcaBeginUnprepared(current.Record); err != nil {
			return err
		}
		if err := ensureRootUnclaimed(repository.store, current.Record.ID, begin.Workspace.Root); err != nil {
			return err
		}
		codec := preparationcontract.IntentCodec{}
		record, intent, err := preparationapp.ApplyOrcaBegin(current.Record, begin, remote.IssueArtifactDir(current.Record.IssueURL))
		if err != nil {
			return err
		}
		intentData, err := codec.Encode(intent)
		if err != nil {
			return err
		}
		recordData, err := recordcodec.EncodeLease(record)
		if err != nil {
			return err
		}
		if err := repository.store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: recordBucket, ID: record.ID, Data: recordData},
			{Bucket: intentBucket, ID: intent.OperationID, Data: intentData, RequireAbsent: true},
		}); err != nil {
			return err
		}
		state = preparationapp.IntentState{
			Snapshot: preparationcontract.Snapshot{Record: record, RecordRaw: recordData},
			Command:  begin.Command.Clone(), Intent: intent, IntentRaw: intentData,
			Owner: begin.Owner, Pending: true,
		}
		return nil
	})
	return state, err
}

func (repository *SQLiteRepository) MarkInvoking(ctx context.Context, state preparationapp.IntentState) (preparationapp.IntentState, error) {
	if err := validateIntentState(state); err != nil {
		return state, err
	}
	updated := preparationapp.MarkOrcaInvoking(state.Intent)
	data, err := (preparationcontract.IntentCodec{}).Encode(updated)
	if err != nil {
		return state, err
	}
	if err := repository.compareAndApply(ctx, state, []port.RecordMutation{{Bucket: intentBucket, ID: updated.OperationID, Data: data}}); err != nil {
		return state, err
	}
	state.Intent, state.IntentRaw = updated, data
	return state, nil
}

func (repository *SQLiteRepository) RecordFailure(ctx context.Context, state preparationapp.IntentState, invocation string, cause error) error {
	if err := validateIntentState(state); err != nil {
		return err
	}
	record, intent, err := preparationapp.ApplyOrcaFailure(state, invocation, func() string { return repository.boundedDiagnostic(cause) })
	if err != nil {
		return err
	}
	intentData, err := (preparationcontract.IntentCodec{}).Encode(intent)
	if err != nil {
		return err
	}
	recordData, err := recordcodec.EncodeLease(record)
	if err != nil {
		return err
	}
	return repository.compareAndApply(ctx, state, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordData},
		{Bucket: intentBucket, ID: intent.OperationID, Data: intentData},
	})
}

func (repository *SQLiteRepository) ApplyReceipt(ctx context.Context, state preparationapp.IntentState, receipt preparationcontract.IntentReceipt) (preparationapp.IntentProgress, error) {
	if err := validateIntentState(state); err != nil {
		return preparationapp.IntentProgress{State: state, Pending: true}, err
	}
	intent := state.Intent
	intent.InvocationState = preparationcontract.InvocationNotInvoked
	intent.InvocationAttempts = 0
	record := state.Snapshot.Record
	switch state.Intent.Stage {
	case preparationcontract.IntentStageWorktree:
		if receipt.Workspace == nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
		}
		prepared := *receipt.Workspace
		intent.Prepared = &prepared
		intent.Launch = &preparationcontract.LaunchIdentity{
			PromptPath: state.OwnerArtifacts.OwnerPromptPath, PromptSHA256: state.OwnerArtifacts.OwnerPromptSHA256,
			ContextPacketPath: state.OwnerArtifacts.ContextPacketPath, ContextPacketSHA256: state.OwnerArtifacts.ContextPacketSHA256,
		}
		intent.ClaimTokenSHA256 = state.OwnerArtifacts.ClaimTokenSHA256
		intent.Stage = preparationcontract.IntentStageTerminal
		record.WorktreePath = prepared.Workspace.Root
		record.PlanPath = state.OwnerArtifacts.PlanPath
		record.Execution.Workspace = leasecontract.Workspace{
			SourceRoot: prepared.Workspace.SourceRoot, Root: prepared.Workspace.Root,
			Branch: prepared.Workspace.Branch, BaseHead: prepared.Workspace.BaseHead,
			ParentWorktree: prepared.Workspace.ParentWorktree, Driver: prepared.Workspace.Driver,
			LinkedAt: state.Intent.StartedAt, ArtifactDir: remote.IssueArtifactDir(record.IssueURL),
		}
	case preparationcontract.IntentStageTerminal:
		if strings.TrimSpace(receipt.TerminalPTYID) == "" {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca terminal candidate is incomplete")
		}
		intent.TerminalPTYID = strings.TrimSpace(receipt.TerminalPTYID)
		intent.Stage = preparationcontract.IntentStageRun
	case preparationcontract.IntentStageRun:
		if strings.TrimSpace(receipt.RunID) == "" {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca Run candidate is incomplete")
		}
		intent.RunID = strings.TrimSpace(receipt.RunID)
		intent.Stage = preparationcontract.IntentStageRunBind
	case preparationcontract.IntentStageRunBind:
		if strings.TrimSpace(receipt.RunID) != state.Intent.RunID || !receipt.RunBound {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca Run binding candidate is incomplete")
		}
		intent.RunBound = true
		intent.Stage = preparationcontract.IntentStageTask
	case preparationcontract.IntentStageTask:
		if strings.TrimSpace(receipt.TaskID) == "" {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca task candidate is incomplete")
		}
		intent.TaskID = strings.TrimSpace(receipt.TaskID)
		intent.Stage = preparationcontract.IntentStageDispatch
	case preparationcontract.IntentStageDispatch:
		if err := port.ValidateExecutionOrcaDeliveryReceipt(preparationDeliveryReceipt(receipt), port.OrcaDeliveryReceiptExpectation{
			Host: state.Intent.Probe.Host, TaskID: state.Intent.TaskID, TerminalPTYID: state.Intent.TerminalPTYID,
			DispatchRequestID: state.Intent.OrcaRequestID, PromptRequestID: state.Intent.OrcaPromptRequestID,
		}); err != nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca dispatch candidate is incomplete: %w", err)
		}
		if state.Intent.Prepared == nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca prepared workspace receipt is missing")
		}
		if state.Intent.Launch == nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca sealed owner artifact identity is missing")
		}
		intent.OrcaRequestID = strings.TrimSpace(receipt.RequestID)
		if receipt.PromptReceipt != nil {
			intent.OrcaPromptRequestID = strings.TrimSpace(receipt.PromptReceipt.RequestID)
		}
		record.Execution.Lease = leasecontract.Lease{
			Generation: state.Intent.Generation, Status: "claimable", ClaimTokenSHA256: state.Intent.ClaimTokenSHA256,
		}
		record.Execution.Orca = &leasecontract.OrcaBinding{
			RuntimeID: state.Intent.Prepared.RuntimeID, RepoID: state.Intent.Prepared.RepoID,
			WorktreeID: state.Intent.Prepared.WorktreeID, WorktreeInstanceID: state.Intent.Prepared.WorktreeInstanceID,
			LeaseGeneration: state.Intent.Generation, OwnerHost: state.Intent.Probe.Host,
			ArtifactIdentityVersion: leasecontract.OrcaArtifactIdentityVersion,
			IssueBodySHA256:         state.Intent.IssueBodySHA256, ContextPacketSHA256: state.Intent.Launch.ContextPacketSHA256,
			OwnerPromptSHA256: state.Intent.Launch.PromptSHA256,
			OwnerModel:        state.Intent.Probe.Model, OwnerEffort: state.Intent.Probe.Effort,
			RunID: state.Intent.RunID, TaskID: state.Intent.TaskID, DispatchID: strings.TrimSpace(receipt.DispatchID),
			TerminalPTYID: state.Intent.TerminalPTYID,
		}
		record.Execution.Pending = nil
		record.Execution.Failure = nil
		recordData, err := recordcodec.EncodeLease(record)
		if err != nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, err
		}
		if err := repository.compareAndApply(ctx, state, []port.RecordMutation{
			{Bucket: recordBucket, ID: record.ID, Data: recordData},
			{Bucket: intentBucket, ID: state.Intent.OperationID, Delete: true},
		}); err != nil {
			return preparationapp.IntentProgress{State: state, Pending: true}, err
		}
		state.Snapshot = preparationcontract.Snapshot{Record: record, RecordRaw: recordData, ClaimTokenPath: state.OwnerArtifacts.ClaimTokenPath}
		state.Pending = false
		result := preparationcontract.Result{
			OK: true, ID: record.ID, ResolvedMode: preparationcontract.ModeOrca,
			Workspace: record.Execution.Workspace, Execution: record.Execution,
			ClaimTokenPath: state.OwnerArtifacts.ClaimTokenPath, IssueBodySHA256: state.Intent.IssueBodySHA256,
			ContextPacketPath: state.OwnerArtifacts.ContextPacketPath, ContextPacketSHA256: state.OwnerArtifacts.ContextPacketSHA256,
			OwnerPromptPath: state.OwnerArtifacts.OwnerPromptPath, OwnerPromptSHA256: state.OwnerArtifacts.OwnerPromptSHA256,
			IssueSnapshotSource: state.Owner.Source,
		}
		return preparationapp.IntentProgress{State: state, Result: result}, nil
	default:
		return preparationapp.IntentProgress{State: state, Pending: true}, fmt.Errorf("unsupported Orca intent stage %q", state.Intent.Stage)
	}
	intentData, err := (preparationcontract.IntentCodec{}).Encode(intent)
	if err != nil {
		return preparationapp.IntentProgress{State: state, Pending: true}, err
	}
	record.Execution.Pending.Kind = preparationdomain.PendingKind(intent.Stage)
	record.Execution.Failure = nil
	recordData, err := recordcodec.EncodeLease(record)
	if err != nil {
		return preparationapp.IntentProgress{State: state, Pending: true}, err
	}
	if err := repository.compareAndApply(ctx, state, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordData},
		{Bucket: intentBucket, ID: intent.OperationID, Data: intentData},
	}); err != nil {
		return preparationapp.IntentProgress{State: state, Pending: true}, err
	}
	state.Intent, state.IntentRaw = intent, intentData
	state.Snapshot = preparationcontract.Snapshot{Record: record, RecordRaw: recordData, ClaimTokenPath: state.OwnerArtifacts.ClaimTokenPath}
	state.Pending = true
	return preparationapp.IntentProgress{State: state, Pending: true}, nil
}

func preparationDeliveryReceipt(receipt preparationcontract.IntentReceipt) port.ExecutionOrcaIntentReceipt {
	result := port.ExecutionOrcaIntentReceipt{
		TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle,
		TaskID: receipt.TaskID, DispatchID: receipt.DispatchID, RequestID: receipt.RequestID,
	}
	if receipt.PromptReceipt != nil {
		result.PromptReceipt = &port.OrcaPromptReceipt{
			RequestID: receipt.PromptReceipt.RequestID, Stages: append([]string(nil), receipt.PromptReceipt.Stages...),
			Provider: receipt.PromptReceipt.Provider, Observation: receipt.PromptReceipt.Observation,
			ProcessIncarnation: receipt.PromptReceipt.ProcessIncarnation, Generation: receipt.PromptReceipt.Generation,
			BaselineWorkingSequence: receipt.PromptReceipt.BaselineWorkingSequence,
		}
	}
	return result
}

func (repository *SQLiteRepository) compareAndApply(ctx context.Context, state preparationapp.IntentState, mutations []port.RecordMutation) error {
	store, ok := repository.store.(port.RecordCASStore)
	if !ok {
		return fmt.Errorf("preparation record store does not support raw CAS")
	}
	return store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: state.Snapshot.Record.ID, Data: state.Snapshot.RecordRaw},
		{Bucket: intentBucket, ID: state.Intent.OperationID, Data: state.IntentRaw},
	}, mutations)
}

func validateIntentState(state preparationapp.IntentState) error {
	if len(state.Snapshot.RecordRaw) == 0 || len(state.IntentRaw) == 0 {
		return fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	return preparationdomain.ValidateIntentRecord(state.Snapshot.Record, state.Intent)
}

func (repository *SQLiteRepository) boundedDiagnostic(cause error) string {
	message := "external operation failed"
	if repository.redact != nil && cause != nil {
		message = strings.TrimSpace(repository.redact(cause.Error()))
		if message == "" {
			message = "external operation failed"
		}
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}

func ensureRootUnclaimed(store port.RecordInventoryStore, selfID, root string) error {
	target := cleanAbsPath(root)
	if target == "" {
		return fmt.Errorf("canonical worktree root is required")
	}
	rows, err := store.GetAll(recordBucket)
	if err != nil {
		return err
	}
	self := strings.TrimSpace(selfID)
	for _, row := range rows {
		if row.ID == self {
			continue
		}
		record, err := recordcodec.DecodeLease(row.ID, row.Data)
		if err != nil {
			return fmt.Errorf("canonical worktree 소유권 스캔이 lifecycle %s 레코드를 읽지 못했다; 손상 레코드를 먼저 해소하라: %w", row.ID, err)
		}
		claims := []string{record.WorktreePath}
		if record.Execution != nil {
			claims = append(claims, record.Execution.Workspace.Root)
		}
		for _, claimed := range claims {
			if cleanAbsPath(claimed) == "" || cleanAbsPath(claimed) != target {
				continue
			}
			return fmt.Errorf(
				"canonical worktree %s는 이미 lifecycle %s(브랜치 %s)가 선점했다; 먼저 그 사이클을 정리하라: issueops cleanup finish --id %s --preview --json",
				target, row.ID, strings.TrimSpace(record.Branch), row.ID,
			)
		}
	}
	return nil
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

type holderIndex struct {
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

func holderIndexKey(actor leasecontract.Actor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}

func currentClaimTokenPath(record leasecontract.Record) string {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return ""
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(record.ID)))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}
