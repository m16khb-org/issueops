// Package fingerprint derives a stable identity fingerprint of a project
// checkout for the lifecycle adapters.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	"os"
	"path/filepath"
	"strings"
)

// ForRoot computes the ProjectFingerprint of the repository at root: its git
// directory (the .git directory, or the gitdir pointer file content for
// worktrees) and the SHA-256 of the origin URL returned by readOrigin. Fields
// are left empty when .git is absent or readOrigin returns "".
func ForRoot(root string, readOrigin func(string) string) lifecyclecontract.ProjectFingerprint {
	gitDir := ""
	if info, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if info.IsDir() {
			gitDir = filepath.Join(root, ".git")
		} else if b, err := os.ReadFile(filepath.Join(root, ".git")); err == nil {
			gitDir = strings.TrimSpace(string(b))
		}
	}
	originHash := ""
	if origin := readOrigin(root); origin != "" {
		sum := sha256.Sum256([]byte(origin))
		originHash = hex.EncodeToString(sum[:])
	}
	return lifecyclecontract.ProjectFingerprint{RepoRoot: root, GitDir: gitDir, GitOriginHash: originHash}
}
