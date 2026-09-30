package install

import (
	installapp "issueops/internal/application/install"
	"issueops/internal/port"
)

func installNativeForTest(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	return (installapp.Service{Environment: Environment{}, Installers: installers}).Install(req)
}
