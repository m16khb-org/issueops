package issueopslease

import (
	"context"
	"errors"
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
)

type ResumeRequest struct {
	ID                 string
	ExpectedGeneration uint64
	Actor              leasedomain.Actor
	Ancestry           []leasedomain.ProcessReceipt
	CWD                string
	Confirm            bool
}

type ResumeResult struct {
	OK          bool
	ID          string
	Disposition leasedomain.ResumeDisposition
	Receipt     leasecontract.ResumeReceipt
}

type ResumeService struct {
	fence        ResumeFence
	repository   ResumeRepository
	artifacts    ResumeArtifacts
	owners       ResumeOwnerInventory
	stages       ResumeStageExecutor
	operationIDs ResumeOperationIDs
	verifier     authorityport.ActorVerifier
	paths        CanonicalPathMatcher
	roleAgents   RoleAgentArgs
}

func NewResumeService(fence ResumeFence, repository ResumeRepository, artifacts ResumeArtifacts, owners ResumeOwnerInventory, stages ResumeStageExecutor, operationIDs ResumeOperationIDs, verifier authorityport.ActorVerifier, paths CanonicalPathMatcher, roleAgents RoleAgentArgs) *ResumeService {
	return &ResumeService{fence: fence, repository: repository, artifacts: artifacts, owners: owners, stages: stages, operationIDs: operationIDs, verifier: verifier, paths: paths, roleAgents: roleAgents}
}

func (s *ResumeService) Resume(ctx context.Context, request ResumeRequest) (ResumeResult, error) {
	if !request.Confirm {
		return ResumeResult{ID: request.ID}, fmt.Errorf("execution resume requires confirm")
	}
	if s.fence == nil || s.repository == nil || s.artifacts == nil || s.owners == nil || s.stages == nil || s.operationIDs == nil || s.paths == nil || s.roleAgents == nil {
		return ResumeResult{ID: request.ID}, fmt.Errorf("resume service dependencies are required")
	}
	var result ResumeResult
	err := s.fence.Within(ctx, request.ID, func(fenceCtx context.Context) error {
		resumed, err := s.resumeWithinFence(fenceCtx, request)
		if err != nil {
			return err
		}
		result = resumed
		return nil
	})
	if err != nil {
		return ResumeResult{ID: request.ID}, err
	}
	return result, nil
}

// resumeWithinFence runs one resume attempt while the lease fence is held.
func (s *ResumeService) resumeWithinFence(ctx context.Context, request ResumeRequest) (ResumeResult, error) {
	snapshot, artifacts, plan, err := s.planResume(ctx, request)
	if err != nil {
		return ResumeResult{}, err
	}
	if plan.Disposition == leasedomain.ResumeExistingBinding {
		return ResumeResult{OK: true, ID: request.ID, Disposition: plan.Disposition, Receipt: leasecontract.ResumeReceipt{Execution: *snapshot.Record.Stable.Execution, Artifacts: artifacts}}, nil
	}
	roleAgentArgs, err := s.terminalRoleAgentArgs(ctx, snapshot.Record.Stable, plan)
	if err != nil {
		return ResumeResult{}, err
	}
	operationID, err := s.operationIDs.New()
	if err != nil {
		return ResumeResult{}, err
	}
	progress, err := s.repository.BeginIntent(ctx, snapshot, artifacts, plan, operationID)
	if err != nil {
		return ResumeResult{}, err
	}
	progress, err = s.driveResumeStages(ctx, progress, roleAgentArgs)
	if err != nil {
		return ResumeResult{}, err
	}
	return ResumeResult{OK: true, ID: request.ID, Disposition: plan.Disposition, Receipt: leasecontract.ResumeReceipt{Execution: progress.Execution, Artifacts: artifacts}}, nil
}

// planResume authenticates the caller, loads the snapshot, verifies artifacts,
// observes the owner inventory, and returns the resume plan.
func (s *ResumeService) planResume(ctx context.Context, request ResumeRequest) (ResumeSnapshot, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, error) {
	if _, err := resolveActor(ctx, request.Actor, request.Ancestry, s.verifier); err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	snapshot, err := s.repository.LoadSnapshot(ctx, request.ID, request.ExpectedGeneration)
	if err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	if snapshot.Record.Stable.Execution == nil || snapshot.Record.Stable.Execution.Orca == nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, fmt.Errorf("execution resume requires an existing Orca binding")
	}
	preflight := resumeDomainRequest(snapshot.Record.Stable, request.ExpectedGeneration, s.paths.Matches(request.CWD, snapshot.Record.CanonicalRoot), leasedomain.ResumeInventory{})
	if _, err := leasedomain.PlanResume(preflight); err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	artifacts, err := s.artifacts.ReadAndVerify(ctx, snapshot.Record.Stable)
	if err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	inventory, err := s.owners.Observe(ctx, snapshot.Record.Stable)
	if err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	plan, err := leasedomain.PlanResume(resumeDomainRequest(snapshot.Record.Stable, request.ExpectedGeneration, true, inventory))
	if err != nil {
		return ResumeSnapshot{}, leasecontract.ResumeArtifacts{}, leasedomain.ResumePlan{}, err
	}
	return snapshot, artifacts, plan, nil
}

// resumeStageAttempts bounds how many owner launch stages one resume drives.
const resumeStageAttempts = 5

// driveResumeStages applies pending owner launch stages until none remain.
func (s *ResumeService) driveResumeStages(ctx context.Context, progress ResumeProgress, roleAgentArgs []string) (ResumeProgress, error) {
	for attempts := 0; attempts < resumeStageAttempts && progress.Pending; attempts++ {
		next, err := s.advanceResumeStage(ctx, progress, roleAgentArgs)
		if err != nil {
			return ResumeProgress{}, err
		}
		progress = next
	}
	if progress.Pending {
		return ResumeProgress{}, fmt.Errorf("execution resume did not complete the owner launch stages")
	}
	return progress, nil
}

// advanceResumeStage inspects the pending intent, adopts, invokes, or refuses
// its stage, and applies the resulting receipt.
func (s *ResumeService) advanceResumeStage(ctx context.Context, progress ResumeProgress, roleAgentArgs []string) (ResumeProgress, error) {
	intent, err := s.repository.LoadIntent(ctx, progress)
	if err != nil {
		return ResumeProgress{}, err
	}
	inventory, err := s.stages.Inspect(ctx, intent)
	if err != nil {
		cause := fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
		_ = s.repository.RecordFailure(ctx, intent, intent.InvocationState, err)
		return ResumeProgress{}, cause
	}
	stagePlan, err := leasedomain.PlanResumeStage(leasedomain.ResumeStageRequest{CandidateCount: len(inventory.Candidates), AuthoritativeZero: inventory.AuthoritativeZero, ExactReplay: inventory.ExactReplay, InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts})
	if err != nil {
		cause := resumeStageDecisionCause(intent, inventory, err)
		_ = s.repository.RecordFailure(ctx, intent, intent.InvocationState, cause)
		return ResumeProgress{}, cause
	}
	receiptIntent, receipt, err := s.resolveStageReceipt(ctx, intent, inventory, stagePlan, roleAgentArgs)
	if err != nil {
		return ResumeProgress{}, err
	}
	next, err := s.repository.ApplyReceipt(ctx, receiptIntent, receipt)
	if err != nil {
		_ = s.repository.RecordFailure(ctx, receiptIntent, "unknown", err)
		return ResumeProgress{}, err
	}
	return next, nil
}

// resolveStageReceipt produces the stage receipt for the planned action and
// the intent state the receipt must be applied against.
func (s *ResumeService) resolveStageReceipt(ctx context.Context, intent ResumeIntentState, inventory leasecontract.ResumeStageInventory, stagePlan leasedomain.ResumeStagePlan, roleAgentArgs []string) (ResumeIntentState, leasecontract.ResumeStageReceipt, error) {
	switch stagePlan.Action {
	case leasedomain.ResumeStageAdopt:
		return intent, inventory.Candidates[stagePlan.CandidateIndex], nil
	case leasedomain.ResumeStageInvoke:
		receiptIntent, err := s.repository.MarkInvoking(ctx, intent)
		if err != nil {
			return intent, leasecontract.ResumeStageReceipt{}, err
		}
		receipt, err := s.stages.Invoke(ctx, receiptIntent, argsForStage(intent.Stage, roleAgentArgs))
		if err != nil {
			_ = s.repository.RecordFailure(ctx, receiptIntent, resumeInvocationFailureState(err), err)
			return receiptIntent, leasecontract.ResumeStageReceipt{}, resumeReconcileRequired(err)
		}
		return receiptIntent, receipt, nil
	case leasedomain.ResumeStageReconcile:
		cause := resumeStageReconcileCause(stagePlan.Reason)
		_ = s.repository.RecordFailure(ctx, intent, intent.InvocationState, cause)
		return intent, leasecontract.ResumeStageReceipt{}, cause
	}
	return intent, leasecontract.ResumeStageReceipt{}, nil
}

func resumeDomainRequest(record leasecontract.Record, generation uint64, canonicalCWD bool, inventory leasedomain.ResumeInventory) leasedomain.ResumeRequest {
	binding := record.Execution.Orca
	return leasedomain.ResumeRequest{
		ExpectedGeneration: generation, Lease: toDomainLease(record.Execution.Lease), BindingGeneration: binding.LeaseGeneration,
		BindingRuntimeID: binding.RuntimeID, BindingTerminalID: binding.TerminalPTYID, CanonicalCWD: canonicalCWD,
		ModeOrca: record.Execution.Mode == "orca", BindingPresent: binding != nil, PendingAbsent: record.Execution.Pending == nil,
		Inventory: inventory,
	}
}

func resumeReconcileRequired(cause error) error {
	return fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", cause)
}

func resumeStageDecisionCause(intent ResumeIntentState, inventory leasecontract.ResumeStageInventory, fallback error) error {
	if len(inventory.Candidates) > 1 {
		return fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
	}
	if intent.InvocationAttempts >= 2 {
		return fmt.Errorf("Orca intent retry is exhausted; intent retained")
	}
	return fallback
}

func resumeStageReconcileCause(reason string) error {
	switch reason {
	case "non-authoritative-zero":
		return fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
	case "unknown-invocation":
		return fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
	default:
		return fmt.Errorf("Orca intent inventory %s; intent retained", reason)
	}
}

func resumeInvocationFailureState(err error) string {
	if typed, ok := errors.AsType[*port.OrcaError](err); ok && !typed.Invoked {
		return "not_invoked_proven"
	}
	return "unknown"
}

// terminalRoleAgentArgs resolves the role agents before BeginIntent, which
// leaves a pending intent that cannot be rolled back. A settings error then
// stops resume without that intent, so the same resume retries after the fix.
func (s *ResumeService) terminalRoleAgentArgs(ctx context.Context, record leasecontract.Record, plan leasedomain.ResumePlan) ([]string, error) {
	if plan.ReusedTerminalPTYID != "" {
		return nil, nil
	}
	args, err := s.roleAgents(ctx, record.Execution.Orca.OwnerHost, record.Repo)
	if err != nil {
		return nil, fmt.Errorf("resolve role agents before the Orca terminal launch: %w", err)
	}
	return args, nil
}

// argsForStage passes role-agent arguments only to the terminal stage.
func argsForStage(stage string, roleAgentArgs []string) []string {
	if stage != terminalStage {
		return nil
	}
	return roleAgentArgs
}
