package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	lifecycledomain "issueops/internal/domain/lifecycle"
	"os"
	"path/filepath"
	"strings"
)

func ForRoot(root string) lifecyclecontract.ProjectFingerprint {
	gitDir := ""
	if info, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if info.IsDir() {
			gitDir = filepath.Join(root, ".git")
		} else if b, err := os.ReadFile(filepath.Join(root, ".git")); err == nil {
			gitDir = strings.TrimSpace(string(b))
		}
	}
	originHash := ""
	if origin := ReadGitOriginURL(root); origin != "" {
		sum := sha256.Sum256([]byte(origin))
		originHash = hex.EncodeToString(sum[:])
	}
	return lifecyclecontract.ProjectFingerprint{RepoRoot: root, GitDir: gitDir, GitOriginHash: originHash}
}

func RepoID(fp lifecyclecontract.ProjectFingerprint) string {
	return lifecycledomain.RepoID(fp)
}

func Equal(a, b lifecyclecontract.ProjectFingerprint) bool {
	return lifecycledomain.EqualFingerprint(a, b)
}
