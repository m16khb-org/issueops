package issueopscleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type RemoteBranchRecords interface {
	Load(string) (model.IssueOpsRecord, error)
}

type RemoteBranchCleaner struct {
	Records      RemoteBranchRecords
	Preview      RemoteBranchPreviewer
	Completion   func(model.IssueOpsRecord) model.RemoteCompletionSection
	ReflectAudit func(context.Context, model.IssueOpsRecord, model.RemoteCompletionSection, string) error
	Now          func() time.Time
}

func (s RemoteBranchCleaner) Run(ctx context.Context, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error) {
	record, err := s.Records.Load(req.ID)
	if err != nil {
		return model.CleanupRemoteBranchResult{OK: false, ID: req.ID}, err
	}
	inventory, result := s.Preview.Plan(ctx, record, req)
	if len(result.Missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("cleanup remote-branch is not ready: %s", strings.Join(result.Missing, ", "))
	}
	// 평가 순서 ②: 원격 브랜치가 이미 없으면 fingerprint stale 검사 이전에
	// 즉시 멱등 성공이다. 성공한 삭제 뒤의 재실행이 stale로 막히면 멱등성과
	// TOCTOU 방어가 서로를 무효화한다(design-review B1).
	if !result.RemoteBranchPresent {
		result.AlreadyAbsent = true
		return result, nil
	}
	fingerprint, err := cleanupRemoteBranchFingerprint(inventory)
	if err != nil {
		return model.CleanupRemoteBranchResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"issueops cleanup remote-branch --id %s --apply --confirm --fingerprint %s%s --json",
			record.ID, fingerprint, cleanupSupersededByFlag(result.SupersededBy))
		return result, nil
	}
	if err := domain.ValidateCleanupRemoteBranchApply(req, fingerprint); err != nil {
		result.OK = false
		return result, err
	}
	// 파괴 이전에 보존 payload를 스냅샷한다(finish C2-F1 선례).
	completionSnapshot := s.Completion(record)
	// fully-qualified ref는 동명 태그를 배제하고, force-with-lease는 preview→push
	// 사이에 남은 TOCTOU를 서버측에서 원자적으로 봉쇄한다(design-review H7).
	if err := s.Preview.Environment.Delete(ctx, record.Repo, inventory.Branch, inventory.RemoteOID); err != nil {
		result.OK = false
		result.FailedStep = "remote_branch_delete"
		result.NextCommand = fmt.Sprintf("issueops cleanup remote-branch --id %s --preview --json", record.ID)
		return result, err
	}
	result.Deleted = true
	result.DeletedAt = s.Now().UTC().Format(time.RFC3339)
	if s.ReflectAudit != nil {
		audit := fmt.Sprintf("원격 브랜치 삭제: branch=%s oid=%s at=%s", inventory.Branch, inventory.RemoteOID, result.DeletedAt)
		if err := s.ReflectAudit(ctx, record, completionSnapshot, audit); err == nil {
			result.AuditReflected = true
		} else {
			// best-effort지만 무흔적 실패는 금지 — 결과에 표면화한다.
			result.AuditError = err.Error()
		}
	}
	return result, nil
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
