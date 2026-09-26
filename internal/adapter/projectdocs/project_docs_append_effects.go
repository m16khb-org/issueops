package projectdocs

import (
	"os"
	"path/filepath"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
)

type appendFileEffects struct{}

func (appendFileEffects) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (appendFileEffects) Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
func (appendFileEffects) EnsureDir(path string) error { return os.MkdirAll(path, 0o755) }
func (appendFileEffects) Write(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
func (appendFileEffects) Render(kind, name, description string, request projectdocscontract.ProjectDocsAppendRequest, now time.Time) string {
	return renderProjectDocsAppendRecordFile(kind, name, description, request, now)
}
func (appendFileEffects) Now() time.Time { return time.Now() }
