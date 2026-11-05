package issueopscleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"issueops/internal/contract/issueops"
)

// CleanupAbandonFailureSeal returns the SHA-256 hex digest that binds an
// abandon failure receipt to its record ID, repo and the inventory fields it
// captured (fingerprint, record SHA, worktree path, branch and OIDs). Preview
// recomputes it to detect a tampered or mismatched receipt.
func CleanupAbandonFailureSeal(record issueops.Record, failure *issueops.CleanupAbandonFailure) string {
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

// CleanupAbandonRecordSHA returns the SHA-256 hex digest of the JSON-encoded
// record, or "" when the record cannot be marshaled. It pins the record state
// the abandon inventory was observed against.
func CleanupAbandonRecordSHA(record issueops.Record) string {
	data, err := json.Marshal(record)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cleanupAbandonFingerprint(inventory issueops.CleanupAbandonInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
