package issueopscleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

type LinkedBranchCleaner struct {
	Records               LinkedBranchAuditRecords
	RemoteRef             func(context.Context, string, string) (string, error)
	ObserveLinkedBranches func(context.Context, string) (linkedbranch.Observation, error)
	DeleteLinkedBranch    func(context.Context, string, string) error
	Now                   func() time.Time
}

// cleanupLinkedBranchInventory는 fingerprint 입력이다. 노드 id와 같은 시점의
// 원격 OID가 함께 들어가므로, preview 이후 링크가 수렴하거나 브랜치가 생기면
// fingerprint가 달라져 apply가 멈춘다(AC-05).
type cleanupLinkedBranchInventory struct {
	ID              string `json:"id"`
	IssueURL        string `json:"issue_url"`
	RequestedBranch string `json:"requested_branch"`
	SealedBase      string `json:"sealed_base"`
	LinkedBranchID  string `json:"linked_branch_id"`
	LinkedCount     int    `json:"linked_count"`
	RemoteRefOID    string `json:"remote_ref_oid"`
}

// Run은 ref-null 고아 linked-branch를 preview → apply+confirm
// +fingerprint로만 정리한다(#306 AC-04·05·06).
func (s LinkedBranchCleaner) Run(ctx context.Context, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
	record, err := s.Records.Load(req.ID)
	if err != nil {
		return issueopscontract.CleanupLinkedBranchResult{OK: false, ID: req.ID}, err
	}
	result := issueopscontract.CleanupLinkedBranchResult{OK: true, ID: record.ID, Preview: !req.Apply}
	audit := LinkedBranchAuditRecorder{Records: s.Records, Now: s.Now}

	missing := domain.LinkedBranchCleanupGates(record, s.ObserveLinkedBranches != nil, s.DeleteLinkedBranch != nil)
	if len(missing) > 0 {
		result.OK, result.Missing = false, missing
		return result, fmt.Errorf("cleanup linked-branch is not ready: %s", strings.Join(missing, ", "))
	}
	prepare := record.BranchPrepare
	result.IssueURL, result.RequestedBranch, result.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA

	observation, err := s.ObserveLinkedBranches(ctx, prepare.IssueURL)
	if err != nil {
		result.OK, result.FailedStep, result.ObserveError = false, "observe_linked_branches", err.Error()
		return result, err
	}
	observation.IssueURL, observation.RequestedBranch, observation.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA
	observation.RemoteOID, err = s.RemoteRef(ctx, record.Repo, prepare.Branch)
	if err != nil {
		result.OK, result.FailedStep, result.ObserveError = false, "observe_remote_ref", err.Error()
		return result, err
	}
	result.LinkedCount, result.RemoteRefOID = observation.TotalCount, observation.RemoteOID

	state, target, reason := linkedbranch.Classify(observation)
	result.State, result.StateReason = string(state), reason

	// 이미 없으면 fingerprint 검사 이전에 멱등 성공이다. 성공한 삭제 뒤의
	// 재실행이 stale로 막히면 멱등성과 TOCTOU 방어가 서로를 무효화한다
	// (remote-branch 정리의 같은 판단을 따른다).
	if state == linkedbranch.StateAbsent {
		result.AlreadyAbsent = true
		result.AuditRecorded, result.AuditError = audit.Record(ctx, record, result)
		return result, nil
	}
	if !linkedbranch.Deletable(state) {
		// 지울 수 없는 이유를 관측과 함께 남긴다. 이 진단이 없으면 사용자는
		// raw GraphQL 삭제로 우회하게 되고, 그것이 이 이슈가 막으려는 것이다.
		result.OK, result.FailedStep = false, "classify_linked_branch"
		result.AuditRecorded, result.AuditError = audit.Record(ctx, record, result)
		return result, fmt.Errorf("linked branch cleanup refuses state %s: %s", state, reason)
	}
	result.LinkedBranchID = target.ID

	fingerprint, err := cleanupLinkedBranchFingerprint(cleanupLinkedBranchInventory{
		ID: record.ID, IssueURL: prepare.IssueURL, RequestedBranch: prepare.Branch, SealedBase: prepare.BaseSHA,
		LinkedBranchID: target.ID, LinkedCount: observation.TotalCount, RemoteRefOID: observation.RemoteOID,
	})
	if err != nil {
		result.OK, result.FailedStep = false, "fingerprint"
		return result, err
	}
	result.Fingerprint = fingerprint

	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"issueops cleanup linked-branch --id %s --apply --confirm --fingerprint %s --json", record.ID, fingerprint)
		return result, nil
	}
	// fingerprint는 방금 다시 관측한 값으로 계산됐다. 사용자가 들고 온 값과
	// 다르면 preview 이후 외부 상태가 움직인 것이다(AC-05).
	if step, err := domain.ValidateLinkedBranchCleanupApply(req, fingerprint); err != nil {
		result.OK, result.FailedStep = false, step
		if step == "stale_fingerprint" {
			result.AuditRecorded, result.AuditError = audit.Record(ctx, record, result)
		}
		return result, err
	}
	if err := s.DeleteLinkedBranch(ctx, prepare.IssueURL, target.ID); err != nil {
		result.OK, result.FailedStep = false, "delete_linked_branch"
		result.AuditRecorded, result.AuditError = audit.Record(ctx, record, result)
		return result, err
	}
	result.Deleted, result.DeletedAt = true, s.Now().UTC().Format(time.RFC3339)
	result.AuditRecorded, result.AuditError = audit.Record(ctx, record, result)
	return result, nil
}

func cleanupLinkedBranchFingerprint(inventory cleanupLinkedBranchInventory) (string, error) {
	encoded, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
