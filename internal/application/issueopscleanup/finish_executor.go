package issueopscleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type FinishExecutor struct {
	Records      port.CleanupFinishRecords
	Acquire      func(context.Context, string) (CleanupLifetime, error)
	Observe      func(context.Context, model.IssueOpsRecord, model.CleanupFinishRequest) (model.CleanupFinishRequest, error)
	Plan         func(context.Context, model.IssueOpsRecord, model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult)
	Fingerprint  func(model.CleanupFinishInventory) (string, error)
	NewAttempt   func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error)
	Completion   func(model.IssueOpsRecord) model.RemoteCompletionSection
	Stop         func(context.Context, model.CleanupFinishInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error)
	RemoveOrca   func(context.Context, string) error
	Directory    func(string) (bool, error)
	Git          func(context.Context, string, ...string) (int, string)
	ReflectAudit func(context.Context, model.IssueOpsRecord, model.RemoteCompletionSection, string) error
	Now          func() time.Time
}

func (s FinishExecutor) Run(ctx context.Context, req model.CleanupFinishRequest) (model.CleanupFinishResult, error) {
	failed := model.CleanupFinishResult{OK: false, ID: req.ID}
	lifetime, err := s.Acquire(ctx, req.ID)
	if err != nil {
		return failed, err
	}
	defer func() {
		if lifetime != nil {
			_ = lifetime.Close()
		}
	}()
	ctx = lifetime.Context(ctx)
	snapshot, err := s.Records.Load(ctx, req.ID)
	if err != nil {
		return failed, err
	}
	record := snapshot.Record
	if err := domain.ValidateCleanupOperationAccess(record, model.CleanupOperationFinish); err != nil {
		return failed, err
	}
	req, err = s.Observe(ctx, record, req)
	if err != nil {
		return failed, &port.CleanupFinishObservationError{Err: err}
	}
	inventory, result := s.Plan(ctx, record, req)
	if len(result.Missing) > 0 {
		result.OK = false
		result.NextCommand = domain.CleanupFinishRemedyCommand(record.ID, result.Missing)
		return result, fmt.Errorf("cleanup finish is not ready: %s", strings.Join(result.Missing, ", "))
	}
	fingerprint, err := s.Fingerprint(inventory)
	if err != nil {
		return failed, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = domain.CleanupFinishApplyCommand(record.ID, fingerprint, result.SupersededBy, req.KeepRemoteBranch)
		return result, nil
	}
	if err := domain.ValidateCleanupFinishApply(req, fingerprint); err != nil {
		result.OK = false
		return result, err
	}
	completion := s.Completion(record)
	attempt, err := s.NewAttempt(model.CleanupOperationFinish)
	if err != nil {
		return failed, err
	}
	snapshot, err = s.Records.Arm(ctx, snapshot, attempt)
	if err != nil {
		result.OK = false
		return result, err
	}
	// Cancellation must stop commands, but must not prevent releasing their
	// durable ownership after the inherited descriptors have safely drained.
	finalizeCtx := context.WithoutCancel(ctx)
	drainageAttempted, drained := false, false
	var drainageErr error
	drain := func() error {
		if drainageAttempted {
			return drainageErr
		}
		drainageAttempted = true
		var next CleanupLifetime
		next, drainageErr = lifetime.Drain(finalizeCtx)
		lifetime = next
		drained = drainageErr == nil
		return drainageErr
	}
	fail := func(step string, cause error) (model.CleanupFinishResult, error) {
		result.OK = false
		result.FailedStep = step
		drainErr := drain()
		_, receiptErr := s.Records.Fail(finalizeCtx, snapshot, model.IssueOpsCleanupFinishFailure{Step: step, Message: errors.Join(cause, drainErr).Error(), At: s.Now().UTC().Format(time.RFC3339Nano)}, drained)
		result.NextCommand = fmt.Sprintf("issueops cleanup finish --id %s --preview --json", record.ID)
		return result, fmt.Errorf("cleanup finish step %s failed (record preserved; re-run preview then apply): %w", step, errors.Join(cause, drainErr, receiptErr))
	}
	if inventory.WorktreePresent && (len(result.WorkspaceProcesses) > 0 || len(inventory.OrcaTerminals) > 0 || inventory.OrcaRuntimeReady) {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepWorkspaceProcessesStop, err)
		}
		result.WorkspaceProcessesStopped, result.OrcaTerminalsStopped, err = s.Stop(ctx, inventory, result.WorkspaceProcesses)
		if err != nil {
			return fail(model.CleanupFailureStepWorkspaceProcessesStop, err)
		}
	}
	if inventory.OrcaWorktreeID != "" {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepOrcaRemove, err)
		}
		if s.RemoveOrca == nil {
			return fail(model.CleanupFailureStepOrcaRemove, fmt.Errorf("orca worktree remover is not configured"))
		}
		if err := s.RemoveOrca(ctx, inventory.OrcaWorktreeID); err != nil {
			return fail(model.CleanupFailureStepOrcaRemove, err)
		}
		result.OrcaRemoved = true
	}
	worktreePresent := inventory.WorktreePresent
	if worktreePresent && result.OrcaRemoved {
		worktreePresent, err = s.Directory(inventory.WorktreeRoot)
		if err != nil {
			return fail(model.CleanupFailureStepWorktreeRemove, err)
		}
		if !worktreePresent {
			result.WorktreeRemoved = true
		}
	}
	if worktreePresent {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepWorktreeRemove, err)
		}
		if code, out := s.Git(ctx, record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			present, observeErr := s.Directory(inventory.WorktreeRoot)
			if observeErr != nil || present {
				return fail(model.CleanupFailureStepWorktreeRemove, errors.Join(fmt.Errorf("git worktree remove: %s", out), observeErr))
			}
		}
		result.WorktreeRemoved = true
	}
	if inventory.BranchOID != "" {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepBranchDelete, err)
		}
		if code, out := s.Git(ctx, record.Repo, "update-ref", "-d", "refs/heads/"+inventory.Branch, inventory.BranchOID); code != 0 {
			observed, _ := s.Git(ctx, record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/"+inventory.Branch)
			if observed != 1 {
				return fail(model.CleanupFailureStepBranchDelete, fmt.Errorf("git update-ref -d: %s", out))
			}
		}
		result.BranchDeleted = true
	}
	if s.ReflectAudit != nil {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepRecordDelete, err)
		}
		audit := domain.CleanupFinishAudit(inventory, result, s.Now().UTC().Format(time.RFC3339))
		if err := s.ReflectAudit(ctx, record, completion, audit); err != nil {
			result.AuditError = err.Error()
		} else {
			next, err := s.Records.MarkAuditReflected(ctx, snapshot, s.Now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				return fail(model.CleanupFailureStepRecordDelete, err)
			}
			snapshot = next
			result.AuditReflected = true
		}
	}
	if err := drain(); err != nil {
		return fail(model.CleanupFailureStepRecordDelete, err)
	}
	if err := s.Records.Delete(finalizeCtx, snapshot); err != nil {
		return fail(model.CleanupFailureStepRecordDelete, err)
	}
	result.RecordDeleted = true
	return result, nil
}
