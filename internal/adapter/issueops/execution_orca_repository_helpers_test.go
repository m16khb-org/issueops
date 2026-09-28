package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	preparationoutbound "issueops/internal/adapter/outbound/issueopspreparation"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	preparationapp "issueops/internal/application/issueopspreparation"
	"issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

func beginOrcaIntentViaRepository(stateRoot string, record issueops.IssueOpsRecord, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, request ExecutionPrepareRequest, owner executionOwnerSnapshot, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		return record, externalOrcaIntentPayload{}, err
	}
	repository := preparationoutbound.NewSQLiteRepository(store)
	snapshot, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		return record, externalOrcaIntentPayload{}, err
	}
	operationID, err := newExecutionOperationID()
	if err != nil {
		return record, externalOrcaIntentPayload{}, err
	}
	command := preparationcontract.Command{
		ID: record.ID, Mode: preparationcontract.ModeOrca,
		OwnerHost: request.OwnerHost, OwnerModel: request.OwnerModel, OwnerEffort: request.OwnerEffort,
	}
	selection := leasecontract.Selection{
		RequestedMode: preparationcontract.ModeOrca, ResolvedMode: preparationcontract.ModeOrca,
		ProbeAttempted: true, ProbeAvailable: true, ProbeReady: true, SelectedAt: executionNow(now),
	}
	selection.ReadinessFingerprint = preparationdomain.Fingerprint(preparationdomain.Decision{
		RequestedMode: selection.RequestedMode, ResolvedMode: selection.ResolvedMode,
		ProbeAttempted: selection.ProbeAttempted, ProbeAvailable: selection.ProbeAvailable, ProbeReady: selection.ProbeReady,
		ProbeProvider: probe.Provider, ProbeIssue: probe.Issue,
	}, command)
	state, err := repository.BeginIntent(context.Background(), preparationapp.OrcaBegin{
		Snapshot: snapshot, Command: command,
		Workspace: intentContractWorkspaceRequest(workspace), Probe: intentContractProbeRequest(probe),
		Owner:       preparationcontract.OwnerEvidence{Provider: probe.Provider, Issue: probe.Issue, BodySHA256: owner.issue.BodySHA256},
		OperationID: operationID, StartedAt: executionNow(now), Selection: selection,
	})
	if err != nil {
		return record, externalOrcaIntentPayload{}, err
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	return persisted, state.Intent, err
}

type reconcileWorktreeTestEffects struct {
	stateRoot string
	readIssue ExecutionIssueSnapshotReadFunc
}

type ExecutionResumeIntentState struct {
	Record             issueops.IssueOpsRecord
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              port.ExecutionOrcaIntentStage
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

func executionResumeIntentPayload(expected ExecutionResumeIntentState) (externalOrcaIntentPayload, error) {
	return (preparationcontract.IntentCodec{}).Decode(expected.OperationID, expected.IntentRaw)
}

func intentContractWorkspaceRequest(workspace port.ExecutionWorkspaceRequest) preparationcontract.WorkspaceRequest {
	return preparationcontract.WorkspaceRequest{
		LifecycleID: workspace.LifecycleID, SourceRoot: workspace.SourceRoot, Root: workspace.Root,
		Branch: workspace.Branch, BaseBranch: workspace.BaseBranch, BaseHead: workspace.BaseHead,
		ParentWorktree: workspace.ParentWorktree, Confirm: workspace.Confirm,
	}
}

func intentContractProbeRequest(probe port.ExecutionOrcaProbeRequest) preparationcontract.ProbeRequest {
	return preparationcontract.ProbeRequest{
		Repo: probe.Repo, Host: probe.Host, Model: probe.Model, Effort: probe.Effort,
		Provider: probe.Provider, Issue: probe.Issue, Marker: probe.Marker,
	}
}

func intentContractStage(stage port.ExecutionOrcaIntentStage) preparationcontract.IntentStage {
	return preparationcontract.IntentStage(stage)
}

func (e reconcileWorktreeTestEffects) PrepareWorktree(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
	return PrepareExecutionPreparationOwner(ctx, e.stateRoot, snapshot, command, intent, receipt, e.readIssue)
}

func advanceOrcaIntentReceiptViaRepository(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, expected externalOrcaIntentPayload, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFunc, _ func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		return record, expected, err
	}
	repository := leaseoutbound.NewReconcileRepository(store, reconcileWorktreeTestEffects{stateRoot: stateRoot, readIssue: readIssue})
	state, err := repository.Canonicalize(ctx, record.ID)
	if err != nil {
		return record, expected, err
	}
	current, err := (preparationcontract.IntentCodec{}).Decode(expected.OperationID, state.IntentRaw)
	if err != nil || !reflect.DeepEqual(current, expected) {
		return record, expected, fmt.Errorf("Orca intent payload changed before receipt CAS: %v", err)
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return record, expected, err
	}
	var converted leasecontract.ReconcileStageReceipt
	if err := json.Unmarshal(data, &converted); err != nil {
		return record, expected, err
	}
	progress, err := repository.ApplyReceipt(ctx, state, converted)
	if err != nil {
		return record, expected, err
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		return record, expected, err
	}
	if !progress.Pending {
		return persisted, expected, nil
	}
	intentRaw, ok, err := store.Get(externalIntentBucket, expected.OperationID)
	if err != nil || !ok {
		return persisted, expected, fmt.Errorf("advanced Orca intent unavailable: %v", err)
	}
	next, err := (preparationcontract.IntentCodec{}).Decode(expected.OperationID, intentRaw)
	return persisted, next, err
}

func loadResumeIntentViaRepository(stateRoot, id, operationID string) (ExecutionResumeIntentState, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	repository := leaseoutbound.NewResumeRepository(store)
	snapshot, err := repository.LoadSnapshot(context.Background(), id, record.Execution.Lease.Generation)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	progress := leaseapp.ResumeProgress{Record: snapshot.Record, Execution: *snapshot.Record.Stable.Execution, Pending: true}
	intent, err := repository.LoadIntent(context.Background(), progress)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	if intent.OperationID != operationID {
		return ExecutionResumeIntentState{}, fmt.Errorf("Orca external intent payload is missing")
	}
	return ExecutionResumeIntentState{
		Record: record, RecordRaw: intent.RecordRaw, IntentRaw: intent.IntentRaw,
		OperationID: intent.OperationID, Stage: port.ExecutionOrcaIntentStage(intent.Stage),
		InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts, Pending: true,
	}, nil
}

func recordResumeIntentFailureViaRepository(stateRoot string, expected ExecutionResumeIntentState, invocationState string, cause error, _ func() time.Time) error {
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	record, err := leasecontract.Decode(expected.Record.ID, expected.RecordRaw)
	if err != nil {
		return err
	}
	return leaseoutbound.NewResumeRepository(store).RecordFailure(context.Background(), leaseapp.ResumeIntentState{
		Progress: leaseapp.ResumeProgress{
			Record: leaseapp.Record{ID: record.ID, Stable: record}, Execution: *record.Execution, Pending: expected.Pending,
		},
		OperationID: expected.OperationID, Stage: string(expected.Stage),
		InvocationState: expected.InvocationState, InvocationAttempts: expected.InvocationAttempts,
		RecordRaw: expected.RecordRaw, IntentRaw: expected.IntentRaw,
	}, invocationState, cause)
}
