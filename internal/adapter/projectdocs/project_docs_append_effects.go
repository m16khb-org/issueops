package projectdocs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
)

type AppendFiles struct{}

func (AppendFiles) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (AppendFiles) Exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
func (AppendFiles) EnsureDir(path string) error { return os.MkdirAll(path, 0o755) }
func (AppendFiles) Write(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
func (AppendFiles) Render(kind, name, description string, request projectdocscontract.ProjectDocsAppendRequest, now time.Time) string {
	return renderProjectDocsAppendRecordFile(kind, name, description, request, now)
}
func (AppendFiles) Now() time.Time { return time.Now() }
