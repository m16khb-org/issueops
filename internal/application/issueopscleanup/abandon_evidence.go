package issueopscleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"issueops/internal/contract/issueops"
)

func CleanupAbandonFailureSeal(record issueops.IssueOpsRecord, failure *issueops.IssueOpsCleanupAbandonFailure) string {
	sealed := struct {
		ID           string `json:"id"`
		Repo         string `json:"repo"`
		Fingerprint  string `json:"fingerprint"`
		RecordSHA    string `json:"record_sha"`
		WorktreePath string `json:"worktree_path"`
		Branch       string `json:"branch"`
		WorktreeHead string `json:"worktree_head"`
		BranchOID    string `json:"branch_oid"`
	}{
		ID: record.ID, Repo: record.Repo, Fingerprint: failure.Fingerprint,
		RecordSHA: failure.RecordSHA, WorktreePath: failure.WorktreePath,
		Branch: failure.Branch, WorktreeHead: failure.WorktreeHead, BranchOID: failure.BranchOID,
	}
	data, _ := json.Marshal(sealed)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func CleanupAbandonRecordSHA(record issueops.IssueOpsRecord) string {
	data, err := json.Marshal(record)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func CleanupAbandonFingerprint(inventory issueops.CleanupAbandonInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
