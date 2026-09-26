package projectdocs

import (
	"os"
	"path/filepath"
	"time"
)

type revisionFileEffects struct{}

func (revisionFileEffects) Read(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(content), true, nil
}

func (revisionFileEffects) Write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (revisionFileEffects) Now() time.Time { return time.Now() }
