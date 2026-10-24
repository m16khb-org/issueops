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

type AbandonExecutor struct {
	Records    port.CleanupAbandonRecords
	Acquire    func(context.Context, string) (CleanupLifetime, error)
	Provider   func(string) (port.IssueProvider, error)
	Observe    func(context.Context, model.IssueOpsRecord, model.CleanupAbandonRequest, port.IssueProvider) (model.CleanupAbandonRequest, error)
	Plan       func(context.Context, model.IssueOpsRecord, model.CleanupAbandonRequest, port.IssueProvider) (model.CleanupAbandonInventory, model.CleanupAbandonResult)
	NewAttempt func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error)
	Stop       func(context.Context, model.CleanupAbandonInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, error)
	Directory  func(string) (bool, error)
	// PruneWorktreesParent removes the emptied `<repo>.worktrees` parent; optional.
	PruneWorktreesParent func(string)
	Git                  func(context.Context, string, ...string) (int, string)
	Now                  func() time.Time
}

// abandonRun carries the state shared by the phases of one abandon execution.
type abandonRun struct {
	s           AbandonExecutor
	ctx         context.Context
	finalizeCtx context.Context
	req         model.CleanupAbandonRequest
	snapshot    model.CleanupSnapshot
	record      model.IssueOpsRecord
	provider    port.IssueProvider
	inventory   model.CleanupAbandonInventory
	fingerprint string
	result      model.CleanupAbandonResult
	drainer     *cleanupDrainer
}

func (s AbandonExecutor) Run(ctx context.Context, req model.CleanupAbandonRequest) (model.CleanupAbandonResult, error) {
	failed := model.CleanupAbandonResult{OK: false, ID: req.ID}
	lifetime, err := s.Acquire(ctx, req.ID)
	if err != nil {
		return failed, err
	}
	drainer := &cleanupDrainer{lifetime: lifetime}
	defer drainer.close()
	r := &abandonRun{s: s, ctx: lifetime.Context(ctx), req: req, drainer: drainer}
	if err := r.load(); err != nil {
		return failed, err
	}
	if res, err := r.plan(failed); err != nil {
		return res, err
	}
	if !r.req.Apply {
		r.result.NextCommand = fmt.Sprintf("issueops cleanup abandon --id %s --reason %q%s --apply --confirm --fingerprint %s --json", r.record.ID, r.result.Reason, domain.CleanupAbandonRemoteFlags(r.req), r.fingerprint)
		return r.result, nil
	}
	if err := r.authorizeApply(); err != nil {
		r.result.OK = false
		return r.result, err
	}
	attempt, err := s.NewAttempt(model.CleanupOperationAbandon)
	if err != nil {
		return failed, err
	}
	failure := abandonFailure(r.record, r.inventory, r.fingerprint, model.CleanupFailureStepApplying, "", attempt.StartedAt)
	r.snapshot, err = s.Records.ArmAbandon(r.ctx, r.snapshot, attempt, failure)
	if err != nil {
		r.result.OK = false
		return r.result, err
	}
	// Cancellation must stop commands, but must not prevent releasing their
	// durable ownership after the inherited descriptors have safely drained.
	r.finalizeCtx = context.WithoutCancel(r.ctx)
	return r.execute()
}

// load reads the record, validates access, resolves the provider, and
// observes remote evidence.
func (r *abandonRun) load() error {
	snapshot, err := r.s.Records.Load(r.ctx, r.req.ID)
	if err != nil {
		return err
	}
	r.snapshot = snapshot
	r.record = snapshot.Record
	if err := domain.ValidateCleanupOperationAccess(r.record, model.CleanupOperationAbandon); err != nil {
		return err
	}
	if err := r.resolveProvider(); err != nil {
		return err
	}
	r.req, err = r.s.Observe(r.ctx, r.record, r.req, r.provider)
	return err
}

func (r *abandonRun) resolveProvider() error {
	if r.record.RemoteArtifact == nil && !r.req.ClosePR && !r.req.CloseIssue && !r.req.DeleteRemoteBranch {
		return nil
	}
	name := domain.ResolveRecordProvider(r.record)
	if name == "" || r.s.Provider == nil {
		return fmt.Errorf("cannot determine provider from IssueOps record; remote abandon evidence needs one")
	}
	provider, err := r.s.Provider(name)
	if err != nil {
		return err
	}
	r.provider = provider
	return nil
}

// plan builds the inventory and fingerprint; failed is returned for errors
// that originally carried the generic failed result.
func (r *abandonRun) plan(failed model.CleanupAbandonResult) (model.CleanupAbandonResult, error) {
	inventory, result := r.s.Plan(r.ctx, r.record, r.req, r.provider)
	r.inventory, r.result = inventory, result
	if len(result.Missing) > 0 {
		return result, fmt.Errorf("cleanup abandon is not ready; unmet gates: %s", strings.Join(result.Missing, ", "))
	}
	fingerprint, err := CleanupAbandonFingerprint(inventory)
	if err != nil {
		return failed, err
	}
	r.fingerprint = fingerprint
	r.result.Fingerprint = fingerprint
	r.result.RemovalPlan = domain.CleanupAbandonRemovalPlan(r.record, inventory)
	r.result.Record = &r.record
	return r.result, nil
}

func (r *abandonRun) authorizeApply() error {
	if !r.req.Confirm {
		return fmt.Errorf("cleanup abandon --apply requires --confirm")
	}
	if r.req.Fingerprint != r.fingerprint {
		return fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	return nil
}

func (r *abandonRun) fail(step string, cause error) (model.CleanupAbandonResult, error) {
	r.result.OK = false
	r.result.FailedStep = step
	drainErr := r.drainer.drain(r.finalizeCtx)
	receiptStep := step
	// A cancelled local command can have removed its target before cancellation
	// reached the parent. Preserve the sealed applying state until fresh preview
	// observes which prefix of worktree -> branch removal actually completed.
	if (step == model.CleanupFailureStepWorktreeRemove || step == model.CleanupFailureStepBranchDelete) &&
		(errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded)) {
		receiptStep = model.CleanupFailureStepApplying
	}
	receipt := abandonFailure(r.record, r.inventory, r.fingerprint, receiptStep, errors.Join(cause, drainErr).Error(), r.s.Now().UTC().Format(time.RFC3339Nano))
	_, receiptErr := r.s.Records.FailAbandon(r.finalizeCtx, r.snapshot, receipt, r.drainer.drained)
	r.result.NextCommand = domain.CleanupAbandonPreviewCommand(r.record.ID, r.result.Reason, r.req)
	return r.result, fmt.Errorf("cleanup abandon %s failed (record preserved; re-run preview then apply): %w", step, errors.Join(cause, drainErr, receiptErr))
}

// execute runs the armed apply steps in order and finalizes the record.
func (r *abandonRun) execute() (model.CleanupAbandonResult, error) {
	if step, err := r.s.applyRemote(r.ctx, r.snapshot, r.req, r.provider, r.inventory, &r.result); err != nil {
		return r.fail(step, err)
	}
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
	if err := r.drainer.drain(r.finalizeCtx); err != nil {
		return r.fail(model.CleanupFailureStepRecordDelete, err)
	}
	var err error
	outcome := domain.CycleOutcomeForRecord(r.snapshot.Record, model.IssueOpsCycleOutcomeAbandoned, r.s.Now().UTC().Format(time.RFC3339))
	r.result.IntentRowsDeleted, err = r.s.Records.DeleteAbandoned(r.finalizeCtx, r.snapshot, outcome)
	if err != nil {
		return r.fail(model.CleanupFailureStepRecordDelete, err)
	}
	r.result.RecordDeleted = true
	r.result.AbandonedAt = r.s.Now().UTC().Format(time.RFC3339)
	return r.result, nil
}

func (r *abandonRun) stopWorkspaceProcesses() error {
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

func (r *abandonRun) removeWorktree() error {
	if !r.inventory.WorktreePresent {
		return nil
	}
	if err := r.s.Records.Check(r.ctx, r.snapshot); err != nil {
		return err
	}
	removal := worktreeRemoval{Git: r.s.Git, Directory: r.s.Directory, PruneParent: r.s.PruneWorktreesParent, HonorCancel: true}
	if err := removal.remove(r.ctx, r.record.Repo, r.inventory.WorktreeRoot); err != nil {
		return err
	}
	r.result.WorktreeRemoved = true
	return nil
}

func (r *abandonRun) deleteBranch() error {
	if r.inventory.BranchOID == "" {
		return nil
	}
	if err := r.s.Records.Check(r.ctx, r.snapshot); err != nil {
		return err
	}
	ref := "refs/heads/" + r.inventory.Branch
	if code, out := r.s.Git(r.ctx, r.record.Repo, "update-ref", "-d", ref, r.inventory.BranchOID); code != 0 {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		observed, _ := r.s.Git(r.ctx, r.record.Repo, "show-ref", "--verify", "--quiet", ref)
		if err := r.ctx.Err(); err != nil {
			return err
		}
		if observed != 1 {
			return fmt.Errorf("git update-ref -d: %s", out)
		}
	}
	r.result.BranchDeleted = true
	return nil
}

func abandonFailure(record model.IssueOpsRecord, inventory model.CleanupAbandonInventory, fingerprint, step, message, now string) model.IssueOpsCleanupAbandonFailure {
	failure := model.IssueOpsCleanupAbandonFailure{Step: step, Message: message, Fingerprint: fingerprint, RecordSHA: inventory.RecordSHA, WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch, WorktreeHead: inventory.WorktreeHead, BranchOID: inventory.BranchOID, At: now}
	failure.InventorySHA256 = CleanupAbandonFailureSeal(record, &failure)
	return failure
}

// ObserveAbandonArtifact binds remote evidence to the record already loaded
// under the execution lifetime. A caller-supplied boolean is not authority.
func ObserveAbandonArtifact(ctx context.Context, record model.IssueOpsRecord, req model.CleanupAbandonRequest, provider port.IssueProvider) (model.CleanupAbandonRequest, error) {
	req.ArtifactUnmerged = false
	if record.RemoteArtifact == nil {
		return req, nil
	}
	reader, ok := provider.(port.IssueProviderArtifactBodyReader)
	if !ok {
		return req, fmt.Errorf("provider does not support reading artifact state")
	}
	artifact := record.RemoteArtifact
	kind := strings.TrimSpace(artifact.Kind)
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	body, err := reader.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Repo: record.Repo, Kind: kind, URL: artifact.URL})
	if err != nil {
		return req, fmt.Errorf("abandon artifact readback failed: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(body.State)) {
	case "open", "opened", "closed":
		req.ArtifactUnmerged = true
	case "merged":
	default:
		return req, fmt.Errorf("abandon artifact state is unknown: %q", body.State)
	}
	return req, nil
}

func (s AbandonExecutor) applyRemote(ctx context.Context, snapshot model.CleanupSnapshot, req model.CleanupAbandonRequest, provider port.IssueProvider, inventory model.CleanupAbandonInventory, result *model.CleanupAbandonResult) (string, error) {
	if !req.ClosePR && !req.CloseIssue && !req.DeleteRemoteBranch {
		return "", nil
	}
	result.RemoteEffects = []string{}
	if req.ClosePR {
		if step, err := s.closeRemotePR(ctx, snapshot, provider, result); err != nil {
			return step, err
		}
	}
	if req.CloseIssue {
		if step, err := s.closeRemoteIssue(ctx, snapshot, provider, result); err != nil {
			return step, err
		}
	}
	if req.DeleteRemoteBranch {
		if step, err := s.deleteRemoteBranch(ctx, snapshot, inventory, result); err != nil {
			return step, err
		}
	}
	return "", nil
}

func (s AbandonExecutor) closeRemotePR(ctx context.Context, snapshot model.CleanupSnapshot, provider port.IssueProvider, result *model.CleanupAbandonResult) (string, error) {
	record := snapshot.Record
	if err := s.Records.Check(ctx, snapshot); err != nil {
		return model.CleanupFailureStepClosePR, err
	}
	closer, ok := provider.(port.IssueProviderPullRequestCloser)
	if !ok {
		return model.CleanupFailureStepClosePR, fmt.Errorf("provider does not support closing a pull request")
	}
	closed, err := closer.ClosePullRequest(ctx, port.IssueProviderClosePullRequestRequest{Repo: record.Repo, ArtifactURL: record.RemoteArtifact.URL, Kind: strings.TrimSpace(record.RemoteArtifact.Kind), Confirm: true})
	if err != nil {
		return model.CleanupFailureStepClosePR, err
	}
	if closed.Merged {
		return model.CleanupFailureStepClosePR, fmt.Errorf("pull request was merged after the preview; run reflect-completion and cleanup finish instead")
	}
	effect := "close_pr"
	if closed.AlreadyClosed {
		effect += ":already_closed"
	}
	result.RemoteEffects = append(result.RemoteEffects, effect)
	result.RemoteArtifactState = closed.State
	result.PRClosed = closed.Closed
	return "", nil
}

func (s AbandonExecutor) closeRemoteIssue(ctx context.Context, snapshot model.CleanupSnapshot, provider port.IssueProvider, result *model.CleanupAbandonResult) (string, error) {
	record := snapshot.Record
	if err := s.Records.Check(ctx, snapshot); err != nil {
		return model.CleanupFailureStepCloseIssue, err
	}
	closed, err := provider.CloseIssue(ctx, port.IssueProviderCloseIssueRequest{Repo: record.Repo, IssueURL: record.IssueURL, Reason: "not_planned", Confirm: true})
	if err != nil {
		return model.CleanupFailureStepCloseIssue, err
	}
	effect := "close_issue"
	if closed.AlreadyClosed {
		effect += ":already_closed"
	}
	result.RemoteEffects = append(result.RemoteEffects, effect)
	result.IssueState = closed.State
	result.IssueClosed = closed.Closed
	return "", nil
}

func (s AbandonExecutor) deleteRemoteBranch(ctx context.Context, snapshot model.CleanupSnapshot, inventory model.CleanupAbandonInventory, result *model.CleanupAbandonResult) (string, error) {
	record := snapshot.Record
	if err := s.Records.Check(ctx, snapshot); err != nil {
		return model.CleanupFailureStepRemoteBranchDelete, err
	}
	effect := "remote_branch_delete"
	if inventory.RemoteBranchOID == "" {
		effect += ":absent"
	} else {
		ref := "refs/heads/" + inventory.Branch
		if code, out := s.Git(ctx, record.Repo, "push", "origin", "--delete", ref, "--force-with-lease="+ref+":"+inventory.RemoteBranchOID); code != 0 {
			return model.CleanupFailureStepRemoteBranchDelete, fmt.Errorf("%s", strings.TrimSpace(out))
		}
		result.RemoteBranchDeleted = true
		// The preview's "planned" would contradict remote_branch_deleted:true
		// in the apply result (QA read the two fields as inconsistent).
		result.RemoteBranchDeletion = "deleted"
	}
	result.RemoteEffects = append(result.RemoteEffects, effect)
	return "", nil
}
