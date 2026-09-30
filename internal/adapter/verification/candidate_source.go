package verification

import (
	"os"
	"path/filepath"
)

func CandidateSource(root string) (string, bool) {
	path := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	_, err := os.Stat(path)
	return path, err == nil
}
