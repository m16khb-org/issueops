//go:build unix

package hostprobe

// withIsolatedHome points the host's home directory at home. Unix hosts find
// their profile through HOME alone.
func withIsolatedHome(environment []string, home string) []string {
	return replaceEnvironmentValue(environment, "HOME", home)
}

// isolatedHomeSubdirs lists the directories under home an isolated host
// expects to exist; Unix needs none beyond home itself.
func isolatedHomeSubdirs(string) []string { return nil }
