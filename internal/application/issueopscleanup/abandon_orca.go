package issueopscleanup

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	preparation "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	reconcile "issueops/internal/domain/issueopsreconcile"
	"issueops/internal/port"
)

type AbandonOrcaObserver struct {
	ReadIntent        func(string) (preparation.Intent, error)
	InspectionRequest func(model.IssueOpsRecord, preparation.Intent) (port.ExecutionOrcaIntentRequest, error)
	Orca              port.ExecutionOrcaProvisioner
	Owner             port.ExecutionOrcaOwnerInspector
}

func (s AbandonOrcaObserver) PendingSafe(ctx context.Context, record model.IssueOpsRecord, worktreePresent bool) error {
	pending := record.Execution.Pending
	if err := domain.ValidateCleanupAbandonPending(record, worktreePresent, reconcile.IsOrcaIntentKind(pending.Kind)); err != nil {
		return err
	}
	intent, err := s.ReadIntent(pending.OperationID)
	if err != nil {
		return err
	}
	if err := domain.ValidateCleanupAbandonIntent(record, domain.CleanupAbandonIntentIdentity{LifecycleID: intent.LifecycleID, Marker: intent.Marker, Generation: intent.Generation, PendingKind: preparationdomain.PendingKind(intent.Stage)}); err != nil {
		return err
	}
	if s.Orca == nil {
		return fmt.Errorf("Orca intent inspector is not configured")
	}
	current, err := s.InspectionRequest(record, intent)
	if err != nil {
		return err
	}
	stages, err := preparationdomain.CleanupAbandonIntentStages(intent.Stage)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		request := abandonInspectionRequest(current, stage)
		observed, err := s.Orca.InspectIntent(ctx, request)
		if err := domain.ValidateCleanupAbandonIntentObservation(string(stage), len(observed.Candidates), observed.AuthoritativeZero, err); err != nil {
			return err
		}
	}
	return nil
}

// Project the sealed request onto each resource that must be observed. Earlier
// stages never inherit later-stage resource IDs or launch receipts.
func abandonInspectionRequest(current port.ExecutionOrcaIntentRequest, stage preparation.IntentStage) port.ExecutionOrcaIntentRequest {
	current.Stage = port.ExecutionOrcaIntentStage(stage)
	switch stage {
	case preparation.IntentStageWorktree:
		current.Prepared = nil
		current.Launch = nil
		current.TerminalPTYID = ""
		current.RunID = ""
		current.RunBound = false
		current.TaskID = ""
	case preparation.IntentStageTerminal:
		current.TerminalPTYID = ""
		current.RunID = ""
		current.RunBound = false
		current.TaskID = ""
	case preparation.IntentStageTask:
		current.RunBound = true
		current.TaskID = ""
	}
	return current
}

func (s AbandonOrcaObserver) ResourcesAbsent(ctx context.Context, record model.IssueOpsRecord, holderless, terminalsReachable bool) error {
	binding := domain.CleanupAbandonOrcaTarget(record)
	if binding == nil {
		return nil
	}
	facts := domain.CleanupAbandonOrcaObservation{InspectorAvailable: s.Owner != nil, TerminalsReachable: terminalsReachable}
	if s.Owner != nil {
		observed, err := s.Owner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
			RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
			DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
			AllowRuntimeRollover: domain.CleanupAbandonAllowsRuntimeRollover(record, holderless),
		})
		facts.Err = err
		facts.TaskLive, facts.TerminalLive = observed.TaskLive, observed.TerminalLive
		facts.TaskStatus, facts.DispatchStatus = observed.TaskStatus, observed.DispatchStatus
	}
	return domain.ValidateCleanupAbandonOrcaObservation(facts)
}
