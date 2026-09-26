package updatecli

import (
	"fmt"
	"os"
	"path/filepath"
)

type updateInstaller struct{}

func (updateInstaller) Install(root string, args []string) error {
	script := filepath.Join(root, "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}
	return installScriptCommandRunner(script, args...)
}

func (updateInstaller) RefreshDaemon() error {
	_, err := postInstallDaemonRefresh()
	return err
}
