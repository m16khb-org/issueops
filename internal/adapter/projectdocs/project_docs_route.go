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
