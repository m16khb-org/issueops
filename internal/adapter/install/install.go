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

type Environment struct{}

func (Environment) AbsClean(path string) string { return absClean(path) }
func (Environment) ResolveStableRoot(root string) (string, error) {
	return ResolveStableNativeRoot(root)
}
func (Environment) ValidateRuntime(root, binary string) error {
	return ValidateStableNativeRuntime(root, binary)
}
func (Environment) ListSkills(root string) ([]string, error) { return ListSkillNames(root) }
