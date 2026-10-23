package issueopsreplacement

import (
	"context"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

type Service struct {
	Records          port.ReplacementRecords
	Workspace        port.ReplacementWorkspace
	ObserveProcesses func() port.ReplacementProcessSnapshot
	PID              int
	Artifacts        port.ReplacementArtifacts
	BaseSync         basesyncport.Inspector
	InspectProcess   func(issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error)
	Verifier         authorityport.ActorVerifier
	Now              func() string
}

func (s Service) Run(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	if s.Verifier == nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("execution replace actor verifier is required")
	}
	verified, err := s.Verifier.Verify(ctx, req.Actor)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	req.Actor = verified.Identity
	switch req.Action {
	case issueops.ExecutionReplacePreview:
		return s.previewExecutionReplacement(ctx, req)
	case issueops.ExecutionReplaceFinalizePreview:
		return s.previewExecutionFinalization(req)
	case issueops.ExecutionReplaceRevoke, issueops.ExecutionReplaceFinalize:
		if !req.Confirm {
			return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("%s requires confirm", req.Action)
		}
		return s.mutateExecutionReplacement(ctx, req)
	default:
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("unsupported execution replace action %q", req.Action)
	}
}

func (s Service) previewExecutionReplacement(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	record, err := s.Records.Load(req.ID)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if err := domain.ValidateReplacementGeneration(record, req.ExpectedGeneration, true); err != nil {
		return issueops.ExecutionReplaceResult{ID: req.ID, Action: req.Action}, err
	}
	if err := domain.ValidateReplacementPreview(record.Execution.Lease.Status); err != nil {
		return issueops.ExecutionReplaceResult{ID: req.ID, Action: req.Action}, err
	}
	if err := s.validateCWD(record, req.CWD); err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if err := observeCompletedExecutionBase(ctx, record, req.CompletionGeneration, s.BaseSync); err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	fingerprint, err := s.inventory(record, req.Actor)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(record, req.Action, fingerprint, "", "")
	switch {
	case record.Execution.Lease.Status == issueops.LeaseStatusReleased ||
		record.Execution.Lease.Status == issueops.LeaseStatusClaimable:
		result.NextCommand = domain.ReplacementReseedCommand(
			record.ID,
			record.Execution.Lease.Generation,
			req.CompletionGeneration,
			fingerprint,
			req.Actor,
			record.Execution.Workspace.Root,
		)
	case record.Execution.Lease.Status == issueops.LeaseStatusActive &&
		s.refuseSelfRevoke(record.ID, record.Execution.Lease, req.Actor) == nil:
		// 인수 체인의 다음 걸음이다. 여기서 비워 두면 죽은 홀더를 인수하려는
		// 세션이 다음 명령을 스스로 지어내야 하고, 그것이 라우터가 금지하는
		// 바로 그 추측이다. `--reason`만 사람이 채우므로 template으로 렌더한다.
		result.NextCommand = domain.ReplacementRevokeCommand(
			record.ID,
			record.Execution.Lease.Generation,
			fingerprint,
			req.Actor,
			record.Execution.Workspace.Root,
		)
	}
	return result, nil
}

func observeCompletedExecutionBase(ctx context.Context, record issueops.IssueOpsRecord, selectedGeneration uint64, inspector basesyncport.Inspector) error {
	needed, err := domain.CompletedReplacementBase(record, selectedGeneration)
	if err != nil || !needed {
		return err
	}
	execution := record.Execution
	completionGeneration := execution.Completion.Generation
	if inspector == nil {
		return fmt.Errorf("completed replacement preview requires base sync inspector")
	}
	receipt, err := inspector.Observe(ctx, basesyncport.Request{
		Worktree: execution.Workspace.Root, BaseBranch: record.BranchPrepare.BaseBranch,
	})
	if err != nil {
		return fmt.Errorf("observe completed execution base: %w", err)
	}
	if receipt.SyncRequired {
		return issueops.NewBaseSyncRequiredError(record.ID, completionGeneration)
	}
	return nil
}

func (s Service) previewExecutionFinalization(req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	record, err := s.recordAtGeneration(req.ID, req.ExpectedGeneration)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if record.Execution.Lease.Status != issueops.LeaseStatusRevoking {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("finalize preview requires a revoking lease")
	}
	if err := s.validateCWD(record, req.CWD); err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	fingerprint, err := s.quiescence(record, req.Actor)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(record, req.Action, "", fingerprint, "")
	// The finalize step needs exactly this fingerprint and the caller's actor;
	// render it so the taking-over session does not assemble flags by hand.
	result.NextCommand = domain.ReplacementFinalizeCommand(record.ID, record.Execution.Lease.Generation, fingerprint, req.Actor, record.Execution.Workspace.Root)
	return result, nil
}

func (s Service) mutateExecutionReplacement(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	var persisted issueops.IssueOpsRecord
	var tokenPath string
	err := s.Records.WithinLock(ctx, req.ID, func(spanCtx context.Context) error {
		record, err := s.recordAtGeneration(req.ID, req.ExpectedGeneration)
		if err != nil {
			return err
		}
		if record.Execution.Pending != nil {
			return fmt.Errorf("execution replacement is blocked by a pending external intent; run execution reconcile")
		}
		lease := &record.Execution.Lease
		if err := s.validateCWD(record, req.CWD); err != nil {
			return err
		}
		now := s.Now()
		switch req.Action {
		case issueops.ExecutionReplaceRevoke:
			return s.revokeReplacement(spanCtx, req, record, lease, now, &persisted)
		case issueops.ExecutionReplaceFinalize:
			return s.finalizeReplacement(spanCtx, req, record, lease, &persisted, &tokenPath)
		}
		return fmt.Errorf("unsupported execution replace action %q", req.Action)
	})
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(persisted, req.Action, "", "", tokenPath)
	switch persisted.Execution.Lease.Status {
	case issueops.LeaseStatusClaimable:
		result.NextCommand = domain.ReplacementClaimCommandAs(persisted.ID, persisted.Execution.Lease.Generation, req.Actor, persisted.Execution.Workspace.Root)
	case issueops.LeaseStatusRevoking:
		result.NextCommand = domain.ReplacementFinalizePreviewCommand(persisted.ID, persisted.Execution.Lease.Generation)
	}
	return result, nil
}

func (s Service) revokeReplacement(spanCtx context.Context, req issueops.ExecutionReplaceRequest, record issueops.IssueOpsRecord, lease *issueops.WriteLease, now string, persisted *issueops.IssueOpsRecord) error {
	if err := domain.ValidateReplacementRevoke(*lease, req.Reason); err != nil {
		return err
	}
	if err := s.refuseSelfRevoke(record.ID, *lease, req.Actor); err != nil {
		return err
	}
	fingerprint, err := s.inventory(record, req.Actor)
	if err != nil {
		return err
	}
	if fingerprint != req.InventoryFingerprint {
		return fmt.Errorf("stale replacement inventory fingerprint")
	}
	previous := *lease.Holder
	*lease = domain.RevokeReplacement(*lease, req.Reason, now)
	*persisted, err = s.Records.Persist(spanCtx, record, &previous)
	return err
}

func (s Service) finalizeReplacement(spanCtx context.Context, req issueops.ExecutionReplaceRequest, record issueops.IssueOpsRecord, lease *issueops.WriteLease, persisted *issueops.IssueOpsRecord, tokenPath *string) error {
	if lease.Status != issueops.LeaseStatusRevoking {
		return fmt.Errorf("finalize requires a revoking lease")
	}
	fingerprint, err := s.quiescence(record, req.Actor)
	if err != nil {
		return err
	}
	if fingerprint != req.QuiescenceFingerprint {
		return fmt.Errorf("stale quiescence fingerprint")
	}
	if err := s.Artifacts.Cleanup(record); err != nil {
		return err
	}
	// 넘겨줄 workspace가 없으면 claimable은 사실이 아니다 — claim
	// token은 그 worktree에서 읽히도록 만들어진다. 아무도 claim할 수
	// 없는 세대를 만들면서 쓰기에 실패해 lease가 revoking에 갇히고,
	// abandon은 claimable/released만 받으므로 회수가 막힌다(#435).
	//
	// terminal 상태인 released가 정확하다. 이 lifecycle은 폐기하거나
	// 처음부터 다시 prepare해야 하며, 두 경로 모두 released에서 열린다.
	if s.Artifacts.WorkspaceAbsent(record.Execution.Workspace.Root) {
		*lease = domain.FinalizeReplacement(*lease, true, "")
		*persisted, err = s.Records.Persist(spanCtx, record, nil)
		return err
	}
	token, path, err := s.Artifacts.CreateToken(record)
	if err != nil {
		return s.cleanupFailure(record, err)
	}
	*tokenPath = path
	*lease = domain.FinalizeReplacement(*lease, false, token)
	// 실패한 token은 즉시 지워 재시도 경로만 남긴다.
	*persisted, err = s.Records.Persist(spanCtx, record, nil)
	if err != nil {
		return s.cleanupFailure(record, err)
	}
	return nil
}

func (s Service) recordAtGeneration(id string, generation uint64) (issueops.IssueOpsRecord, error) {
	record, err := s.Records.Load(id)
	if err != nil {
		return record, err
	}
	return record, domain.ValidateReplacementGeneration(record, generation, false)
}
func (s Service) cleanupFailure(record issueops.IssueOpsRecord, cause error) error {
	if err := s.Artifacts.Cleanup(record); err != nil {
		return fmt.Errorf("%w; replacement residue cleanup failed: %v", cause, err) //nolint:errorlint // only the cause belongs in the chain; the cleanup failure is diagnostic text
	}
	return cause
}
func replaceResult(record issueops.IssueOpsRecord, action, inventory, quiescence, tokenPath string) issueops.ExecutionReplaceResult {
	return issueops.ExecutionReplaceResult{
		OK: true, ID: record.ID, Action: action, Execution: *record.Execution,
		InventoryFingerprint: inventory, QuiescenceFingerprint: quiescence, ClaimTokenPath: tokenPath,
	}
}
