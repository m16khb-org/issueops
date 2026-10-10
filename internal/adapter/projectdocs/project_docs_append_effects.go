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

// AppendLine writes through O_APPEND so concurrent appends from other sessions
// never overwrite each other's lines.
func (AppendFiles) AppendLine(path, line string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 0 {
		last := make([]byte, 1)
		if _, err := file.ReadAt(last, info.Size()-1); err != nil {
			return err
		}
		if last[0] != '\n' {
			line = "\n" + line
		}
	}
	_, err = file.WriteString(line + "\n")
	return err
}
func (AppendFiles) Render(kind, name, description string, request projectdocscontract.ProjectDocsAppendRequest, now time.Time) string {
	return renderProjectDocsAppendRecordFile(kind, name, description, request, now)
}
func (AppendFiles) Now() time.Time { return time.Now() }
