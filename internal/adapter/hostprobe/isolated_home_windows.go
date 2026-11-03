//go:build windows

package hostprobe

import "path/filepath"

// withIsolatedHome points the host's home directory at home. Windows hosts
// (Node, Bun, Rust) resolve their profile through USERPROFILE and keep config
// and caches under APPDATA and LOCALAPPDATA, so all of them move into the
// episode; HOME is set as well for tools that honor it.
func withIsolatedHome(environment []string, home string) []string {
	subdirs := isolatedHomeSubdirs(home)
	environment = replaceEnvironmentValue(environment, "HOME", home)
	environment = replaceEnvironmentValue(environment, "USERPROFILE", home)
	environment = replaceEnvironmentValue(environment, "APPDATA", subdirs[0])
	return replaceEnvironmentValue(environment, "LOCALAPPDATA", subdirs[1])
}

// isolatedHomeSubdirs lists the directories under home an isolated host
// expects to exist: the APPDATA and LOCALAPPDATA roots.
func isolatedHomeSubdirs(home string) []string {
	return []string{filepath.Join(home, "AppData", "Roaming"), filepath.Join(home, "AppData", "Local")}
}
