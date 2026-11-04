package issueops

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"issueops/internal/adapter/outbound/gitbin"

	model "issueops/internal/contract/issueops"
)

// MaterialFiles is the file and path boundary of artifact materialization. It holds no state.
type MaterialFiles struct{}

// ReadFile reads the file at p.
func (MaterialFiles) ReadFile(p string) ([]byte, error) { return os.ReadFile(p) }

// WriteFile writes b to p with mode m.
func (MaterialFiles) WriteFile(p string, b []byte, m fs.FileMode) error { return os.WriteFile(p, b, m) }

// MkdirAll creates directory p and any missing parents with mode m.
func (MaterialFiles) MkdirAll(p string, m fs.FileMode) error { return os.MkdirAll(p, m) }

// Stat returns file information for p, following symlinks.
func (MaterialFiles) Stat(p string) (fs.FileInfo, error) { return os.Stat(p) }

// ArtifactPath returns the absolute path of the sealed artifact name under the worktree root.
func (MaterialFiles) ArtifactPath(r model.Record, root, name string) string {
	return sealedArtifactPath(r, root, name)
}

// Ignored asks git whether rel (relative to root) is ignored there.
func (MaterialFiles) Ignored(root, rel string) bool {
	return exec.Command(gitbin.Path(), "-C", root, "check-ignore", "-q", "--", rel).Run() == nil
}

// Join joins path elements with the platform separator.
func (MaterialFiles) Join(parts ...string) string { return filepath.Join(parts...) }

// Parent returns the directory part of p.
func (MaterialFiles) Parent(p string) string { return filepath.Dir(p) }

// Resolve returns p cleaned, joined onto root first when p is relative.
func (MaterialFiles) Resolve(root, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p)
}

// Relative returns p relative to root with forward slashes.
func (MaterialFiles) Relative(root, p string) (string, error) {
	rel, err := filepath.Rel(root, filepath.Clean(p))
	return filepath.ToSlash(rel), err
}
