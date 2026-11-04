package gates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WorkspaceFileStore confines one request's ledger I/O to its authorized
// workspace. Relative names, including the domain defaults, resolve against
// CWD; os.Root then refuses absolute, "..", and symlink escapes at each open.
type WorkspaceFileStore struct {
	Root string
	CWD  string
}

// Discover lists the gate ledger files under cwd inside the workspace root,
// in the same order as DiscoverGateFiles. It fails when cwd escapes the root.
func (s WorkspaceFileStore) Discover(cwd string) ([]string, error) {
	dir, err := s.name(cwd)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return discoverGateFilesIn(root.FS(), filepath.ToSlash(dir), cwd), nil
}

func (s WorkspaceFileStore) Read(file string) ([]byte, error) {
	var data []byte
	err := s.within(file, func(root *os.Root, name string) (err error) {
		data, err = root.ReadFile(name)
		return err
	})
	return data, err
}

// ExistsFile reports whether file resolves inside the workspace root and is
// not a directory; any confinement or stat failure reports false.
func (s WorkspaceFileStore) ExistsFile(file string) bool {
	exists := false
	_ = s.within(file, func(root *os.Root, name string) error {
		info, err := root.Stat(name)
		exists = err == nil && !info.IsDir()
		return nil
	})
	return exists
}

// Create writes data to file with mode 0644, creating missing parent
// directories (0755) inside the workspace root.
func (s WorkspaceFileStore) Create(file string, data []byte) error {
	return s.within(file, func(root *os.Root, name string) error {
		if dir := filepath.Dir(name); dir != "." {
			if err := root.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		return root.WriteFile(name, data, 0o644)
	})
}

// WritePreservingMode overwrites file with data and keeps its existing
// permission bits, defaulting to 0644 when the file does not exist yet.
func (s WorkspaceFileStore) WritePreservingMode(file string, data []byte) error {
	return s.within(file, func(root *os.Root, name string) error {
		mode := os.FileMode(0o644)
		if info, err := root.Stat(name); err == nil {
			mode = info.Mode().Perm()
		}
		return root.WriteFile(name, data, mode)
	})
}

func (s WorkspaceFileStore) within(file string, fn func(*os.Root, string) error) error {
	name, err := s.name(file)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	return fn(root, name)
}

// name maps a request path to a root-relative name. Resolving the deepest
// existing ancestor only lets an aliased spelling of the root (/tmp versus
// /private/tmp) match; containment itself is enforced by os.Root.
func (s WorkspaceFileStore) name(file string) (string, error) {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.CWD, path)
	}
	resolved, err := resolveExistingAncestor(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(s.Root, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("gate file %s is outside the authorized workspace", file)
	}
	return relative, nil
}

func resolveExistingAncestor(path string) (string, error) {
	missing := ""
	for current := path; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, missing), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		missing = filepath.Join(filepath.Base(current), missing)
	}
}
