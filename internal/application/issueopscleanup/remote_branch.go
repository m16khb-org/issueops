package issueopscleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type RemoteBranchCleaner struct {
	Records      port.CleanupRemoteBranchRecords
	Acquire      func(context.Context, string) (CleanupLifetime, error)
	NewAttempt   func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error)
	Preview      RemoteBranchPreviewer
	Completion   func(model.IssueOpsRecord) model.RemoteCompletionSection
	ReflectAudit func(context.Context, model.IssueOpsRecord, model.RemoteCompletionSection, string) error
	Now          func() time.Time
}

func (s RemoteBranchCleaner) Run(ctx context.Context, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error) {
	failed := model.CleanupRemoteBranchResult{OK: false, ID: req.ID}
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
	if err := domain.ValidateCleanupOperationAccess(record, model.CleanupOperationRemoteBranch); err != nil {
		return failed, err
	}
	inventory, result := s.Preview.Plan(ctx, record, req)
	if len(result.Missing) > 0 {
		return result, fmt.Errorf("cleanup remote-branch is not ready: %s", strings.Join(result.Missing, ", "))
	}
	var completion model.RemoteCompletionSection
	if !result.RemoteBranchPresent {
		// Absence still precedes confirmation and fingerprint. Only an explicit
		// apply may release a crashed same-operation attempt; preview never writes.
		result.AlreadyAbsent = true
		if record.CleanupAttempt == nil {
			return result, nil
		}
		if !req.Apply {
			result.NextCommand = fmt.Sprintf("issueops cleanup remote-branch --id %s --apply --json", record.ID)
			return result, nil
		}
	} else {
		fingerprint, err := cleanupRemoteBranchFingerprint(inventory)
		if err != nil {
			return failed, err
		}
		result.Fingerprint = fingerprint
		if !req.Apply {
			result.NextCommand = fmt.Sprintf("issueops cleanup remote-branch --id %s --apply --confirm --fingerprint %s%s --json", record.ID, fingerprint, cleanupSupersededByFlag(result.SupersededBy))
			return result, nil
		}
		if err := domain.ValidateCleanupRemoteBranchApply(req, fingerprint); err != nil {
			result.OK = false
			return result, err
		}
		completion = s.Completion(record)
	}
	attempt, err := s.NewAttempt(model.CleanupOperationRemoteBranch)
	if err != nil {
		return failed, err
	}
	snapshot, err = s.Records.Arm(ctx, snapshot, attempt)
	if err != nil {
		result.OK = false
		return result, err
	}
	finalizeCtx := context.WithoutCancel(ctx)
	// No external calls may follow this single drainage attempt. A live child
	// or replacement owner retains its guard even when the original call failed.
	finalize := func(step string, cause error) (model.CleanupRemoteBranchResult, error) {
		next, drainErr := lifetime.Drain(finalizeCtx)
		lifetime = next
		var releaseErr error
		if drainErr == nil {
			_, releaseErr = s.Records.Release(finalizeCtx, snapshot, s.Now().UTC().Format(time.RFC3339Nano))
		}
		if err := errors.Join(cause, drainErr, releaseErr); err != nil {
			result.OK = false
			if step == "" {
				step = "record_release"
			}
			result.FailedStep = step
			result.NextCommand = fmt.Sprintf("issueops cleanup remote-branch --id %s --preview --json", record.ID)
			return result, err
		}
		return result, nil
	}
	if result.AlreadyAbsent {
		return finalize("", nil)
	}
	if err := s.Records.Check(ctx, snapshot); err != nil {
		return finalize("record_check", err)
	}
	if err := s.Preview.Environment.Delete(ctx, record.Repo, inventory.Branch, inventory.RemoteOID); err != nil {
		return finalize("remote_branch_delete", err)
	}
	result.Deleted, result.DeletedAt = true, s.Now().UTC().Format(time.RFC3339)
	if s.ReflectAudit != nil {
		if err := s.Records.Check(ctx, snapshot); err != nil {
			return finalize("audit_receipt", err)
		}
		audit := fmt.Sprintf("원격 브랜치 삭제: branch=%s oid=%s at=%s", inventory.Branch, inventory.RemoteOID, result.DeletedAt)
		if err := s.ReflectAudit(ctx, record, completion, audit); err != nil {
			result.AuditError = err.Error()
		} else {
			next, err := s.Records.MarkAuditReflected(ctx, snapshot, s.Now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				return finalize("audit_receipt", err)
			}
			snapshot = next
			result.AuditReflected = true
		}
	}
	return finalize("", nil)
}

func cleanupSupersededByFlag(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return " --superseded-by '" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func cleanupRemoteBranchFingerprint(inventory model.CleanupRemoteBranchInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
