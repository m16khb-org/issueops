// Package install is the adapter for native installation: it resolves the stable source root, validates the
// installed runtime binary, lists skills, tracks retired state, and reads build generations.
package install

import (
	"os"
	"path/filepath"

	"issueops/internal/port"
)

// DefaultNativeInstallRequest normalizes common installation inputs while keeping
// host-specific file decisions in adapter implementations.
func DefaultNativeInstallRequest(root, home, codexHome, binPath string) port.NativeInstallRequest {
	invokingRoot := absClean(root)
	root = invokingRoot
	defaultBin := binPath == "" || absClean(binPath) == filepath.Join(invokingRoot, "bin", nativeBinaryName)
	if stableRoot, err := ResolveStableNativeRoot(invokingRoot); err == nil {
		root = stableRoot
		if defaultBin {
			binPath = filepath.Join(stableRoot, "bin", nativeBinaryName)
		}
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	home = absClean(home)
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	codexHome = absClean(codexHome)
	if binPath == "" && root != "" {
		binPath = filepath.Join(root, "bin", nativeBinaryName)
	}
	binPath = absClean(binPath)
	return port.NativeInstallRequest{
		Root:      root,
		Home:      home,
		CodexHome: codexHome,
		BinPath:   binPath,
	}
}

// Environment adapts this package's path and skill helpers to the port the native installer uses.
// It carries no state.
type Environment struct{}

// AbsClean returns path as a cleaned absolute path, or "" for an empty path.
func (Environment) AbsClean(path string) string { return absClean(path) }

// ResolveStableRoot maps a linked worktree to the source checkout that owns its .git directory; see ResolveStableNativeRoot.
func (Environment) ResolveStableRoot(root string) (string, error) {
	return ResolveStableNativeRoot(root)
}

// ValidateRuntime requires binary to live inside the stable source checkout of root; see validateStableNativeRuntime.
func (Environment) ValidateRuntime(root, binary string) error {
	return validateStableNativeRuntime(root, binary)
}

// ListSkills lists the skill names under root's skills directory; see ListSkillNames.
func (Environment) ListSkills(root string) ([]string, error) { return ListSkillNames(root) }
