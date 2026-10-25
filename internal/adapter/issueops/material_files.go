package issueops

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"issueops/internal/adapter/outbound/gitbin"

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

// Ignored asks git whether rel (relative to root) is ignored there.
func (MaterialFiles) Ignored(root, rel string) bool {
	return exec.Command(gitbin.Path(), "-C", root, "check-ignore", "-q", "--", rel).Run() == nil
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
