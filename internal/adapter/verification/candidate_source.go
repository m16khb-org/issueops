package verification

import (
	"os"
	"path/filepath"
)

// CandidateSource returns the path of skills/self-verify/CANDIDATES.md under
// root and whether that file exists.
func CandidateSource(root string) (string, bool) {
	path := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	_, err := os.Stat(path)
	return path, err == nil
}
