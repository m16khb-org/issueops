package issueopsapp

import (
	"context"
	"encoding/json"
	"fmt"
	ownerdomain "issueops/internal/domain/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"

	"issueops/internal/adapter/orca"
	basesyncoutbound "issueops/internal/adapter/outbound/issueopsbasesync"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/adapter/provider"
	leaseapp "issueops/internal/application/issueopslease"
	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

func issueOpsReseedHandler(ctx context.Context, stateRoot string, request model.ExecutionReseedRequest) (model.ExecutionReplaceResult, error) {
	return issueOpsReseedHandlerWithOwner(ctx, stateRoot, request, orca.NewExecution())
}

func issueOpsReseedHandlerWithOwner(ctx context.Context, stateRoot string, request model.ExecutionReseedRequest, owner port.ExecutionOrcaOwnerInspector) (model.ExecutionReplaceResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return model.ExecutionReplaceResult{ID: request.ID, Action: model.ExecutionReplaceReseed}, err
	}
	fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		return model.ExecutionReplaceResult{ID: request.ID, Action: model.ExecutionReplaceReseed}, err
	}
	inventory := leaseoutbound.NewReseedInventory(owner, leaseoutbound.InspectNativeProcess)
	readIssue := request.ReadIssue
	if readIssue == nil {
		readIssue = provider.ReadExecutionIssueSnapshot
	}
	ownerContext := newIssueOpsOwnerContext(stateRoot, readIssue)
	artifacts := leaseoutbound.NewReseedArtifacts(func(ctx context.Context, record leasecontract.Record) (leasecontract.ReseedReceipt, error) {
		execution, err := issueOpsReseedExecution(record)
		if err != nil {
			return leasecontract.ReseedReceipt{}, err
		}
		prepared, err := ownerContext.Reseed(ctx, record.ID, execution)
		if err != nil {
			return leasecontract.ReseedReceipt{}, err
		}
		return leasecontract.ReseedReceipt{IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketPath: prepared.ContextPacketPath, ContextPacketSHA256: prepared.ContextPacketSHA256, OwnerPromptPath: prepared.OwnerPromptPath, OwnerPromptSHA256: prepared.OwnerPromptSHA256}, nil
	})
	baseSync := basesyncoutbound.NewInspector(basesyncoutbound.RunGit)
	service := leaseapp.NewReseedService(fence, leaseoutbound.NewReseedRepository(db), inventory, baseSync, artifacts, leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, leaseoutbound.FilesystemPathMatcher{})
	return leaseinbound.NewReseedHandler(service, ownerdomain.OwnerReseedNextCommand)(ctx, stateRoot, request)
}

func issueOpsReseedExecution(record leasecontract.Record) (model.Execution, error) {
	if record.Execution == nil {
		return model.Execution{}, fmt.Errorf("reseed owner artifacts require an execution")
	}
	data, err := json.Marshal(record.Execution)
	if err != nil {
		return model.Execution{}, err
	}
	var execution model.Execution
	if err := json.Unmarshal(data, &execution); err != nil {
		return model.Execution{}, err
	}
	return execution, nil
}
