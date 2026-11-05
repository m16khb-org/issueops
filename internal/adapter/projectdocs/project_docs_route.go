package projectdocs

import (
	"os"
	"path/filepath"
	"time"
)

// RouteFiles is the read-only filesystem effect set the project-docs route
// workflow uses to locate documents and dated records.
type RouteFiles struct{}

// Path joins root with the slash-separated relative path rel using the host
// path separator.
func (RouteFiles) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

// Exists reports whether path can be stat-ed without error.
func (RouteFiles) Exists(path string) bool { _, err := os.Stat(path); return err == nil }

// Now returns the current wall-clock time; it is the seam callers replace in
// tests.
func (RouteFiles) Now() time.Time { return time.Now() }

// ReadDir returns the names of the regular files directly inside dir;
// subdirectories and symlinks are skipped.
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

// ReadFile returns the whole content of path as a string alongside any read
// error.
func (RouteFiles) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
