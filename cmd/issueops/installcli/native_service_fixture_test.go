package installcli

import (
	"issueops/internal/adapter/install"
	installapp "issueops/internal/application/install"
	"issueops/internal/port"
)

func installNativeForTest(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	return (installapp.Service{Environment: install.Environment{}, Installers: installers}).Install(req)
}
