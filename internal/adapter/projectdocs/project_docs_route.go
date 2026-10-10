package projectdocs

import (
	"os"
	"path/filepath"
	"time"
)

type RouteFiles struct{}

func (RouteFiles) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (RouteFiles) Exists(path string) bool { _, err := os.Stat(path); return err == nil }
func (RouteFiles) Now() time.Time          { return time.Now() }

func (RouteFiles) ReadDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

func (RouteFiles) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
