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
	Stop       func(context.Context, model.CleanupAbandonInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error)
	Directory  func(string) (bool, error)
	Git        func(context.Context, string, ...string) (int, string)
	Now        func() time.Time
}

func (s AbandonExecutor) Run(ctx context.Context, req model.CleanupAbandonRequest) (model.CleanupAbandonResult, error) {
	failed := model.CleanupAbandonResult{OK: false, ID: req.ID}
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
	if err := domain.ValidateCleanupOperationAccess(record, model.CleanupOperationAbandon); err != nil {
		return failed, err
	}
	var provider port.IssueProvider
	if record.RemoteArtifact != nil || req.ClosePR || req.CloseIssue || req.DeleteRemoteBranch {
		name := domain.ResolveRecordProvider(record)
		if name == "" || s.Provider == nil {
			return failed, fmt.Errorf("cannot determine provider from IssueOps record; remote abandon evidence needs one")
		}
		provider, err = s.Provider(name)
		if err != nil {
			return failed, err
		}
	}
	req, err = s.Observe(ctx, record, req, provider)
	if err != nil {
		return failed, err
	}
	inventory, result := s.Plan(ctx, record, req, provider)
	if len(result.Missing) > 0 {
		return result, fmt.Errorf("cleanup abandon is not ready: %s", strings.Join(result.Missing, ", "))
	}
	fingerprint, err := CleanupAbandonFingerprint(inventory)
	if err != nil {
		return failed, err
	}
	result.Fingerprint = fingerprint
	result.RemovalPlan = domain.CleanupAbandonRemovalPlan(record, inventory)
	result.Record = &record
	if !req.Apply {
		result.NextCommand = fmt.Sprintf("issueops cleanup abandon --id %s --reason %q%s --apply --confirm --fingerprint %s --json", record.ID, result.Reason, domain.CleanupAbandonRemoteFlags(req), fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("cleanup abandon --apply requires --confirm")
	}
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	attempt, err := s.NewAttempt(model.CleanupOperationAbandon)
	if err != nil {
		return failed, err
	}
	failure := abandonFailure(record, inventory, fingerprint, model.CleanupFailureStepApplying, "", attempt.StartedAt)
	snapshot, err = s.Records.ArmAbandon(ctx, snapshot, attempt, failure)
	if err != nil {
		result.OK = false
		return result, err
	}
	finalizeCtx := context.WithoutCancel(ctx)
	drainageAttempted, drained := false, false
	var drainageErr error
	drain := func() error {
		if drainageAttempted {
			return drainageErr
		}
		drainageAttempted = true
		lifetime, drainageErr = lifetime.Drain(finalizeCtx)
		drained = drainageErr == nil
		return drainageErr
	}
	fail := func(step string, cause error) (model.CleanupAbandonResult, error) {
		result.OK = false
		result.FailedStep = step
		drainErr := drain()
		receiptStep := step
		// A cancelled local command can have removed its target before cancellation
		// reached the parent. Preserve the sealed applying state until fresh preview
		// observes which prefix of worktree -> branch removal actually completed.
		if (step == model.CleanupFailureStepWorktreeRemove || step == model.CleanupFailureStepBranchDelete) &&
			(errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded)) {
			receiptStep = model.CleanupFailureStepApplying
		}
		receipt := abandonFailure(record, inventory, fingerprint, receiptStep, errors.Join(cause, drainErr).Error(), s.Now().UTC().Format(time.RFC3339Nano))
		_, receiptErr := s.Records.FailAbandon(finalizeCtx, snapshot, receipt, drained)
		result.NextCommand = domain.CleanupAbandonPreviewCommand(record.ID, result.Reason, req)
		return result, fmt.Errorf("cleanup abandon %s failed (record preserved; re-run preview then apply): %w", step, errors.Join(cause, drainErr, receiptErr))
	}
	if step, err := s.applyRemote(ctx, snapshot, req, provider, inventory, &result); err != nil {
		return fail(step, err)
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
	if inventory.WorktreePresent {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return fail(model.CleanupFailureStepWorktreeRemove, err)
		}
		if code, out := s.Git(ctx, record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			if err := ctx.Err(); err != nil {
				return fail(model.CleanupFailureStepWorktreeRemove, err)
			}
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
			if err := ctx.Err(); err != nil {
				return fail(model.CleanupFailureStepBranchDelete, err)
			}
			observed, _ := s.Git(ctx, record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/"+inventory.Branch)
			if err := ctx.Err(); err != nil {
				return fail(model.CleanupFailureStepBranchDelete, err)
			}
			if observed != 1 {
				return fail(model.CleanupFailureStepBranchDelete, fmt.Errorf("git update-ref -d: %s", out))
			}
		}
		result.BranchDeleted = true
	}
	if err := drain(); err != nil {
		return fail(model.CleanupFailureStepRecordDelete, err)
	}
	result.IntentRowsDeleted, err = s.Records.DeleteAbandoned(finalizeCtx, snapshot)
	if err != nil {
		return fail(model.CleanupFailureStepRecordDelete, err)
	}
	result.RecordDeleted = true
	result.AbandonedAt = s.Now().UTC().Format(time.RFC3339)
	return result, nil
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
	record := snapshot.Record
	result.RemoteEffects = []string{}
	if req.ClosePR {
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
	}
	if req.CloseIssue {
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
	}
	if req.DeleteRemoteBranch {
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
		}
		result.RemoteEffects = append(result.RemoteEffects, effect)
	}
	return "", nil
}
