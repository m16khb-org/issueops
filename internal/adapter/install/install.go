package install

import (
	"os"
	"path/filepath"

	installapp "issueops/internal/application/install"
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

// InstallNative supplies host filesystem observations to the shared use case.
func InstallNative(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	return (installapp.Service{Environment: nativeEnvironment{}, Installers: installers}).Install(req)
}

type nativeEnvironment struct{}

func (nativeEnvironment) AbsClean(path string) string { return absClean(path) }
func (nativeEnvironment) ResolveStableRoot(root string) (string, error) {
	return ResolveStableNativeRoot(root)
}
func (nativeEnvironment) ValidateRuntime(root, binary string) error {
	return ValidateStableNativeRuntime(root, binary)
}
func (nativeEnvironment) ListSkills(root string) ([]string, error) { return ListSkillNames(root) }
