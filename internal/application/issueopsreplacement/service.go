package issueopsreplacement

import (
	"context"
	"fmt"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

type Service struct {
	Records          port.ReplacementRecords
	Workspace        port.ReplacementWorkspace
	ObserveProcesses func() port.ReplacementProcessSnapshot
	OrcaOwner        port.ExecutionOrcaOwnerInspector
	PID              int
	ResealOwner      func(context.Context, issueops.IssueOpsRecord) (issueops.ReplacementArtifacts, error)
	Artifacts        port.ReplacementArtifacts
	BaseSync         basesyncport.Inspector
	InspectProcess   func(issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error)
	Now              func() string
}

func (s Service) Run(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	actor, err := cycleapp.NormalizeNativeActor(req.Actor, s.InspectProcess)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	req.Actor = actor
	switch req.Action {
	case issueops.ExecutionReplacePreview:
		return s.previewExecutionReplacement(ctx, req)
	case issueops.ExecutionReplaceFinalizePreview:
		return s.previewExecutionFinalization(ctx, req)
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
	fingerprint, _, err := s.inventory(ctx, record, req.Actor)
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

func (s Service) previewExecutionFinalization(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
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
	fingerprint, err := s.quiescence(ctx, record, req.Actor)
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	return replaceResult(record, req.Action, "", fingerprint, ""), nil
}

func (s Service) mutateExecutionReplacement(ctx context.Context, req issueops.ExecutionReplaceRequest) (issueops.ExecutionReplaceResult, error) {
	var persisted issueops.IssueOpsRecord
	var tokenPath string
	var resealed issueops.ReplacementArtifacts
	err := s.Records.WithinLock(ctx, req.ID, func() error {
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
			if err := domain.ValidateReplacementRevoke(*lease, req.Reason); err != nil {
				return err
			}
			if err := s.refuseSelfRevoke(record.ID, *lease, req.Actor); err != nil {
				return err
			}
			fingerprint, _, err := s.inventory(ctx, record, req.Actor)
			if err != nil {
				return err
			}
			if fingerprint != req.InventoryFingerprint {
				return fmt.Errorf("stale replacement inventory fingerprint")
			}
			previous := *lease.Holder
			*lease = domain.RevokeReplacement(*lease, req.Reason, now)
			persisted, err = s.Records.Persist(record, &previous)
			return err
		case issueops.ExecutionReplaceFinalize:
			if lease.Status != issueops.LeaseStatusRevoking {
				return fmt.Errorf("finalize requires a revoking lease")
			}
			fingerprint, err := s.quiescence(ctx, record, req.Actor)
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
			// token은 그 worktree에서 읽히도록 만들어지고 owner context
			// 재봉인도 거기에 쓴다. 아무도 claim할 수 없는 세대를 만들면서
			// 쓰기에 실패해 lease가 revoking에 갇히고, abandon은
			// claimable/released만 받으므로 회수가 막힌다(#435).
			//
			// terminal 상태인 released가 정확하다. 이 lifecycle은 폐기하거나
			// 처음부터 다시 prepare해야 하며, 두 경로 모두 released에서 열린다.
			if s.Artifacts.WorkspaceAbsent(record.Execution.Workspace.Root) {
				*lease = domain.FinalizeReplacement(*lease, true, "")
				persisted, err = s.Records.Persist(record, nil)
				return err
			}
			token, path, err := s.Artifacts.CreateToken(record)
			if err != nil {
				return s.cleanupFailure(record, err)
			}
			tokenPath = path
			*lease = domain.FinalizeReplacement(*lease, false, token)
			// revoking 세대의 durable 상태는 재봉인이 모두 성공한 뒤에만
			// claimable로 바뀐다. 실패한 token은 즉시 지워 재시도 경로만 남긴다.
			reseal, err := s.ResealOwner(ctx, record)
			if err != nil {
				return s.cleanupFailure(record, err)
			}
			domain.SealReplacementOwner(record.Execution.Orca, lease.Generation, reseal)
			resealed = reseal
			persisted, err = s.Records.Persist(record, nil)
			if err != nil {
				return s.cleanupFailure(record, err)
			}
			return nil
		}
		return fmt.Errorf("unsupported execution replace action %q", req.Action)
	})
	if err != nil {
		return issueops.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(persisted, req.Action, "", "", tokenPath)
	result.IssueBodySHA256 = resealed.IssueBodySHA256
	result.ContextPacketPath, result.ContextPacketSHA256 = resealed.ContextPacketPath, resealed.ContextPacketSHA256
	result.OwnerPromptPath, result.OwnerPromptSHA256 = resealed.OwnerPromptPath, resealed.OwnerPromptSHA256
	if persisted.Execution.Lease.Status == issueops.LeaseStatusClaimable {
		switch persisted.Execution.Mode {
		case issueops.ExecutionModeOrca:
			result.NextCommand = domain.ReplacementResumeCommand(persisted.ID, persisted.Execution.Lease.Generation)
		case issueops.ExecutionModeDirect:
			result.NextCommand = domain.ReplacementClaimCommand(
				persisted.ID,
				persisted.Execution.Lease.Generation,
				tokenPath,
			)
		}
	}
	return result, nil
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
		return fmt.Errorf("%w; replacement residue cleanup failed: %v", cause, err)
	}
	return cause
}
func replaceResult(record issueops.IssueOpsRecord, action, inventory, quiescence, tokenPath string) issueops.ExecutionReplaceResult {
	return issueops.ExecutionReplaceResult{
		OK: true, ID: record.ID, Action: action, Execution: *record.Execution,
		InventoryFingerprint: inventory, QuiescenceFingerprint: quiescence, ClaimTokenPath: tokenPath,
	}
}
