package issueopsinventory

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/domain/repoidentity"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type CleanPath struct{}

func (CleanPath) Normalize(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return ""
	}
	if !filepath.IsAbs(clean) {
		if absolute, err := filepath.Abs(clean); err == nil {
			clean = absolute
		}
	}
	clean = filepath.Clean(clean)
	command := exec.Command("git", "rev-parse", "--path-format=relative", "--git-common-dir")
	command.Dir = clean
	commonDir, err := command.Output()
	if err != nil {
		return clean
	}
	return repoidentity.SourceRoot(clean, string(commonDir))
}
