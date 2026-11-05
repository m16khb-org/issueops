package projectdocs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// RevisionFiles is the filesystem effect set the project-docs revise workflow
// uses to read and overwrite one document.
type RevisionFiles struct{}

func (RevisionFiles) Read(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(content), true, nil
}

func (RevisionFiles) Write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// Now returns the current wall-clock time; it is the seam callers replace in
// tests.
func (RevisionFiles) Now() time.Time { return time.Now() }
