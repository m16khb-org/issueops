package augmentation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	port "issueops/internal/port/selfaugment"
)

type Repository struct{ ListDocs func(string) []string }

var _ port.PlanRepository = Repository{}

func (Repository) ReadGeniusThink(root string) (string, string, error) {
	path := filepath.Join(root, "GENIUS_THINK.md")
	content, err := os.ReadFile(path)
	return path, string(content), err
}
func (Repository) HasImplementationDelta(root string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
