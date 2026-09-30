package issueopscleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	orphancontract "issueops/internal/contract/issueopsorphancleanup"
)

func orphanFingerprint(result orphancontract.Result) string {
	payload := struct {
		ID                string `json:"id"`
		RepoRoot          string `json:"repo_root"`
		WorktreePath      string `json:"worktree_path"`
		Branch            string `json:"branch"`
		Provider          string `json:"provider"`
		ArtifactKind      string `json:"artifact_kind"`
		RemoteArtifactURL string `json:"remote_artifact_url"`
		HeadSHA           string `json:"head_sha"`
		LocalBranchOID    string `json:"local_branch_oid"`
	}{
		ID: result.ID, RepoRoot: result.RepoRoot, WorktreePath: result.WorktreePath, Branch: result.Branch,
		Provider: result.Provider, ArtifactKind: result.ArtifactKind, RemoteArtifactURL: result.RemoteArtifactURL,
		HeadSHA: result.HeadSHA, LocalBranchOID: result.LocalBranchOID,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
