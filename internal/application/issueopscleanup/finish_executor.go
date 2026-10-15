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
	MaterialWarning func(model.IssueOpsRecord) string
	Records         port.CleanupFinishRecords
	Acquire         func(context.Context, string) (CleanupLifetime, error)
	Observe         func(context.Context, model.IssueOpsRecord, model.CleanupFinishRequest) (model.CleanupFinishRequest, error)
	Plan            func(context.Context, model.IssueOpsRecord, model.CleanupFinishRequest) (model.CleanupFinishInventory, model.CleanupFinishResult)
	Fingerprint     func(model.CleanupFinishInventory) (string, error)
	NewAttempt      func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error)
	Stop            func(context.Context, model.CleanupFinishInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, error)
	Directory       func(string) (bool, error)
	Git             func(context.Context, string, ...string) (int, string)
	Now             func() time.Time
}

// finishRun carries the state shared by the phases of one finish execution.
type finishRun struct {
	s           FinishExecutor
	ctx         context.Context
	finalizeCtx context.Context
	req         model.CleanupFinishRequest
	snapshot    model.CleanupSnapshot
	record      model.IssueOpsRecord
	inventory   model.CleanupFinishInventory
	result      model.CleanupFinishResult
	drainer     *cleanupDrainer
}

func (s FinishExecutor) Run(ctx context.Context, req model.CleanupFinishRequest) (model.CleanupFinishResult, error) {
	failed := model.CleanupFinishResult{OK: false, ID: req.ID}
	lifetime, err := s.Acquire(ctx, req.ID)
	if err != nil {
		return failed, err
	}
	drainer := &cleanupDrainer{lifetime: lifetime}
	defer drainer.close()
	r := &finishRun{s: s, ctx: lifetime.Context(ctx), req: req, drainer: drainer}
	if err := r.load(); err != nil {
		return failed, err
	}
	if res, done, err := r.plan(failed); done {
		return res, err
	}
	if err := domain.ValidateCleanupFinishApply(r.req, r.result.Fingerprint); err != nil {
		r.result.OK = false
		return r.result, err
	}
	attempt, err := s.NewAttempt(model.CleanupOperationFinish)
	if err != nil {
		return failed, err
	}
	r.snapshot, err = s.Records.Arm(r.ctx, r.snapshot, attempt)
	if err != nil {
		r.result.OK = false
		return r.result, err
	}
	// Cancellation must stop commands, but must not prevent releasing their
	// durable ownership after the inherited descriptors have safely drained.
	r.finalizeCtx = context.WithoutCancel(r.ctx)
	return r.execute()
}

// load reads the record, validates access, and observes request evidence.
func (r *finishRun) load() error {
	snapshot, err := r.s.Records.Load(r.ctx, r.req.ID)
	if err != nil {
		return err
	}
	r.snapshot = snapshot
	r.record = snapshot.Record
	if err := domain.ValidateCleanupOperationAccess(r.record, model.CleanupOperationFinish); err != nil {
		return err
	}
	r.req, err = r.s.Observe(r.ctx, r.record, r.req)
	if err != nil {
		return &port.CleanupFinishObservationError{Err: err}
	}
	return nil
}

// plan builds the inventory and fingerprint. done reports that Run must
// return (res, err) now: not ready, fingerprint failure, or preview-only.
func (r *finishRun) plan(failed model.CleanupFinishResult) (res model.CleanupFinishResult, done bool, err error) {
	inventory, result := r.s.Plan(r.ctx, r.record, r.req)
	r.inventory, r.result = inventory, result
	if r.s.MaterialWarning != nil {
		if warning := r.s.MaterialWarning(r.record); warning != "" {
			r.result.Warnings = append(r.result.Warnings, warning)
		}
	}
	if len(r.result.Missing) > 0 {
		r.result.OK = false
		r.result.NextCommand = domain.CleanupFinishRemedyCommand(r.record.ID, r.result.Missing)
		return r.result, true, fmt.Errorf("cleanup finish is not ready: %s", strings.Join(r.result.Missing, ", "))
	}
	fingerprint, err := r.s.Fingerprint(inventory)
	if err != nil {
		return failed, true, err
	}
	r.result.Fingerprint = fingerprint
	if !r.req.Apply {
		r.result.NextCommand = domain.CleanupFinishApplyCommand(r.record.ID, fingerprint, r.result.SupersededBy, r.req.KeepRemoteBranch)
		return r.result, true, nil
	}
	return r.result, false, nil
}

func (r *finishRun) fail(step string, cause error) (model.CleanupFinishResult, error) {
	r.result.OK = false
	r.result.FailedStep = step
	drainErr := r.drainer.drain(r.finalizeCtx)
	_, receiptErr := r.s.Records.Fail(r.finalizeCtx, r.snapshot, model.IssueOpsCleanupFinishFailure{Step: step, Message: errors.Join(cause, drainErr).Error(), At: r.s.Now().UTC().Format(time.RFC3339Nano)}, r.drainer.drained)
	r.result.NextCommand = fmt.Sprintf("issueops cleanup finish --id %s --preview --json", r.record.ID)
	return r.result, fmt.Errorf("cleanup finish step %s failed (record preserved; re-run preview then apply): %w", step, errors.Join(cause, drainErr, receiptErr))
}

// execute runs the armed apply steps in order and finalizes the record.
func (r *finishRun) execute() (model.CleanupFinishResult, error) {
	steps := []struct {
		step string
		run  func() error
	}{
		{model.CleanupFailureStepWorkspaceProcessesStop, r.stopWorkspaceProcesses},
		{model.CleanupFailureStepWorktreeRemove, r.removeWorktree},
		{model.CleanupFailureStepBranchDelete, r.deleteBranch},
	}
	for _, st := range steps {
		if err := st.run(); err != nil {
			return r.fail(st.step, err)
		}
	}
	r.result.Audit = domain.CleanupFinishAudit(r.inventory, r.result, r.s.Now().UTC().Format(time.RFC3339))
	if err := r.drainer.drain(r.finalizeCtx); err != nil {
		return r.fail(model.CleanupFailureStepRecordDelete, err)
	}
	if err := r.s.Records.Delete(r.finalizeCtx, r.snapshot); err != nil {
		return r.fail(model.CleanupFailureStepRecordDelete, err)
	}
	r.result.RecordDeleted = true
	return r.result, nil
}

func (r *finishRun) stopWorkspaceProcesses() error {
	inv := r.inventory
	if !inv.WorktreePresent || len(r.result.WorkspaceProcesses) == 0 {
		return nil
	}
	if err := r.s.Records.Check(r.ctx, r.snapshot); err != nil {
		return err
	}
	var err error
	r.result.WorkspaceProcessesStopped, err = r.s.Stop(r.ctx, inv, r.result.WorkspaceProcesses)
	return err
}

func (r *finishRun) removeWorktree() error {
	if !r.inventory.WorktreePresent {
		return nil
	}
	if err := r.s.Records.Check(r.ctx, r.snapshot); err != nil {
		return err
	}
	if code, out := r.s.Git(r.ctx, r.record.Repo, "worktree", "remove", r.inventory.WorktreeRoot); code != 0 {
		present, observeErr := r.s.Directory(r.inventory.WorktreeRoot)
		if observeErr != nil || present {
			return errors.Join(fmt.Errorf("git worktree remove: %s", out), observeErr)
		}
	}
	r.result.WorktreeRemoved = true
	return nil
}

func (r *finishRun) deleteBranch() error {
	if r.inventory.BranchOID == "" {
		return nil
	}
	if err := r.s.Records.Check(r.ctx, r.snapshot); err != nil {
		return err
	}
	ref := "refs/heads/" + r.inventory.Branch
	if code, out := r.s.Git(r.ctx, r.record.Repo, "update-ref", "-d", ref, r.inventory.BranchOID); code != 0 {
		observed, _ := r.s.Git(r.ctx, r.record.Repo, "show-ref", "--verify", "--quiet", ref)
		if observed != 1 {
			return fmt.Errorf("git update-ref -d: %s", out)
		}
	}
	r.result.BranchDeleted = true
	return nil
}
