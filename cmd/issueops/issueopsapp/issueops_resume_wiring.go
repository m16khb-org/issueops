package issueopsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/orca"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	preparationoutbound "issueops/internal/adapter/outbound/issueopspreparation"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	ownerapp "issueops/internal/application/issueopsowner"
	preparationapp "issueops/internal/application/issueopspreparation"
	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	ownerdomain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

func issueOpsResumeHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionResumeRequest) (issueopscontract.ExecutionResumeResult, error) {
	orcaExecution := orca.NewExecution()
	service, err := newIssueOpsResumeService(stateRoot, orcaExecution, orcaExecution)
	if err != nil {
		return issueopscontract.ExecutionResumeResult{ID: request.ID}, err
	}
	return leaseinbound.NewResumeHandler(service, ownerdomain.OwnerResumeNextCommand)(ctx, stateRoot, request)
}

func newIssueOpsResumeService(stateRoot string, provisioner port.ExecutionOrcaProvisioner, owner port.ExecutionOrcaOwnerInspector) (*leaseapp.ResumeService, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		return nil, err
	}
	effects := &resumeHostAdapter{stateRoot: stateRoot, provisioner: newHandoffDeliveryProvisioner(stateRoot, provisioner, time.Now), owner: owner}
	repository := leaseoutbound.NewResumeRepositoryWithDiagnosticRedactor(db, policy.RedactDiagnostic, time.Now)
	return leaseapp.NewResumeService(
		fence,
		repository,
		leaseoutbound.NewResumeArtifacts(effects.readArtifacts),
		leaseoutbound.NewResumeOwnerInventory(effects.observeOwner),
		leaseoutbound.NewResumeStageExecutor(effects.inspectStage, effects.invokeStage),
		resumeOperationIDs{},
		leaseoutbound.InspectNativeProcess,
		leaseoutbound.FilesystemPathMatcher{},
	), nil
}

type resumeOperationIDs struct{}

func (resumeOperationIDs) New() (string, error) { return issueops.NewExecutionOperationID() }

type resumeHostAdapter struct {
	stateRoot   string
	provisioner port.ExecutionOrcaProvisioner
	owner       port.ExecutionOrcaOwnerInspector
}

func (e *resumeHostAdapter) readArtifacts(_ context.Context, record leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
	coreRecord, err := resumeCoreRecord(record)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	artifacts, err := (ownerapp.ResumeReader{Files: issueops.OwnerContextFiles{StateRoot: e.stateRoot}}).Read(coreRecord)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	return leasecontract.ResumeArtifacts{ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256}, nil
}

func (e *resumeHostAdapter) observeOwner(ctx context.Context, record leasecontract.Record) (leasedomain.ResumeInventory, error) {
	if e.owner == nil || record.Execution == nil || record.Execution.Orca == nil {
		return leasedomain.ResumeInventory{}, fmt.Errorf("resume owner inspector is required")
	}
	binding := record.Execution.Orca
	inventory, err := e.owner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID, AllowRuntimeRollover: true})
	if err != nil {
		return leasedomain.ResumeInventory{}, fmt.Errorf("inspect previous Orca owner: %w", err)
	}
	return leasedomain.ResumeInventory{
		RuntimeID: inventory.RuntimeID, TerminalLive: inventory.TerminalLive, TerminalInventoryComplete: inventory.TerminalInventoryComplete,
		TaskLive: inventory.TaskLive, TerminalID: inventory.TerminalID, TaskStatus: inventory.TaskStatus, DispatchStatus: inventory.DispatchStatus,
		DispatchAssigneeHandle: inventory.DispatchAssigneeHandle, DispatchAssigneePresent: inventory.DispatchAssigneePresent,
	}, nil
}

func (e *resumeHostAdapter) inspectStage(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	if e.provisioner == nil {
		return leasecontract.ResumeStageInventory{}, fmt.Errorf("resume Orca provisioner is required")
	}
	request, err := issueOpsOrcaIntentRequest(intent.Progress.Record.Stable, intent.OperationID, intent.IntentRaw)
	if err != nil {
		return leasecontract.ResumeStageInventory{}, err
	}
	inventory, err := e.provisioner.InspectIntent(ctx, request)
	if err != nil {
		return leasecontract.ResumeStageInventory{}, err
	}
	result := leasecontract.ResumeStageInventory{AuthoritativeZero: inventory.AuthoritativeZero, ExactReplay: inventory.ExactReplay}
	for _, candidate := range inventory.Candidates {
		result.Candidates = append(result.Candidates, resumeContractReceipt(candidate))
	}
	return result, nil
}

func (e *resumeHostAdapter) invokeStage(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
	if e.provisioner == nil {
		return leasecontract.ResumeStageReceipt{}, fmt.Errorf("resume Orca provisioner is required")
	}
	request, err := issueOpsOrcaIntentRequest(intent.Progress.Record.Stable, intent.OperationID, intent.IntentRaw)
	if err != nil {
		return leasecontract.ResumeStageReceipt{}, err
	}
	receipt, err := e.provisioner.InvokeIntent(ctx, request)
	if err != nil {
		return leasecontract.ResumeStageReceipt{}, err
	}
	return resumeContractReceipt(receipt), nil
}

func resumeCoreRecord(record leasecontract.Record) (issueopscontract.IssueOpsRecord, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	var result issueopscontract.IssueOpsRecord
	if err := json.Unmarshal(data, &result); err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	return result, nil
}

func issueOpsOrcaIntentRequest(record leasecontract.Record, operationID string, intentRaw []byte) (port.ExecutionOrcaIntentRequest, error) {
	sealed, err := (preparationcontract.IntentCodec{}).Decode(operationID, intentRaw)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	request, err := (preparationapp.IntentRequestBuilder{Files: issueops.OrcaIntentFiles{}}).Build(record, sealed)
	return preparationoutbound.OrcaIntentRequest(request), err
}

func resumeContractReceipt(receipt port.ExecutionOrcaIntentReceipt) leasecontract.ResumeStageReceipt {
	result := leasecontract.ResumeStageReceipt{
		TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle, RunID: receipt.RunID, RunBound: receipt.RunBound,
		TaskID: receipt.TaskID, DispatchID: receipt.DispatchID, RequestID: receipt.RequestID,
	}
	if receipt.PromptReceipt != nil {
		result.PromptReceipt = &leasecontract.OrcaPromptReceipt{
			RequestID: receipt.PromptReceipt.RequestID, Stages: append([]string(nil), receipt.PromptReceipt.Stages...), Provider: receipt.PromptReceipt.Provider,
			Observation: receipt.PromptReceipt.Observation, ProcessIncarnation: receipt.PromptReceipt.ProcessIncarnation,
			Generation: receipt.PromptReceipt.Generation, BaselineWorkingSequence: receipt.PromptReceipt.BaselineWorkingSequence,
		}
	}
	return result
}
