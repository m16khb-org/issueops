package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"time"

	issueopscontract "issueops/internal/contract/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"

	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	statecontract "issueops/internal/contract/state"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

func issueOpsReconcileHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionReconcileRequest, deps port.ExecutionReconcileDependencies) (issueopscontract.ExecutionReconcileResult, error) {
	service, err := newIssueOpsReconcileService(stateRoot, deps.Orca, deps.ReadIssue, request.Snapshot, deps.Now)
	if err != nil {
		return issueopscontract.ExecutionReconcileResult{ID: request.ID}, err
	}
	return leaseinbound.NewReconcileHandler(service)(ctx, stateRoot, request, deps)
}

func newIssueOpsReconcileService(stateRoot string, provisioner port.ExecutionOrcaProvisioner, readIssue executionissue.ExecutionIssueSnapshotReadFunc, snapshot *issueopscontract.IssueOpsRecord, now func() time.Time) (*leaseapp.ReconcileService, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	var expected *leasecontract.Record
	if snapshot != nil {
		record, err := reconcileContractRecord(*snapshot)
		if err != nil {
			return nil, err
		}
		expected = &record
	}
	effects := &coreReconcileEffects{stateRoot: stateRoot, provisioner: newHandoffDeliveryProvisioner(stateRoot, provisioner, now), readIssue: readIssue}
	return leaseapp.NewReconcileService(
		leaseoutbound.NewReconcileRepositoryWithSnapshot(db, effects, expected, policy.RedactDiagnostic, now),
		leaseoutbound.NewReconcileStageExecutor(effects.inspectStage, effects.invokeStage),
		stateRoleAgentArgs(stateRoot),
	), nil
}

type coreReconcileEffects struct {
	stateRoot   string
	provisioner port.ExecutionOrcaProvisioner
	readIssue   executionissue.ExecutionIssueSnapshotReadFunc
}

func (e *coreReconcileEffects) PrepareWorktree(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
	return newIssueOpsOwnerContext(e.stateRoot, e.readIssue).Prepare(ctx, snapshot, command, intent, receipt)
}

func (e *coreReconcileEffects) inspectStage(ctx context.Context, intent leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
	if e.provisioner == nil {
		return leasecontract.ReconcileStageInventory{}, false, fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := e.reconcileRequest(intent)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, false, err
	}
	inventory, err := e.provisioner.InspectIntent(ctx, request)
	if err != nil {
		return leasecontract.ReconcileStageInventory{}, true, err
	}
	result := leasecontract.ReconcileStageInventory{AuthoritativeZero: inventory.AuthoritativeZero, ExactReplay: inventory.ExactReplay}
	for _, candidate := range inventory.Candidates {
		converted, err := reconcileContractReceipt(candidate)
		if err != nil {
			return leasecontract.ReconcileStageInventory{}, true, err
		}
		result.Candidates = append(result.Candidates, converted)
	}
	return result, true, nil
}

func (e *coreReconcileEffects) invokeStage(ctx context.Context, intent leaseapp.ReconcileIntentState, roleAgentArgs []string) (leasecontract.ReconcileStageReceipt, string, error) {
	if e.provisioner == nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", fmt.Errorf("Orca intent reconciliation is unavailable")
	}
	request, err := e.reconcileRequest(intent)
	if err != nil {
		return leasecontract.ReconcileStageReceipt{}, "unknown", err
	}
	request.RoleAgentArgs = roleAgentArgs
	receipt, err := e.provisioner.InvokeIntent(ctx, request)
	if err != nil {
		invocation := "unknown"
		if typed, ok := errors.AsType[*port.OrcaError](err); ok && !typed.Invoked {
			invocation = "not_invoked_proven"
		}
		return leasecontract.ReconcileStageReceipt{}, invocation, err
	}
	converted, err := reconcileContractReceipt(receipt)
	return converted, "", err
}

func (e *coreReconcileEffects) reconcileRequest(intent leaseapp.ReconcileIntentState) (port.ExecutionOrcaIntentRequest, error) {
	return issueOpsOrcaIntentRequest(intent.Progress.Record, intent.OperationID, intent.IntentRaw)
}

func reconcileContractRecord(record issueopscontract.IssueOpsRecord) (leasecontract.Record, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return leasecontract.Record{}, err
	}
	decoded, err := leasecontract.Decode(record.ID, data)
	if err != nil {
		return leasecontract.Record{}, err
	}
	if err := leasedomain.ValidatePersistedRecord(decoded); err != nil {
		return leasecontract.Record{}, statecontract.Invalid("")
	}
	return decoded, nil
}

func reconcileContractReceipt(receipt port.ExecutionOrcaIntentReceipt) (leasecontract.ReconcileStageReceipt, error) {
	return convertReconcileReceipt[leasecontract.ReconcileStageReceipt](receipt)
}

func convertReconcileReceipt[T any](source any) (T, error) {
	var target T
	data, err := json.Marshal(source)
	if err != nil {
		return target, err
	}
	return target, json.Unmarshal(data, &target)
}
