package projectdocs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
)

// AppendFiles is the filesystem effect set the project-docs append workflow
// uses to write dated caution and ADR records under a repository root.
type AppendFiles struct{}

// Path joins root with the slash-separated relative path rel using the host
// path separator.
func (AppendFiles) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

// Exists reports whether path is present; any stat error other than "not
// exist" counts as present so callers never overwrite an unreadable file.
func (AppendFiles) Exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// EnsureDir creates path and any missing parents with mode 0755.
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

// Render builds the markdown body of one dated append record for the given
// kind and request at the supplied time.
func (AppendFiles) Render(kind, name, description string, request projectdocscontract.AppendRequest, now time.Time) string {
	return renderProjectDocsAppendRecordFile(kind, name, description, request, now)
}

// Now returns the current wall-clock time; it is the seam callers replace in
// tests.
func (AppendFiles) Now() time.Time { return time.Now() }
