package issueops

import (
	"io/fs"
	"os"
	"path/filepath"

	model "issueops/internal/contract/issueops"
)

type MaterialFiles struct{}

func (MaterialFiles) ReadFile(p string) ([]byte, error)                 { return os.ReadFile(p) }
func (MaterialFiles) WriteFile(p string, b []byte, m fs.FileMode) error { return os.WriteFile(p, b, m) }
func (MaterialFiles) MkdirAll(p string, m fs.FileMode) error            { return os.MkdirAll(p, m) }
func (MaterialFiles) Stat(p string) (fs.FileInfo, error)                { return os.Stat(p) }
func (MaterialFiles) ArtifactPath(r model.IssueOpsRecord, root, name string) string {
	return sealedArtifactPath(r, root, name)
}

func (MaterialFiles) Join(parts ...string) string { return filepath.Join(parts...) }
func (MaterialFiles) Parent(p string) string      { return filepath.Dir(p) }
func (MaterialFiles) Resolve(root, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p)
}
func (MaterialFiles) Relative(root, p string) (string, error) {
	rel, err := filepath.Rel(root, filepath.Clean(p))
	return filepath.ToSlash(rel), err
}
