package issueopslease

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	authorityport "issueops/internal/port/authority"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

type ReseedRequest struct {
	ID                   string
	ExpectedGeneration   uint64
	CompletionGeneration uint64
	Actor                leasedomain.Actor
	Ancestry             []leasedomain.ProcessReceipt
	CWD                  string
	InventoryFingerprint string
	Reason               string
	Confirm              bool
}

type ReseedResult struct {
	OK        bool
	ID        string
	Receipt   leasecontract.ReseedReceipt
	Execution leasecontract.Execution
}

type ReseedService struct {
	fence      ReseedFence
	repository ReseedRepository
	inventory  ReseedInventory
	baseSync   basesyncport.Inspector
	artifacts  ReseedArtifacts
	clock      Clock
	verifier   authorityport.ActorVerifier
	paths      CanonicalPathMatcher
}

func NewReseedService(fence ReseedFence, repository ReseedRepository, inventory ReseedInventory, baseSync basesyncport.Inspector, artifacts ReseedArtifacts, clock Clock, verifier authorityport.ActorVerifier, paths CanonicalPathMatcher) *ReseedService {
	return &ReseedService{fence: fence, repository: repository, inventory: inventory, baseSync: baseSync, artifacts: artifacts, clock: clock, verifier: verifier, paths: paths}
}

func (s *ReseedService) Reseed(ctx context.Context, request ReseedRequest) (ReseedResult, error) {
	if s.fence == nil || s.repository == nil || s.inventory == nil || s.baseSync == nil || s.artifacts == nil || s.clock == nil || s.paths == nil {
		return ReseedResult{ID: request.ID}, fmt.Errorf("reseed service dependencies are required")
	}
	var result ReseedResult
	err := s.fence.Within(ctx, request.ID, func(fenceCtx context.Context) error {
		reseeded, err := s.reseedWithinFence(fenceCtx, request)
		if err != nil {
			return err
		}
		result = reseeded
		return nil
	})
	if err != nil {
		return ReseedResult{ID: request.ID}, err
	}
	return result, nil
}

// reseedWithinFence runs one reseed attempt while the lease fence is held.
func (s *ReseedService) reseedWithinFence(ctx context.Context, request ReseedRequest) (ReseedResult, error) {
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.verifier)
	if err != nil {
		return ReseedResult{}, err
	}
	if !request.Confirm {
		return ReseedResult{}, fmt.Errorf("reseed requires confirm")
	}
	snapshot, err := s.repository.LoadSnapshot(ctx, request.ID)
	if err != nil {
		return ReseedResult{}, err
	}
	before := snapshot.Record
	if err := s.validateReseedPreconditions(before, request); err != nil {
		return ReseedResult{}, err
	}
	completionGeneration, err := resolveCompletionGeneration(before.Stable.Execution, request.CompletionGeneration)
	if err != nil {
		return ReseedResult{}, err
	}
	if err := s.observeCompletedBase(ctx, before.Stable, completionGeneration); err != nil {
		return ReseedResult{}, err
	}
	observed, err := s.inventory.Observe(ctx, before.Stable, actor)
	if err != nil {
		return ReseedResult{}, err
	}
	if observed.Fingerprint != request.InventoryFingerprint {
		return ReseedResult{}, fmt.Errorf("stale replacement inventory fingerprint")
	}
	if err := validateReseedRuntimeRollover(before, observed); err != nil {
		return ReseedResult{}, err
	}
	next, outcome, err := s.reseededRecord(before, request, observed, completionGeneration)
	if err != nil {
		return ReseedResult{}, err
	}
	prepared, err := s.artifacts.Prepare(ctx, next.Stable)
	if err != nil {
		return ReseedResult{}, err
	}
	applyPreparedReseedArtifacts(&next, outcome.Generation, prepared)
	after, err := s.repository.CommitReseed(ctx, snapshot, next)
	if err != nil {
		if rollbackErr := s.artifacts.Rollback(ctx, prepared); rollbackErr != nil {
			return ReseedResult{}, fmt.Errorf("%w; replacement residue cleanup failed: %v", err, rollbackErr)
		}
		return ReseedResult{}, err
	}
	_ = s.artifacts.CleanupSuperseded(ctx, before.Stable)
	prepared.Receipt.Execution = after.Execution
	return ReseedResult{OK: true, ID: request.ID, Receipt: prepared.Receipt, Execution: after.Execution}, nil
}

// validateReseedPreconditions checks the loaded record and request before any
// inventory or base observation happens.
func (s *ReseedService) validateReseedPreconditions(before Record, request ReseedRequest) error {
	if before.Stable.Execution == nil {
		return leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	if cleanupAbandonApplying(before.Stable.CleanupAbandonFailure) {
		return fmt.Errorf("execution reseed is blocked by cleanup abandon apply")
	}
	if err := leasedomain.ValidateReseedGeneration(toDomainLease(before.Lease), request.ExpectedGeneration); err != nil {
		return err
	}
	if before.Stable.Execution.Pending != nil {
		return fmt.Errorf("execution replacement is blocked by a pending external intent; run execution reconcile")
	}
	canonicalCWD := s.paths.Matches(request.CWD, before.SourceRoot) || s.paths.Matches(request.CWD, before.CanonicalRoot)
	return leasedomain.ValidateReseed(toDomainLease(before.Lease), leasedomain.ReseedRequest{ExpectedGeneration: request.ExpectedGeneration, CanonicalCWD: canonicalCWD, Reason: request.Reason})
}

// validateReseedRuntimeRollover rejects an Orca runtime change while a holder
// still owns the lease. Non-Orca executions have nothing to roll over.
func validateReseedRuntimeRollover(before Record, observed ReseedInventoryReceipt) error {
	if before.Stable.Execution.Mode != "orca" {
		return nil
	}
	binding := before.Stable.Execution.Orca
	if binding == nil {
		return leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	return leasedomain.ValidateHolderlessRuntimeRollover(leasedomain.HolderlessRuntimeRolloverRequest{
		Lease: toDomainLease(before.Lease), BindingRuntimeID: binding.RuntimeID, Inventory: observed.Inventory,
	})
}

// reseededRecord builds the replacement record: a cloned stable record with the
// reseeded lease applied and the completed execution reopened.
func (s *ReseedService) reseededRecord(before Record, request ReseedRequest, observed ReseedInventoryReceipt, completionGeneration uint64) (Record, leasedomain.ReseedOutcome, error) {
	stable, err := cloneReseedRecord(before.Stable)
	if err != nil {
		return Record{}, leasedomain.ReseedOutcome{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	next := before
	next.Stable = stable
	if next.Stable.Execution.Orca != nil && strings.TrimSpace(observed.RuntimeID) != "" {
		next.Stable.Execution.Orca.RuntimeID = strings.TrimSpace(observed.RuntimeID)
	}
	reseededAt := s.clock.Now()
	outcome := leasedomain.ApplyReseed(reseededAt, toDomainLease(before.Lease), leasedomain.ReseedRequest{ExpectedGeneration: request.ExpectedGeneration, Reason: request.Reason})
	lease := &next.Stable.Execution.Lease
	lease.Generation = outcome.Generation
	lease.Status = outcome.Status
	lease.Holder = toContractReseedActor(outcome.Holder)
	lease.ClaimTokenSHA256 = outcome.ClaimTokenSHA256
	lease.ReplacedAt = outcome.ReplacedAt
	lease.ReplacementReason = outcome.ReplacementReason
	if err := reopenCompletedExecution(&next.Stable, completionGeneration, before.Lease.Generation, outcome.Generation, outcome.ReplacementReason, outcome.ReplacedAt); err != nil {
		return Record{}, leasedomain.ReseedOutcome{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	return next, outcome, nil
}

// applyPreparedReseedArtifacts records the prepared artifact identities on the
// replacement record and mirrors the execution lease into the record lease.
func applyPreparedReseedArtifacts(next *Record, generation uint64, prepared ReseedArtifactReceipt) {
	next.Stable.Execution.Lease.ClaimTokenSHA256 = prepared.TokenSHA256
	if binding := next.Stable.Execution.Orca; binding != nil {
		binding.LeaseGeneration = generation
		binding.ArtifactIdentityVersion = leasecontract.OrcaArtifactIdentityVersion
		binding.IssueBodySHA256 = prepared.Receipt.IssueBodySHA256
		binding.ContextPacketSHA256 = prepared.Receipt.ContextPacketSHA256
		binding.OwnerPromptSHA256 = prepared.Receipt.OwnerPromptSHA256
	}
	next.Lease = next.Stable.Execution.Lease
}

func (s *ReseedService) observeCompletedBase(ctx context.Context, record leasecontract.Record, completionGeneration uint64) error {
	if record.Execution == nil || record.Execution.Completion == nil ||
		(record.Execution.Lease.Status != "released" && record.Execution.Lease.Status != "claimable") {
		return nil
	}
	var branchPrepare struct {
		BaseBranch string `json:"base_branch"`
	}
	if len(record.BranchPrepare) == 0 {
		return fmt.Errorf("completed reseed requires branch_prepare.base_branch")
	}
	if err := json.Unmarshal(record.BranchPrepare, &branchPrepare); err != nil {
		return fmt.Errorf("decode branch_prepare for completed reseed: %w", err)
	}
	if strings.TrimSpace(branchPrepare.BaseBranch) == "" {
		return fmt.Errorf("completed reseed requires branch_prepare.base_branch")
	}
	receipt, err := s.baseSync.Observe(ctx, basesyncport.Request{
		Worktree: record.Execution.Workspace.Root, BaseBranch: branchPrepare.BaseBranch,
	})
	if err != nil {
		return fmt.Errorf("observe completed execution base: %w", err)
	}
	if receipt.SyncRequired {
		return issueopscontract.NewBaseSyncRequiredError(record.ID, completionGeneration)
	}
	return nil
}

func resolveCompletionGeneration(execution *leasecontract.Execution, selected uint64) (uint64, error) {
	if execution == nil || execution.Completion == nil {
		if selected != 0 {
			return 0, fmt.Errorf("completion_generation requires a current completion")
		}
		return 0, nil
	}
	stamped := execution.Completion.Generation
	if stamped == 0 {
		return 0, fmt.Errorf("invalid or missing stamped completion generation")
	}
	if selected != 0 && selected != stamped {
		return 0, fmt.Errorf("completion_generation conflicts with stamped completion generation %d", stamped)
	}
	return stamped, nil
}

func reopenCompletedExecution(record *leasecontract.Record, completionGeneration, previousGeneration, nextGeneration uint64, reason, reopenedAt string) error {
	if record.Execution == nil || record.Execution.Completion == nil {
		return nil
	}
	completion := *record.Execution.Completion
	completion.Verification = append([]string(nil), record.Execution.Completion.Verification...)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "completed execution reseed"
	}
	record.Execution.CompletionHistory = append(record.Execution.CompletionHistory, leasecontract.CompletionHistoryEntry{
		Generation: completionGeneration,
		Completion: completion,
		Reason:     reason,
		ReopenedAt: reopenedAt,
	})
	record.Execution.Completion = nil
	record.Phase = "implement"
	record.AISlopCleanAt = ""
	record.AISlopCleanHead = ""
	record.PRHead = ""
	record.AISlopCleanFingerprint = ""
	record.AISlopCleanCategories = nil
	record.AISlopCleanVerification = nil
	record.ImplementationReview = nil
	record.RemoteCompletion = nil
	return staleCompletedReseedLedger(record, previousGeneration, nextGeneration)
}

func staleCompletedReseedLedger(record *leasecontract.Record, previousGeneration, nextGeneration uint64) error {
	ledger := issueopscontract.IssueOpsPhaseLedger{}
	if len(record.PhaseLedger) > 0 {
		if err := json.Unmarshal(record.PhaseLedger, &ledger); err != nil {
			return fmt.Errorf("decode phase ledger: %w", err)
		}
	}
	note := fmt.Sprintf("stale: completed execution reseed (%d -> %d)", previousGeneration, nextGeneration)
	for _, phase := range []issueopscontract.IssueOpsPhase{
		issueopscontract.IssueOpsPhaseImplement,
		issueopscontract.IssueOpsPhaseAISlopClean,
		issueopscontract.IssueOpsPhaseFeedback,
		issueopscontract.IssueOpsPhasePR,
		issueopscontract.IssueOpsPhaseDone,
	} {
		entry := ledger[phase]
		entry.Phase = phase
		entry.CompletedAt = ""
		if !containsLedgerNote(entry.Notes, note) {
			entry.Notes = append(entry.Notes, note)
		}
		ledger[phase] = entry
	}
	encoded, err := json.Marshal(ledger)
	if err != nil {
		return fmt.Errorf("encode phase ledger: %w", err)
	}
	record.PhaseLedger = encoded
	return nil
}

func containsLedgerNote(notes []string, want string) bool {
	return slices.Contains(notes, want)
}

func cleanupAbandonApplying(raw json.RawMessage) bool {
	var receipt struct {
		Step string `json:"step"`
	}
	return len(raw) > 0 && json.Unmarshal(raw, &receipt) == nil && receipt.Step == "applying"
}

func toContractReseedActor(actor *leasedomain.Actor) *leasecontract.Actor {
	if actor == nil {
		return nil
	}
	result := &leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}

func cloneReseedRecord(record leasecontract.Record) (leasecontract.Record, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return leasecontract.Record{}, err
	}
	var clone leasecontract.Record
	if err := json.Unmarshal(data, &clone); err != nil {
		return leasecontract.Record{}, err
	}
	return clone, nil
}
