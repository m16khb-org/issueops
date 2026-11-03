//go:build windows

package installcli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"issueops/internal/port"
)

const (
	localBinPathKind       = "user Path"
	manualLocalBinPathHint = `add %USERPROFILE%\.local\bin to your user Path (System Properties > Environment Variables)`
)

// addLocalBinToPath appends ~/.local/bin to the user Path in HKCU\Environment,
// the Windows counterpart of the shell rc PATH line, and announces the change
// so newly started terminals see it. Terminals already open keep their PATH,
// as they keep it after an rc edit. The registry is not a file, so the plan
// reports it as a message rather than a file entry.
func addLocalBinToPath(result *port.NativeInstallResult, req port.NativeInstallRequest) error {
	localBin := filepath.Join(req.Home, ".local", "bin")
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open user environment: %w", err)
	}
	defer key.Close()
	current, valueType, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("read user Path: %w", err)
	}
	if userPathContains(current, localBin) {
		return nil
	}
	if req.DryRun {
		result.Messages = append(result.Messages, "dry-run: would add "+localBin+` to the user Path in HKCU\Environment`)
		return nil
	}
	next := localBin
	if trimmed := strings.TrimRight(current, ";"); trimmed != "" {
		next = trimmed + ";" + localBin
	}
	if valueType == registry.SZ {
		err = key.SetStringValue("Path", next)
	} else {
		err = key.SetExpandStringValue("Path", next)
	}
	if err != nil {
		return fmt.Errorf("write user Path: %w", err)
	}
	broadcastEnvironmentChange()
	result.Messages = append(result.Messages, "added "+localBin+` to the user Path in HKCU\Environment; open a new terminal to use issueops and io`)
	return nil
}

// userPathContains reports whether a user Path value lists dir, expanding
// environment references and comparing case-insensitively like Windows does.
func userPathContains(value, dir string) bool {
	for _, entry := range filepath.SplitList(value) {
		expanded, err := registry.ExpandString(entry)
		if err != nil {
			expanded = entry
		}
		if strings.TrimSpace(expanded) != "" && strings.EqualFold(filepath.Clean(expanded), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}

var sendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// broadcastEnvironmentChange sends WM_SETTINGCHANGE "Environment", which
// Explorer uses to reload the user environment for programs it starts.
func broadcastEnvironmentChange() {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001a
		smtoAbortIfHung = 0x0002
	)
	name, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var answer uintptr
	_, _, _ = sendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(name)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&answer)))
}
